package acquisition

import (
	"context"
	"sync"
	"testing"
)

func TestConfigurationGenerationConcurrentOperations(t *testing.T) {
	service := NewService(IntegrationConfig{EbookCategory: "old", AudiobookCategory: "old"})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				service.Reconfigure(IntegrationConfig{EbookCategory: "new", AudiobookCategory: "new"})
				config := service.IntegrationConfig()
				if config.EbookCategory != config.AudiobookCategory {
					t.Error("mixed configuration generations")
				}
				_ = service.Health(context.Background())
				_, _ = service.Downloads(context.Background(), DownloadListQuery{})
				_, _ = service.Search(context.Background(), ReleaseSearchQuery{Query: "fixture"})
			}
		}()
	}
	wg.Wait()
}
