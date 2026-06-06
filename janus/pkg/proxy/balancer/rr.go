package balancer

import (
	"strings"
	"sync"
)

// ActiveNodes và HealthMutex
var (
	ActiveNodes = make(map[string]bool)
	HealthMutex sync.RWMutex
)

type (
	// RoundrobinBalancer balancer
	RoundrobinBalancer struct {
		current int // current backend position
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

	// Lặp tối đa qua tất cả các host để tìm node CÒN SỐNG
	for i := 0; i < len(hosts); i++ {
		b.mu.Lock()
		if b.current >= len(hosts) {
			b.current = 0
		}
		host := hosts[b.current]
		b.current++
		b.mu.Unlock()

		// Đọc sổ tay Health Check xem node này có bị DOWN không
		isAlive := true
		HealthMutex.RLock()
		for name, status := range ActiveNodes {
			// So khớp tên (vd: product-service-1) với đích đến
			if strings.Contains(host.Target, name) {
				isAlive = status
				break
			}
		}
		HealthMutex.RUnlock()

		// CHÌA KHÓA Ở ĐÂY: Nếu node còn sống -> Chọn luôn!
		// Nếu đã chết -> Vòng lặp sẽ tự động bỏ qua và thử tìm node tiếp theo.
		if isAlive {
			return host, nil
		}
	}

	// Nếu lặp qua hết mà tất cả các node đều chết sạch
	return nil, ErrEmptyBackendList
}
