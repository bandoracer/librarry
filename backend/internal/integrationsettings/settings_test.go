package integrationsettings

import (
	"context"
	"strconv"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
)

func TestFromResourcesAppliesPersistedIntegrationSettings(t *testing.T) {
	store := &fakeResourceStore{resources: []compatdata.Resource{
		{
			ResourceType: "indexer",
			CompatID:     1,
			Name:         "Prowlarr",
			Payload: map[string]any{
				"enable": true,
				"fields": []any{
					map[string]any{"name": "baseUrl", "value": "http://prowlarr.local/"},
					map[string]any{"name": "apiKey", "value": "********"},
				},
			},
		},
		{
			ResourceType: "download-client",
			CompatID:     1,
			Name:         "qBittorrent",
			Payload: map[string]any{
				"implementation": "qBittorrent",
				"fields": []any{
					map[string]any{"name": "host", "value": "http://qbit.local/"},
					map[string]any{"name": "username", "value": "ryan"},
					map[string]any{"name": "password", "value": "qbit-secret"},
					map[string]any{"name": "category", "value": "book-torrents"},
					map[string]any{"name": "librarryAudiobookCategory", "value": "book-audio"},
					map[string]any{"name": "librarryBookTorrentRoot", "value": "/downloads/books"},
				},
			},
		},
		{
			ResourceType: "download-client",
			CompatID:     2,
			Name:         "SABnzbd",
			Payload: map[string]any{
				"implementation": "SABnzbd",
				"fields": []any{
					map[string]any{"name": "host", "value": "http://sab.local"},
					map[string]any{"name": "apiKey", "value": "sab-secret"},
				},
			},
		},
	}}

	config, err := FromResources(context.Background(), store, acquisition.IntegrationConfig{
		ProwlarrAPIKey: "env-prowlarr-secret",
	})
	if err != nil {
		t.Fatalf("from resources: %v", err)
	}
	if config.ProwlarrURL != "http://prowlarr.local" || config.ProwlarrAPIKey != "env-prowlarr-secret" {
		t.Fatalf("unexpected prowlarr config: %+v", config)
	}
	if config.QBittorrentURL != "http://qbit.local" || config.QBittorrentUser != "ryan" || config.QBittorrentPass != "qbit-secret" {
		t.Fatalf("unexpected qBittorrent config: %+v", config)
	}
	if config.SABnzbdURL != "http://sab.local" || config.SABnzbdAPIKey != "sab-secret" {
		t.Fatalf("unexpected SABnzbd config: %+v", config)
	}
	if config.EbookCategory != "book-torrents" || config.AudiobookCategory != "book-audio" || config.BookTorrentRoot != "/downloads/books" {
		t.Fatalf("unexpected category/root config: %+v", config)
	}
}

func TestApplyPatchPreservesBlankSecretsAndSupportsClears(t *testing.T) {
	blank := ""
	nextSecret := "new-key"
	config := ApplyPatch(acquisition.IntegrationConfig{
		ProwlarrAPIKey: "existing-key",
		SABnzbdAPIKey:  "existing-sab-key",
	}, Patch{
		ProwlarrAPIKey: &blank,
		SABnzbdAPIKey:  &nextSecret,
	})
	if config.ProwlarrAPIKey != "existing-key" {
		t.Fatalf("expected blank secret to preserve existing key, got %q", config.ProwlarrAPIKey)
	}
	if config.SABnzbdAPIKey != "new-key" {
		t.Fatalf("expected new SABnzbd key, got %q", config.SABnzbdAPIKey)
	}

	config = ApplyPatch(config, Patch{ClearProwlarrAPIKey: true})
	if config.ProwlarrAPIKey != "" {
		t.Fatalf("expected clear flag to remove prowlarr key, got %q", config.ProwlarrAPIKey)
	}
}

func TestSaveResourcesPersistsCompatRecords(t *testing.T) {
	store := &fakeResourceStore{}
	err := SaveResources(context.Background(), store, acquisition.IntegrationConfig{
		ProwlarrURL:       "http://prowlarr.local",
		ProwlarrAPIKey:    "prowlarr-secret",
		QBittorrentURL:    "http://qbit.local",
		QBittorrentUser:   "admin",
		QBittorrentPass:   "qbit-secret",
		SABnzbdURL:        "http://sab.local",
		SABnzbdAPIKey:     "sab-secret",
		EbookCategory:     "books-ebook",
		AudiobookCategory: "books-audiobook",
		BookTorrentRoot:   "/data/torrents/books",
	})
	if err != nil {
		t.Fatalf("save resources: %v", err)
	}
	if len(store.resourcesByKey) != 3 {
		t.Fatalf("expected indexer plus two download client records, got %#v", store.resourcesByKey)
	}
	qbit := store.resourcesByKey["download-client:1"]
	if resourceFieldString(qbit.Payload, "host") != "http://qbit.local" || resourceFieldString(qbit.Payload, "password") != "qbit-secret" {
		t.Fatalf("unexpected qBittorrent payload: %#v", qbit.Payload)
	}
}

type fakeResourceStore struct {
	resources      []compatdata.Resource
	resourcesByKey map[string]compatdata.Resource
}

func (f *fakeResourceStore) ListResources(_ context.Context, resourceType string) ([]compatdata.Resource, error) {
	var resources []compatdata.Resource
	for _, resource := range f.resources {
		if resource.ResourceType == resourceType {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (f *fakeResourceStore) UpsertResource(_ context.Context, resource compatdata.Resource) (compatdata.Resource, error) {
	if f.resourcesByKey == nil {
		f.resourcesByKey = map[string]compatdata.Resource{}
	}
	f.resourcesByKey[resource.ResourceType+":"+strconv.Itoa(resource.CompatID)] = resource
	return resource, nil
}

func (f *fakeResourceStore) DeleteResource(_ context.Context, resourceType string, compatID int) (bool, error) {
	if f.resourcesByKey != nil {
		delete(f.resourcesByKey, resourceType+":"+strconv.Itoa(compatID))
	}
	return true, nil
}
