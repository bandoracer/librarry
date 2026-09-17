// Package kindle sends explicitly selected library documents through authenticated SMTP.
package kindle

import (
	"net/mail"
	"os"
	"strconv"
	"strings"
)

type Settings struct {
	Enabled            bool   `json:"enabled"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	TLSMode            string `json:"tlsMode"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
	PasswordConfigured bool   `json:"passwordConfigured"`
	From               string `json:"from"`
	FromName           string `json:"fromName"`
	Recipient          string `json:"recipient"`
}

func FromEnv() Settings {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 465
	}
	mode := os.Getenv("SMTP_TLS_MODE")
	if mode == "" {
		mode = "implicit"
	}
	password := os.Getenv("SMTP_PASSWORD")
	if password == "" {
		password = os.Getenv("RESEND_API_KEY")
	}
	return Settings{Enabled: os.Getenv("LIBRARRY_KINDLE_ENABLED") == "true", Host: os.Getenv("SMTP_HOST"), Port: port, TLSMode: mode, Username: os.Getenv("SMTP_USERNAME"), Password: password, From: os.Getenv("SMTP_FROM"), FromName: os.Getenv("SMTP_FROM_NAME"), Recipient: os.Getenv("LIBRARRY_KINDLE_EMAIL")}
}

func (s Settings) Redacted() Settings {
	s.PasswordConfigured = s.Password != ""
	s.Password = ""
	return s
}
func (s Settings) Validate() error {
	if s.Port < 1 || s.Port > 65535 {
		return invalid("SMTP port must be between 1 and 65535")
	}
	if s.TLSMode != "implicit" && s.TLSMode != "starttls" {
		return invalid("SMTP requires implicit TLS or STARTTLS")
	}
	if strings.ContainsAny(s.Host, "\r\n/ ") || strings.ContainsAny(s.Username+s.FromName, "\r\n") || len(s.FromName) > 100 {
		return invalid("invalid SMTP host, username or sender name")
	}
	if s.From != "" && !plainAddress(s.From) {
		return invalid("enter a plain sender email address")
	}
	if s.Recipient != "" && !kindleAddress(s.Recipient) {
		return invalid("recipient must be a @kindle.com or @free.kindle.com address")
	}
	if s.Enabled && (s.Host == "" || s.Username == "" || s.Password == "" || s.From == "" || s.Recipient == "") {
		return invalid("SMTP host, credentials, sender and Kindle address are required")
	}
	return nil
}
func plainAddress(v string) bool {
	a, e := mail.ParseAddress(v)
	return e == nil && a.Address == v && !strings.ContainsAny(v, "\r\n")
}
func kindleAddress(v string) bool {
	if !plainAddress(v) {
		return false
	}
	parts := strings.Split(v, "@")
	if len(parts) != 2 {
		return false
	}
	d := strings.ToLower(parts[1])
	return d == "kindle.com" || d == "free.kindle.com"
}

// InvalidError contains a safe, actionable operator message.
type InvalidError struct{ Message string }

func (e *InvalidError) Error() string { return e.Message }
func invalid(message string) error    { return &InvalidError{Message: message} }
