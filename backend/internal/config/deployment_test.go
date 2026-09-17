package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAllRuntimeSettingsReachEveryInstaller(t *testing.T) {
	source, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	names := regexp.MustCompile(`"(LIBRARRY_[A-Z_]+)"`).FindAllStringSubmatch(string(source), -1)
	for _, variant := range []string{"deploy/docker-compose.yml", "deploy/docker-compose.build.yml", "deploy/unraid/docker-compose.yml", "deploy/truenas/install.yaml"} {
		t.Run(variant, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../..", variant))
			if err != nil {
				t.Fatal(err)
			}
			for _, match := range names {
				if !strings.Contains(string(data), match[1]+":") {
					t.Errorf("runtime setting %s is not passed to the API", match[1])
				}
			}
		})
	}
}
