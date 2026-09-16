package config

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// ValidateEnvironment rejects malformed typed settings before workers or database
// startup. In particular, a misspelled false must not become default auto-grab.
// Error messages identify the variable and expected type, never its value.
func ValidateEnvironment() error {
	schema := reflect.TypeOf(Config{})
	durationType := reflect.TypeOf(time.Duration(0))
	for i := 0; i < schema.NumField(); i++ {
		field := schema.Field(i)
		key := field.Tag.Get("env")
		raw := strings.TrimSpace(os.Getenv(key))
		if key == "" || raw == "" {
			continue
		}
		var err error
		expected := field.Type.Kind().String()
		switch {
		case field.Type == durationType:
			var value time.Duration
			value, err = time.ParseDuration(raw)
			if err != nil {
				// Historical numeric duration values are minutes.
				var minutes int64
				minutes, err = strconv.ParseInt(raw, 10, 64)
				if err == nil {
					if minutes > math.MaxInt64/int64(time.Minute) || minutes < 0 {
						err = fmt.Errorf("invalid duration")
					} else {
						value = time.Duration(minutes) * time.Minute
					}
				}
			}
			if err == nil && value <= 0 {
				err = fmt.Errorf("nonpositive duration")
			}
			expected = "positive duration (for example 30m)"
		case field.Type.Kind() == reflect.Bool:
			_, err = strconv.ParseBool(raw)
			expected = "boolean (true or false)"
		case field.Type.Kind() == reflect.Int:
			_, err = strconv.Atoi(raw)
		case field.Type.Kind() == reflect.Float64:
			var value float64
			value, err = strconv.ParseFloat(raw, 64)
			if err == nil && (math.IsNaN(value) || math.IsInf(value, 0)) {
				err = fmt.Errorf("nonfinite number")
			}
			expected = "finite number"
		}
		if err != nil {
			return fmt.Errorf("%s must be a %s", key, expected)
		}
	}
	if mode := strings.TrimSpace(os.Getenv("LIBRARRY_COMPLETED_IMPORT_MODE")); mode != "" {
		switch strings.ToLower(mode) {
		case "hardlinkorcopy", "hardlink", "copy":
		default:
			return fmt.Errorf("LIBRARRY_COMPLETED_IMPORT_MODE must be hardlinkOrCopy, hardlink, or copy")
		}
	}
	return nil
}
