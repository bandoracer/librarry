package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var healthVersionPattern = regexp.MustCompile(`(?i)^v?([0-9]{1,9}(?:\.[0-9]{1,9}){1,3})(?:(?:alpha|beta|rc)[0-9]*|[-+][a-z0-9._-]+| \([a-z0-9._-]+\))?$`)

func healthVersion(value string) string {
	if len(value) > 256 {
		return ""
	}
	m := healthVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
func healthBool(value bool) *bool { return &value }
func healthBase(name string, configured bool) IntegrationHealth {
	h := IntegrationHealth{Name: name, Configured: configured, Status: "missing_credentials", Message: "Configure the integration endpoint and required credentials."}
	if configured {
		h.Status = "degraded"
		h.Message = "The response did not match the expected API shape."
	}
	return h
}

// Response bodies and request URLs never become health messages. Even a 2xx
// needs protocol validation before it can count as a successful connection.
func healthRead(ctx context.Context, client *http.Client, req *http.Request, h *IntegrationHealth) ([]byte, http.Header, int) {
	response, err := healthClient(client).Do(req.WithContext(ctx))
	if err != nil {
		h.Status = "unavailable"
		h.Message = "Connection failed or timed out."
		h.Reachable = healthBool(false)
		return nil, nil, 0
	}
	defer response.Body.Close()
	h.Reachable = healthBool(true)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		h.Status = "degraded"
		h.Message = "The integration returned an unexpected HTTP response."
		switch response.StatusCode {
		case 401, 403:
			h.Status = "invalid_credentials"
			h.Message = "The integration rejected access. Check credentials and access rules."
			h.Authenticated = healthBool(false)
		case 429:
			h.Status = "rate_limited"
			h.Message = "The integration requested a delay before checking again."
			at := time.Now().UTC().Add(time.Minute)
			if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds >= 0 && seconds <= 86400 {
				at = time.Now().UTC().Add(time.Duration(seconds) * time.Second)
			} else if date, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil && date.After(time.Now()) {
				at = date
			}
			maximum := time.Now().UTC().Add(24 * time.Hour)
			if at.After(maximum) {
				at = maximum
			}
			h.RetryAfter = &at
		default:
			if response.StatusCode >= 500 {
				h.Status = "unavailable"
				h.Message = "The integration is temporarily unavailable."
			}
		}
		return nil, response.Header, response.StatusCode
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		h.Message = "The integration response was incomplete or too large."
		return nil, response.Header, response.StatusCode
	}
	return body, response.Header, response.StatusCode
}
func healthReady(h IntegrationHealth, version string) IntegrationHealth {
	h.Status = "ready"
	h.Message = "Connection verified by a read-only API request."
	h.Authenticated = healthBool(true)
	h.Version = healthVersion(version)
	return h
}

func (c *ProwlarrClient) Health(ctx context.Context) IntegrationHealth {
	h := healthBase(c.Name(), c.Configured())
	if !h.Configured {
		return h
	}
	req, err := c.request(ctx, http.MethodGet, "/api/v1/system/status", nil)
	if err != nil {
		return h
	}
	body, _, _ := healthRead(ctx, c.client, req, &h)
	if body == nil {
		return h
	}
	var payload struct {
		AppName string `json:"appName"`
		Version string `json:"version"`
	}
	if json.Unmarshal(body, &payload) != nil || !strings.EqualFold(payload.AppName, "Prowlarr") || healthVersion(payload.Version) == "" {
		return h
	}
	return healthReady(h, payload.Version)
}

func (c *QBittorrentClient) Health(ctx context.Context) IntegrationHealth {
	h := healthBase(c.Name(), c.Configured())
	if !h.Configured {
		return h
	}
	if c.username != "" || c.password != "" {
		values := url.Values{"username": {c.username}, "password": {c.password}}
		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v2/auth/login", strings.NewReader(values.Encode()))
		if err != nil {
			return h
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		body, _, _ := healthRead(ctx, c.client, req, &h)
		if body == nil {
			return h
		}
		if strings.TrimSpace(string(body)) != "Ok." {
			h.Status = "invalid_credentials"
			h.Message = "The integration rejected login."
			h.Authenticated = healthBool(false)
			return h
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v2/app/version", nil)
	if err != nil {
		return h
	}
	body, _, _ := healthRead(ctx, c.client, req, &h)
	if body == nil || healthVersion(string(body)) == "" {
		return h
	}
	result := healthReady(h, string(body))
	if c.username == "" && c.password == "" {
		result.Authenticated = nil
	}
	return result
}

func (c *TransmissionClient) Health(ctx context.Context) IntegrationHealth {
	h := healthBase(c.Name(), c.Configured())
	if !h.Configured {
		return h
	}
	data := []byte(`{"method":"session-get","arguments":{"fields":["version","rpc-version"]}}`)
	sessionID := ""
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL(), bytes.NewReader(data))
		if err != nil {
			return h
		}
		req.Header.Set("Content-Type", "application/json")
		if sessionID != "" {
			req.Header.Set("X-Transmission-Session-Id", sessionID)
		}
		if c.username != "" || c.password != "" {
			req.SetBasicAuth(c.username, c.password)
		}
		body, headers, status := healthRead(ctx, c.client, req, &h)
		if status == 409 && attempt == 0 && headers.Get("X-Transmission-Session-Id") != "" {
			sessionID = headers.Get("X-Transmission-Session-Id")
			continue
		}
		if body == nil {
			return h
		}
		var payload struct {
			Result    string `json:"result"`
			Arguments struct {
				Version    string `json:"version"`
				RPCVersion int    `json:"rpc-version"`
			} `json:"arguments"`
		}
		if json.Unmarshal(body, &payload) != nil || payload.Result != "success" || payload.Arguments.RPCVersion <= 0 || healthVersion(payload.Arguments.Version) == "" {
			return h
		}
		result := healthReady(h, payload.Arguments.Version)
		if c.username == "" && c.password == "" {
			result.Authenticated = nil
		}
		return result
	}
	return h
}

func (c *SABnzbdClient) Health(ctx context.Context) IntegrationHealth {
	h := healthBase(c.Name(), c.Configured())
	if !h.Configured {
		return h
	}
	// version does not require an API key. A bounded queue read proves the key can
	// read download state, while discarding all job names and other private data.
	values := url.Values{"mode": {"queue"}, "limit": {"1"}, "output": {"json"}, "apikey": {c.apiKey}}
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api?"+values.Encode(), nil)
	if err != nil {
		return h
	}
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	body, _, _ := healthRead(ctx, c.client, req, &h)
	if body == nil {
		return h
	}
	var payload struct {
		Error string `json:"error"`
		Queue *struct {
			Version string            `json:"version"`
			Status  string            `json:"status"`
			Slots   []json.RawMessage `json:"slots"`
		} `json:"queue"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return h
	}
	if payload.Error != "" {
		if payload.Error == "API Key Incorrect" || payload.Error == "API Key Required" {
			h.Status = "invalid_credentials"
			h.Message = "The integration rejected the API key."
			h.Authenticated = healthBool(false)
		}
		return h
	}
	if payload.Queue == nil || payload.Queue.Status == "" || payload.Queue.Slots == nil || healthVersion(payload.Queue.Version) == "" {
		return h
	}
	return healthReady(h, payload.Queue.Version)
}
