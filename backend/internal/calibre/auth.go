package calibre

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/icholy/digest"
)

// Discover the server's authentication scheme with a read-only request before
// sending an upload or mutation. Never probe a mutation endpoint with an empty
// body, replay an ambiguous write, or forward these credentials through redirects.
func (c *Client) do(request *http.Request, settings Settings) (*http.Response, error) {
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	request.Header.Del("Authorization")
	if settings.Username != "" {
		base, err := baseURL(settings)
		if err != nil {
			return nil, err
		}
		if (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
			return nil, errors.New("Calibre host must be an HTTP(S) URL without embedded credentials, query or fragment")
		}
		base.Path = joinURLPath(base.Path, "ajax", "library-info")
		probe, err := http.NewRequestWithContext(request.Context(), http.MethodGet, base.String(), nil)
		if err != nil {
			return nil, errors.New("invalid Calibre authentication URL")
		}
		response, err := client.Do(probe)
		if err != nil {
			return nil, calibreRequestError(request)
		}
		header := response.Header.Clone()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			return nil, fmt.Errorf("Calibre did not provide an authentication challenge (HTTP %d); check the server authentication settings", response.StatusCode)
		}
		challenge, err := digest.FindChallenge(header)
		if err == nil {
			credentials, err := digest.Digest(challenge, digest.Options{Username: settings.Username, Password: settings.Password, Method: request.Method, URI: request.URL.RequestURI(), GetBody: request.GetBody, Count: 1})
			if err != nil {
				return nil, errors.New("Calibre Digest challenge is unsupported")
			}
			request.Header.Set("Authorization", credentials.String())
		} else {
			basic := false
			for _, value := range header.Values("WWW-Authenticate") {
				basic = basic || strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "basic ")
			}
			if !basic {
				return nil, errors.New("Calibre authentication challenge is unsupported")
			}
			request.SetBasicAuth(settings.Username, settings.Password)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, calibreRequestError(request)
	}
	return response, nil
}

func calibreRequestError(request *http.Request) error {
	if request.Context().Err() != nil {
		return request.Context().Err()
	}
	// net/http errors can contain the URL, including proxy credentials or query
	// strings supplied by a transport. Keep them out of API responses and history.
	return errors.New("Calibre request failed; verify the server connection before retrying")
}
