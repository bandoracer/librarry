package notify

import "strings"

// telegram botTokens are the only settings value treated as a secret: webhook
// and ntfy URLs are operator-entered endpoints that must round-trip through
// the settings UI intact.
const secretSettingKey = "botToken"

// RedactTarget masks secret settings values for GET responses, keeping the
// last 4 characters so operators can recognize which credential is stored.
func RedactTarget(target Target) Target {
	if !strings.EqualFold(strings.TrimSpace(target.Type), TargetTypeTelegram) {
		return target
	}
	settings := map[string]string{}
	for key, value := range target.Settings {
		if strings.EqualFold(strings.TrimSpace(key), secretSettingKey) {
			value = redactSecret(value)
		}
		settings[key] = value
	}
	target.Settings = settings
	return target
}

// MergeSecrets restores stored secrets when an update leaves them blank or
// echoes the redacted placeholder back (blank-means-keep).
func MergeSecrets(incoming Target, stored Target) Target {
	if !strings.EqualFold(strings.TrimSpace(incoming.Type), TargetTypeTelegram) {
		return incoming
	}
	if incoming.Settings == nil {
		incoming.Settings = map[string]string{}
	}
	value := ""
	key := secretSettingKey
	for candidate, candidateValue := range incoming.Settings {
		if strings.EqualFold(strings.TrimSpace(candidate), secretSettingKey) {
			key = candidate
			value = strings.TrimSpace(candidateValue)
			break
		}
	}
	storedValue := stored.Setting(secretSettingKey)
	if storedValue == "" {
		return incoming
	}
	if value == "" || value == redactSecret(storedValue) {
		incoming.Settings[key] = storedValue
	}
	return incoming
}

func redactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}
