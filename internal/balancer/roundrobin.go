package balancer

import (
	"sync/atomic"

	"github.com/nano-vllm/go-serving/internal/backend"
)

type RoundRobin struct {
	counter atomic.Uint64
}

func (r *RoundRobin) Pick(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}
	idx := r.counter.Add(1) - 1
	return backends[idx%uint64(len(backends))]
}
