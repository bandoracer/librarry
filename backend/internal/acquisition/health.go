package acquisition

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

const integrationFreshFor = 10 * time.Minute
const integrationCheckSpacing = 15 * time.Second

var ErrIntegrationUnknown = errors.New("unknown integration")
var ErrIntegrationChanged = errors.New("integration configuration changed; refresh and check again")

type integrationObservation struct {
	mu      sync.Mutex
	latest  IntegrationHealth
	running chan struct{}
}

func newIntegrationObservation() *integrationObservation { return &integrationObservation{} }

func (s *integrationState) healthAdapter(index int) (string, bool, func(context.Context) IntegrationHealth) {
	switch index {
	case 0:
		return s.prowlarr.Name(), s.prowlarr.Configured(), s.prowlarr.Health
	case 1:
		return s.qbit.Name(), s.qbit.Configured(), s.qbit.Health
	case 2:
		return s.trans.Name(), s.trans.Configured(), s.trans.Health
	default:
		return s.sab.Name(), s.sab.Configured(), s.sab.Health
	}
}

// Health is a read-only process-local snapshot; only explicit checks and the
// scheduled health worker perform external IO. Reconfiguration replaces evidence.
func (s *integrationState) Health(ctx context.Context) []IntegrationHealth {
	results := make([]IntegrationHealth, 0, 4)
	for index, observation := range s.health {
		name, configured, _ := s.healthAdapter(index)
		observation.mu.Lock()
		results = append(results, observation.snapshot(name, configured, time.Now().UTC()))
		observation.mu.Unlock()
	}
	return results
}

func (o *integrationObservation) snapshot(name string, configured bool, now time.Time) IntegrationHealth {
	h := o.latest
	h.Name, h.Configured, h.Checking = name, configured, o.running != nil
	h.Freshness = "never_checked"
	if !configured {
		h.Status = "missing_credentials"
		h.Message = "Configure the integration endpoint and required credentials."
		return h
	}
	if h.Status == "" {
		h.Status = "configured"
		h.Message = "Configured; connection has not been checked."
	}
	if h.LastCheckedAt != nil {
		h.ObservedStatus = h.Status
		h.Freshness = "fresh"
		if now.Sub(*h.LastCheckedAt) > integrationFreshFor {
			h.Freshness = "stale"
			h.Status = "stale"
			h.Message = "The last connection check is older than 10 minutes. Check again for current evidence."
		}
	}
	return h
}

func (s *integrationState) checkHealth(ctx context.Context, index int) IntegrationHealth {
	name, configured, probe := s.healthAdapter(index)
	o := s.health[index]
	o.mu.Lock()
	now := time.Now().UTC()
	if !configured || ctx.Err() != nil || (o.latest.LastCheckedAt != nil && now.Sub(*o.latest.LastCheckedAt) < integrationCheckSpacing) || (o.latest.RetryAfter != nil && now.Before(*o.latest.RetryAfter)) {
		result := o.snapshot(name, configured, now)
		o.mu.Unlock()
		return result
	}
	if pending := o.running; pending != nil {
		o.mu.Unlock()
		select {
		case <-pending:
		case <-ctx.Done():
		}
		o.mu.Lock()
		result := o.snapshot(name, configured, time.Now().UTC())
		o.mu.Unlock()
		return result
	}
	done := make(chan struct{})
	o.running = done
	o.mu.Unlock()
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	result := probe(checkCtx)
	cancel()
	o.mu.Lock()
	// A cancelled caller is not evidence that the integration failed. An actual
	// per-check timeout is an observation, but shutdown/browser cancellation is not.
	if ctx.Err() == nil {
		at := time.Now().UTC()
		result.LastCheckedAt = &at
		result.LastSuccessAt = o.latest.LastSuccessAt
		if result.Status == "ready" {
			result.LastSuccessAt = &at
		}
		if result.Version != "" {
			result.LastVersionAt = &at
		} else {
			result.Version = o.latest.Version
			result.LastVersionAt = o.latest.LastVersionAt
		}
		o.latest = result
	}
	o.running = nil
	close(done)
	result = o.snapshot(name, configured, time.Now().UTC())
	o.mu.Unlock()
	return result
}

func (s *Service) CheckIntegration(ctx context.Context, name string) (IntegrationHealth, error) {
	state := s.current.Load()
	for i := range state.health {
		candidate, _, _ := state.healthAdapter(i)
		if candidate != name {
			continue
		}
		result := state.checkHealth(ctx, i)
		if s.current.Load() != state {
			return IntegrationHealth{}, ErrIntegrationChanged
		}
		return result, nil
	}
	return IntegrationHealth{}, ErrIntegrationUnknown
}

func (s *Service) CheckIntegrations(ctx context.Context) {
	state := s.current.Load()
	var wg sync.WaitGroup
	for i := range state.health {
		wg.Add(1)
		go func(index int) { defer wg.Done(); state.checkHealth(ctx, index) }(i)
	}
	wg.Wait()
}

// Health checks use their own redirect policy and Transmission session; they
// cannot alter in-flight acquisition commands or forward credentials elsewhere.
func healthClient(original *http.Client) *http.Client {
	copy := *original
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

// HealthSnapshot keeps configuration and observations in the same generation.
func (s *Service) HealthSnapshot() (IntegrationConfig, []IntegrationHealth) {
	state := s.current.Load()
	return state.IntegrationConfig(), state.Health(context.Background())
}
