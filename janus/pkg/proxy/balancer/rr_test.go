package balancer

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RoundRobinTestSuite struct {
	suite.Suite
	hosts []*Target
}

func (suite *RoundRobinTestSuite) SetupTest() {
	HealthMutex.Lock()
	ActiveNodes = make(map[string]bool)
	HealthMutex.Unlock()

	suite.hosts = []*Target{
		{Target: "127.0.0.1", Weight: 5},
		{Target: "http://test.com", Weight: 10},
		{Target: "http://example.com", Weight: 8},
	}
}

func (suite *RoundRobinTestSuite) TestRoundRobinBalancerSuccessfulBalance() {
	balancer := NewRoundrobinBalancer()

	electedHost, err := balancer.Elect(suite.hosts)
	suite.NoError(err)
	suite.Equal(suite.hosts[0], electedHost)

	electedHost, err = balancer.Elect(suite.hosts)
	suite.NoError(err)
	suite.Equal(suite.hosts[1], electedHost)

	electedHost, err = balancer.Elect(suite.hosts)
	suite.NoError(err)
	suite.Equal(suite.hosts[2], electedHost)

	electedHost, err = balancer.Elect(suite.hosts)
	suite.NoError(err)
	suite.Equal(suite.hosts[0], electedHost)
}

func (suite *RoundRobinTestSuite) TestRoundRobinBalancerEmptyList() {
	balancer := NewRoundrobinBalancer()

	_, err := balancer.Elect([]*Target{})
	suite.Error(err)
}

func (suite *RoundRobinTestSuite) TestRoundRobinBalancerAllowsLocalJanusTarget() {
	HealthMutex.Lock()
	ActiveNodes = map[string]bool{
		"http://product-service-1:9002": true,
	}
	HealthMutex.Unlock()

	balancer := NewRoundrobinBalancer()
	localTarget := []*Target{{Target: "http://127.0.0.1:8880/notifications"}}

	electedHost, err := balancer.Elect(localTarget)

	suite.NoError(err)
	suite.Equal(localTarget[0], electedHost)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRoundRobinTestSuite(t *testing.T) {
	suite.Run(t, new(RoundRobinTestSuite))
}
