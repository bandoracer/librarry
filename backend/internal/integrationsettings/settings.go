package integrationsettings

import (
	"context"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
)

type ResourceStore interface {
	ListResources(ctx context.Context, resourceType string) ([]compatdata.Resource, error)
	UpsertResource(ctx context.Context, resource compatdata.Resource) (compatdata.Resource, error)
	DeleteResource(ctx context.Context, resourceType string, compatID int) (bool, error)
}

type Settings struct {
	ProwlarrURL                string `json:"prowlarrUrl"`
	ProwlarrAPIKey             string `json:"prowlarrApiKey,omitempty"`
	ProwlarrAPIKeyConfigured   bool   `json:"prowlarrApiKeyConfigured"`
	QBittorrentURL             string `json:"qbittorrentUrl"`
	QBittorrentUsername        string `json:"qbittorrentUsername"`
	QBittorrentPassword        string `json:"qbittorrentPassword,omitempty"`
	QBittorrentPassConfigured  bool   `json:"qbittorrentPasswordConfigured"`
	TransmissionURL            string `json:"transmissionUrl"`
	TransmissionUsername       string `json:"transmissionUsername"`
	TransmissionPassword       string `json:"transmissionPassword,omitempty"`
	TransmissionPassConfigured bool   `json:"transmissionPasswordConfigured"`
	SABnzbdURL                 string `json:"sabnzbdUrl"`
	SABnzbdAPIKey              string `json:"sabnzbdApiKey,omitempty"`
	SABnzbdAPIKeyConfigured    bool   `json:"sabnzbdApiKeyConfigured"`
	SABnzbdUsername            string `json:"sabnzbdUsername"`
	SABnzbdPassword            string `json:"sabnzbdPassword,omitempty"`
	SABnzbdPassConfigured      bool   `json:"sabnzbdPasswordConfigured"`
	EbookCategory              string `json:"ebookCategory"`
	AudiobookCategory          string `json:"audiobookCategory"`
	BookTorrentRoot            string `json:"bookTorrentRoot"`
}

type Patch struct {
	ProwlarrURL           *string `json:"prowlarrUrl"`
	ProwlarrAPIKey        *string `json:"prowlarrApiKey"`
	ClearProwlarrAPIKey   bool    `json:"clearProwlarrApiKey"`
	QBittorrentURL        *string `json:"qbittorrentUrl"`
	QBittorrentUsername   *string `json:"qbittorrentUsername"`
	QBittorrentPassword   *string `json:"qbittorrentPassword"`
	ClearQBittorrentPass  bool    `json:"clearQbittorrentPassword"`
	TransmissionURL       *string `json:"transmissionUrl"`
	TransmissionUsername  *string `json:"transmissionUsername"`
	TransmissionPassword  *string `json:"transmissionPassword"`
	ClearTransmissionPass bool    `json:"clearTransmissionPassword"`
	SABnzbdURL            *string `json:"sabnzbdUrl"`
	SABnzbdAPIKey         *string `json:"sabnzbdApiKey"`
	ClearSABnzbdAPIKey    bool    `json:"clearSabnzbdApiKey"`
	SABnzbdUsername       *string `json:"sabnzbdUsername"`
	SABnzbdPassword       *string `json:"sabnzbdPassword"`
	ClearSABnzbdPass      bool    `json:"clearSabnzbdPassword"`
	EbookCategory         *string `json:"ebookCategory"`
	AudiobookCategory     *string `json:"audiobookCategory"`
	BookTorrentRoot       *string `json:"bookTorrentRoot"`
}

func FromResources(ctx context.Context, store ResourceStore, base acquisition.IntegrationConfig) (acquisition.IntegrationConfig, error) {
	if store == nil {
		return base, nil
	}
	next := base
	indexers, err := store.ListResources(ctx, "indexer")
	if err != nil {
		return base, err
	}
	for _, resource := range indexers {
		if !payloadBoolDefault(resource.Payload, "enable", true) {
			continue
		}
		baseURL := resourceFieldString(resource.Payload, "baseUrl")
		if baseURL == "" {
			baseURL = resourceFieldString(resource.Payload, "host")
		}
		if baseURL == "" {
			continue
		}
		next.ProwlarrURL = trimURL(baseURL)
		if apiKey := resourceFieldString(resource.Payload, "apiKey"); usableSecret(apiKey) {
			next.ProwlarrAPIKey = apiKey
		}
		break
	}

	clients, err := store.ListResources(ctx, "download-client")
	if err != nil {
		return base, err
	}
	for _, resource := range clients {
		if !payloadBoolDefault(resource.Payload, "enable", true) {
			continue
		}
		implementation := strings.ToLower(firstNonEmpty(
			resourceFieldString(resource.Payload, "librarryImplementation"),
			payloadString(resource.Payload, "implementation"),
			payloadString(resource.Payload, "implementationName"),
			resource.Name,
		))
		host := resourceFieldString(resource.Payload, "host")
		if host == "" {
			host = resourceFieldString(resource.Payload, "baseUrl")
		}
		if host == "" {
			continue
		}
		username := resourceFieldString(resource.Payload, "username")
		password := resourceFieldString(resource.Payload, "password")
		apiKey := resourceFieldString(resource.Payload, "apiKey")
		category := resourceFieldString(resource.Payload, "category")
		if category != "" {
			next.EbookCategory = category
		}
		if audiobookCategory := resourceFieldString(resource.Payload, "librarryAudiobookCategory"); audiobookCategory != "" {
			next.AudiobookCategory = audiobookCategory
		}
		if root := resourceFieldString(resource.Payload, "librarryBookTorrentRoot"); root != "" {
			next.BookTorrentRoot = root
		}
		switch {
		case strings.Contains(implementation, "transmission"):
			next.TransmissionURL = trimURL(host)
			next.TransmissionUser = username
			if usableSecret(password) {
				next.TransmissionPass = password
			}
		case strings.Contains(implementation, "sab"):
			next.SABnzbdURL = trimURL(host)
			next.SABnzbdUser = username
			if usableSecret(password) {
				next.SABnzbdPass = password
			}
			if usableSecret(apiKey) {
				next.SABnzbdAPIKey = apiKey
			}
		default:
			next.QBittorrentURL = trimURL(host)
			next.QBittorrentUser = username
			if usableSecret(password) {
				next.QBittorrentPass = password
			}
		}
	}
	return next, nil
}

func ApplyPatch(base acquisition.IntegrationConfig, patch Patch) acquisition.IntegrationConfig {
	next := base
	if patch.ProwlarrURL != nil {
		next.ProwlarrURL = trimURL(*patch.ProwlarrURL)
	}
	if patch.ClearProwlarrAPIKey {
		next.ProwlarrAPIKey = ""
	} else if patch.ProwlarrAPIKey != nil && usableSecret(*patch.ProwlarrAPIKey) {
		next.ProwlarrAPIKey = strings.TrimSpace(*patch.ProwlarrAPIKey)
	}
	if patch.QBittorrentURL != nil {
		next.QBittorrentURL = trimURL(*patch.QBittorrentURL)
	}
	if patch.QBittorrentUsername != nil {
		next.QBittorrentUser = strings.TrimSpace(*patch.QBittorrentUsername)
	}
	if patch.ClearQBittorrentPass {
		next.QBittorrentPass = ""
	} else if patch.QBittorrentPassword != nil && usableSecret(*patch.QBittorrentPassword) {
		next.QBittorrentPass = strings.TrimSpace(*patch.QBittorrentPassword)
	}
	if patch.TransmissionURL != nil {
		next.TransmissionURL = trimURL(*patch.TransmissionURL)
	}
	if patch.TransmissionUsername != nil {
		next.TransmissionUser = strings.TrimSpace(*patch.TransmissionUsername)
	}
	if patch.ClearTransmissionPass {
		next.TransmissionPass = ""
	} else if patch.TransmissionPassword != nil && usableSecret(*patch.TransmissionPassword) {
		next.TransmissionPass = strings.TrimSpace(*patch.TransmissionPassword)
	}
	if patch.SABnzbdURL != nil {
		next.SABnzbdURL = trimURL(*patch.SABnzbdURL)
	}
	if patch.ClearSABnzbdAPIKey {
		next.SABnzbdAPIKey = ""
	} else if patch.SABnzbdAPIKey != nil && usableSecret(*patch.SABnzbdAPIKey) {
		next.SABnzbdAPIKey = strings.TrimSpace(*patch.SABnzbdAPIKey)
	}
	if patch.SABnzbdUsername != nil {
		next.SABnzbdUser = strings.TrimSpace(*patch.SABnzbdUsername)
	}
	if patch.ClearSABnzbdPass {
		next.SABnzbdPass = ""
	} else if patch.SABnzbdPassword != nil && usableSecret(*patch.SABnzbdPassword) {
		next.SABnzbdPass = strings.TrimSpace(*patch.SABnzbdPassword)
	}
	if patch.EbookCategory != nil {
		next.EbookCategory = firstNonEmpty(strings.TrimSpace(*patch.EbookCategory), "books-ebook")
	}
	if patch.AudiobookCategory != nil {
		next.AudiobookCategory = firstNonEmpty(strings.TrimSpace(*patch.AudiobookCategory), "books-audiobook")
	}
	if patch.BookTorrentRoot != nil {
		next.BookTorrentRoot = firstNonEmpty(strings.TrimSpace(*patch.BookTorrentRoot), "/data/torrents/books")
	}
	return next
}

func ToSettings(config acquisition.IntegrationConfig) Settings {
	return Settings{
		ProwlarrURL:                config.ProwlarrURL,
		ProwlarrAPIKeyConfigured:   config.ProwlarrAPIKey != "",
		QBittorrentURL:             config.QBittorrentURL,
		QBittorrentUsername:        config.QBittorrentUser,
		QBittorrentPassConfigured:  config.QBittorrentPass != "",
		TransmissionURL:            config.TransmissionURL,
		TransmissionUsername:       config.TransmissionUser,
		TransmissionPassConfigured: config.TransmissionPass != "",
		SABnzbdURL:                 config.SABnzbdURL,
		SABnzbdAPIKeyConfigured:    config.SABnzbdAPIKey != "",
		SABnzbdUsername:            config.SABnzbdUser,
		SABnzbdPassConfigured:      config.SABnzbdPass != "",
		EbookCategory:              config.EbookCategory,
		AudiobookCategory:          config.AudiobookCategory,
		BookTorrentRoot:            config.BookTorrentRoot,
	}
}

func SaveResources(ctx context.Context, store ResourceStore, config acquisition.IntegrationConfig) error {
	if store == nil {
		return nil
	}
	if err := saveIndexer(ctx, store, config); err != nil {
		return err
	}
	for _, client := range []struct {
		id               int
		name             string
		protocol         string
		host             string
		username         string
		password         string
		apiKey           string
		category         string
		extraAudioCat    string
		extraTorrentRoot string
	}{
		{1, "qBittorrent", "torrent", config.QBittorrentURL, config.QBittorrentUser, config.QBittorrentPass, "", config.EbookCategory, config.AudiobookCategory, config.BookTorrentRoot},
		{3, "Transmission", "torrent", config.TransmissionURL, config.TransmissionUser, config.TransmissionPass, "", config.EbookCategory, config.AudiobookCategory, config.BookTorrentRoot},
		{2, "SABnzbd", "usenet", config.SABnzbdURL, config.SABnzbdUser, config.SABnzbdPass, config.SABnzbdAPIKey, config.EbookCategory, config.AudiobookCategory, config.BookTorrentRoot},
	} {
		if err := saveDownloadClient(ctx, store, client.id, client.name, client.protocol, client.host, client.username, client.password, client.apiKey, client.category, client.extraAudioCat, client.extraTorrentRoot); err != nil {
			return err
		}
	}
	return nil
}

func saveIndexer(ctx context.Context, store ResourceStore, config acquisition.IntegrationConfig) error {
	if strings.TrimSpace(config.ProwlarrURL) == "" {
		_, err := store.DeleteResource(ctx, "indexer", 1)
		return err
	}
	_, err := store.UpsertResource(ctx, compatdata.Resource{
		ResourceType: "indexer",
		CompatID:     1,
		Name:         "Prowlarr",
		Payload: map[string]any{
			"name":                    "Prowlarr",
			"implementation":          "Torznab",
			"implementationName":      "Prowlarr",
			"configContract":          "TorznabSettings",
			"protocol":                "torrent",
			"enable":                  true,
			"enableRss":               true,
			"enableAutomaticSearch":   true,
			"enableInteractiveSearch": true,
			"priority":                25,
			"fields": []map[string]any{
				{"name": "baseUrl", "value": config.ProwlarrURL},
				{"name": "apiKey", "value": config.ProwlarrAPIKey},
				{"name": "categories", "value": "7020,8010"},
			},
		},
	})
	return err
}

func saveDownloadClient(ctx context.Context, store ResourceStore, id int, name string, protocol string, host string, username string, password string, apiKey string, category string, audiobookCategory string, torrentRoot string) error {
	if strings.TrimSpace(host) == "" {
		_, err := store.DeleteResource(ctx, "download-client", id)
		return err
	}
	fields := []map[string]any{
		{"name": "host", "value": trimURL(host)},
		{"name": "username", "value": username},
		{"name": "password", "value": password},
		{"name": "category", "value": category},
		{"name": "recentPriority", "value": 0},
		{"name": "librarryImplementation", "value": name},
		{"name": "librarryAudiobookCategory", "value": audiobookCategory},
		{"name": "librarryBookTorrentRoot", "value": torrentRoot},
	}
	if apiKey != "" {
		fields = append(fields, map[string]any{"name": "apiKey", "value": apiKey})
	}
	_, err := store.UpsertResource(ctx, compatdata.Resource{
		ResourceType: "download-client",
		CompatID:     id,
		Name:         name,
		Payload: map[string]any{
			"name":               name,
			"implementation":     name,
			"implementationName": name,
			"configContract":     name + "Settings",
			"protocol":           protocol,
			"enable":             true,
			"priority":           id,
			"fields":             fields,
		},
	})
	return err
}

func resourceFieldString(payload map[string]any, name string) string {
	if value := payloadString(payload, name); value != "" {
		return value
	}
	for _, field := range payloadArray(payload, "fields") {
		fieldMap, ok := field.(map[string]any)
		if !ok {
			continue
		}
		if !strings.EqualFold(payloadString(fieldMap, "name"), name) {
			continue
		}
		return strings.TrimSpace(anyString(fieldMap["value"]))
	}
	return ""
}

func payloadArray(payload map[string]any, key string) []any {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []any:
		return typed
	case []map[string]any:
		values := make([]any, 0, len(typed))
		for _, value := range typed {
			values = append(values, value)
		}
		return values
	default:
		return nil
	}
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	return strings.TrimSpace(anyString(payload[key]))
}

func payloadBoolDefault(payload map[string]any, key string, fallback bool) bool {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return fallback
	}
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return fallback
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

func usableSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, char := range trimmed {
		if char != '*' && char != '•' {
			return true
		}
	}
	return false
}

func trimURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
