package balancer

import "github.com/nano-vllm/go-serving/internal/backend"

type LeastConn struct{}

func (l *LeastConn) Pick(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}
	var pick *backend.Backend
	min := int64(-1)
	for _, b := range backends {
		c := b.ActiveConns.Load()
		if min < 0 || c < min {
			min = c
			pick = b
		}
	}
	return pick
}
