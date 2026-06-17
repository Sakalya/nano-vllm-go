package balancer

import "github.com/nano-vllm/go-serving/internal/backend"

type Balancer interface {
	Pick(backends []*backend.Backend) *backend.Backend
}

func New(strategy string) Balancer {
	switch strategy {
	case "least_conn":
		return &LeastConn{}
	default:
		return &RoundRobin{}
	}
}
