package balancer

import "sync"

var (
	ActiveNodes = make(map[string]bool)
	HealthMutex sync.RWMutex
)

type (
	// RoundrobinBalancer balancer
	RoundrobinBalancer struct {
		current int
		mu      sync.RWMutex
	}
)

// NewRoundrobinBalancer creates a new instance of Roundrobin
func NewRoundrobinBalancer() *RoundrobinBalancer {
	return &RoundrobinBalancer{}
}

// Elect backend using roundrobin strategy
func (b *RoundrobinBalancer) Elect(hosts []*Target) (*Target, error) {
	if len(hosts) == 0 {
		return nil, ErrEmptyBackendList
	}

	for i := 0; i < len(hosts); i++ {
		b.mu.Lock()
		if b.current >= len(hosts) {
			b.current = 0
		}
		host := hosts[b.current]
		b.current++
		b.mu.Unlock()

		if isTargetAlive(host.Target) {
			return host, nil
		}
	}

	return nil, ErrEmptyBackendList
}

func isTargetAlive(target string) bool {
	HealthMutex.RLock()
	defer HealthMutex.RUnlock()

	if len(ActiveNodes) == 0 {
		return true
	}

	isAlive, ok := ActiveNodes[target]
	return ok && isAlive
}
