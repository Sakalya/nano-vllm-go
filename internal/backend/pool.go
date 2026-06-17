package backend

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nano-vllm/go-serving/internal/config"
)

type Backend struct {
	URL         string
	Weight      int
	ActiveConns atomic.Int64
	Healthy     atomic.Bool
}

func NewBackend(cfg config.BackendConfig) *Backend {
	b := &Backend{URL: cfg.URL, Weight: cfg.Weight}
	b.Healthy.Store(true)
	return b
}

type Pool struct {
	mu       sync.RWMutex
	backends []*Backend
	client   *http.Client
}

func NewPool(backends []*Backend, cfg config.PoolConfig) *Pool {
	transport := &http.Transport{
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
	}
	return &Pool{
		backends: backends,
		client:   &http.Client{Transport: transport},
	}
}

func (p *Pool) Backends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*Backend, len(p.backends))
	copy(out, p.backends)
	return out
}

func (p *Pool) HealthyBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*Backend
	for _, b := range p.backends {
		if b.Healthy.Load() {
			out = append(out, b)
		}
	}
	return out
}

func (p *Pool) Client() *http.Client {
	return p.client
}

func (p *Pool) StartHealthChecks(ctx context.Context, cfg config.HealthCheckConfig) {
	for _, b := range p.backends {
		go p.checkLoop(ctx, b, cfg)
	}
}

func (p *Pool) checkLoop(ctx context.Context, b *Backend, cfg config.HealthCheckConfig) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.probe(b, cfg)
		}
	}
}

func (p *Pool) probe(b *Backend, cfg config.HealthCheckConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, b.URL+cfg.Path, nil)
	resp, err := p.client.Do(req)
	if err != nil || resp.StatusCode >= 500 {
		if b.Healthy.Swap(false) {
			slog.Warn("backend unhealthy", "url", b.URL)
		}
		return
	}
	resp.Body.Close()
	if !b.Healthy.Swap(true) {
		slog.Info("backend recovered", "url", b.URL)
	}
}
