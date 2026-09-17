package acquisition

import "context"

// DownloadEvidence never substitutes saved rows for a live client response.
// A partial response proves positive matches, but cannot prove absence.
type DownloadEvidence struct {
	Downloads []DownloadStatus `json:"downloads"`
	Status    string           `json:"status"` // fresh, partial, unavailable, notConfigured
}

func (s *Service) LiveDownloadEvidence(ctx context.Context, query DownloadListQuery) DownloadEvidence {
	return s.current.Load().liveDownloadEvidence(ctx, query)
}

func (s *integrationState) liveDownloadEvidence(ctx context.Context, query DownloadListQuery) DownloadEvidence {
	result := DownloadEvidence{Downloads: []DownloadStatus{}, Status: "notConfigured"}
	configured, succeeded := 0, 0
	clients := []struct {
		name       string
		configured bool
		list       func(context.Context, DownloadListQuery) ([]DownloadStatus, error)
	}{
		{s.qbit.Name(), s.qbit.Configured(), s.qbit.List},
		{s.trans.Name(), s.trans.Configured(), s.trans.List},
		{s.sab.Name(), s.sab.Configured(), s.sab.List},
	}
	for _, client := range clients {
		if !client.configured || !s.includeClient(query, client.name) {
			continue
		}
		configured++
		downloads, err := client.list(ctx, query)
		if err != nil {
			continue
		}
		succeeded++
		result.Downloads = append(result.Downloads, downloads...)
	}
	switch {
	case configured == 0:
	case succeeded == configured:
		result.Status = "fresh"
	case succeeded == 0:
		result.Status = "unavailable"
	default:
		result.Status = "partial"
	}
	// Saved import bookkeeping can exclude an already-imported source, but it
	// never supplies a download absent from the successful live responses.
	result.Downloads = s.mergeStoredDownloadState(ctx, result.Downloads, query)
	return result
}
