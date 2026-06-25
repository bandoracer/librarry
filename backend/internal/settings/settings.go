package settings

import "strings"

type Settings struct {
	HardcoverToken    string `json:"hardcoverToken"`
	GoogleBooksAPIKey string `json:"googleBooksApiKey"`
	ProwlarrURL       string `json:"prowlarrUrl"`
	ProwlarrAPIKey    string `json:"prowlarrApiKey"`
	QBittorrentURL    string `json:"qbittorrentUrl"`
	SABnzbdURL        string `json:"sabnzbdUrl"`
	SABnzbdAPIKey     string `json:"sabnzbdApiKey"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

func Validate(input Settings) ValidationResult {
	var result ValidationResult

	if strings.TrimSpace(input.HardcoverToken) == "" {
		result.Warnings = append(result.Warnings, "Hardcover token is missing; rich metadata search will be disabled.")
	}
	if strings.TrimSpace(input.GoogleBooksAPIKey) == "" {
		result.Warnings = append(result.Warnings, "Google Books API key is missing; exact fallback lookups will be disabled.")
	}

	if hasAny(input.ProwlarrURL, input.ProwlarrAPIKey) && !hasAll(input.ProwlarrURL, input.ProwlarrAPIKey) {
		result.Errors = append(result.Errors, "Prowlarr URL and API key must be provided together.")
	}

	if strings.TrimSpace(input.QBittorrentURL) == "" && strings.TrimSpace(input.SABnzbdURL) == "" {
		result.Warnings = append(result.Warnings, "No download client is configured yet.")
	}
	if hasAny(input.SABnzbdURL, input.SABnzbdAPIKey) && !hasAll(input.SABnzbdURL, input.SABnzbdAPIKey) {
		result.Errors = append(result.Errors, "SABnzbd URL and API key must be provided together.")
	}

	result.Valid = len(result.Errors) == 0
	return result
}

func hasAny(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func hasAll(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}
