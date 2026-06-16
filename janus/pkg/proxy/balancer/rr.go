package balancer

import (
	"net/url"
	"sync"
)

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
	if isLocalTarget(target) {
		return true
	}

	HealthMutex.RLock()
	defer HealthMutex.RUnlock()

	if len(ActiveNodes) == 0 {
		return true
	}

	isAlive, ok := ActiveNodes[target]
	if ok {
		return isAlive
	}

	isAlive, ok = ActiveNodes[targetHealthKey(target)]
	return ok && isAlive
}

func targetHealthKey(target string) string {
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return target
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func isLocalTarget(target string) bool {
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	host := u.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
