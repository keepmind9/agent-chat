package mcp

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/keepmind9/agent-chat/internal/notify"
)

func TestNewWSClient(t *testing.T) {
	n := notify.NopNotifier{}
	l := slog.Default()
	client := NewWSClient("http://localhost:8080", "test-agent", n, l)

	assert.Equal(t, "http://localhost:8080", client.serverURL)
	assert.Equal(t, "test-agent", client.agentName)
	assert.Equal(t, n, client.notifier)
	require.NotNil(t, client.logger)
	require.NotNil(t, client.stopCh)
	assert.Nil(t, client.conn)
}

func TestWSURL(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		agentName string
		expected  string
	}{
		{
			name:      "http to ws",
			serverURL: "http://localhost:8080",
			agentName: "agent1",
			expected:  "ws://localhost:8080/ws?agent=agent1",
		},
		{
			name:      "https to wss",
			serverURL: "https://example.com",
			agentName: "agent2",
			expected:  "wss://example.com/ws?agent=agent2",
		},
		{
			name:      "trailing slash stripped",
			serverURL: "http://localhost:8080/",
			agentName: "agent3",
			expected:  "ws://localhost:8080/ws?agent=agent3",
		},
		{
			name:      "bare ws URL passed through",
			serverURL: "ws://already-ws.com",
			agentName: "agent4",
			expected:  "ws://already-ws.com/ws?agent=agent4",
		},
		{
			name:      "bare wss URL passed through",
			serverURL: "wss://secure.example.com",
			agentName: "agent5",
			expected:  "wss://secure.example.com/ws?agent=agent5",
		},
		{
			name:      "port preserved",
			serverURL: "https://example.com:8443/api",
			agentName: "agent7",
			expected:  "wss://example.com:8443/api/ws?agent=agent7",
		},
		{
			name:      "empty path",
			serverURL: "http://localhost",
			agentName: "agent8",
			expected:  "ws://localhost/ws?agent=agent8",
		},
		{
			name:      "agent with underscore and dash",
			serverURL: "http://localhost:8080",
			agentName: "my_agent-v2",
			expected:  "ws://localhost:8080/ws?agent=my_agent-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWSClient(tt.serverURL, tt.agentName, nil, slog.Default())
			assert.Equal(t, tt.expected, client.wsURL())
		})
	}
}

func TestStop(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())

	client.Stop()
	client.Stop() // Idempotent; must not panic
}

func TestSleepWithStop_Timeout(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())

	start := time.Now()
	result := client.sleepWithStop(100 * time.Millisecond)
	elapsed := time.Since(start)

	assert.False(t, result)
	assert.GreaterOrEqual(t, elapsed, 90*time.Millisecond)
	assert.Less(t, elapsed, 200*time.Millisecond)
}

func TestSleepWithStop_StopCh(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())

	client.Stop()

	start := time.Now()
	result := client.sleepWithStop(10 * time.Second)
	elapsed := time.Since(start)

	assert.True(t, result)
	assert.Less(t, elapsed, 200*time.Millisecond)
}

func TestJitter_Bounds(t *testing.T) {
	base := 1 * time.Second

	for i := 0; i < 200; i++ {
		j := jitter(base)
		assert.GreaterOrEqual(t, j, base/2, "jitter %v should be >= %v", j, base/2)
		assert.LessOrEqual(t, j, base, "jitter %v should be <= %v", j, base)
	}
}

func TestJitter_ZeroDuration(t *testing.T) {
	j := jitter(0)
	assert.Equal(t, time.Duration(0), j)
}

func TestJitter_VerySmallDuration(t *testing.T) {
	j := jitter(1 * time.Nanosecond)
	assert.GreaterOrEqual(t, j, time.Duration(0))
	assert.LessOrEqual(t, j, 1*time.Nanosecond)
}

func TestJitter_Distribution(t *testing.T) {
	// For base=10s, jitter range is [5s, 10s].
	// We divide [5s, 10s] into 5 buckets of 1s each and verify
	// that with 1000 samples all buckets receive at least some hits.
	base := 10 * time.Second
	buckets := make([]int, 5)
	samples := 1000

	for i := 0; i < samples; i++ {
		j := jitter(base)
		// bucket 0: [5s, 6s), bucket 1: [6s, 7s), ..., bucket 4: [9s, 10s]
		b := int((j - base/2).Seconds())
		if b < 0 {
			b = 0
		}
		if b > 4 {
			b = 4
		}
		buckets[b]++
	}

	for i := 0; i < 5; i++ {
		assert.Greater(t, buckets[i], 0, "bucket %d ([%ds, %ds)) should have at least 1 sample, got %d",
			i, 5+i, 6+i, buckets[i])
	}
}

func TestJitter_DeterministicHalf(t *testing.T) {
	base := 3 * time.Second
	j := jitter(base)
	assert.GreaterOrEqual(t, j, base/2)
	assert.LessOrEqual(t, j, base)
}

func TestJitter_LargeDuration(t *testing.T) {
	base := 1 * time.Hour
	for i := 0; i < 50; i++ {
		j := jitter(base)
		assert.GreaterOrEqual(t, j, 30*time.Minute)
		assert.LessOrEqual(t, j, 1*time.Hour)
	}
}

// stopped is a test helper to check if stopCh is closed.
func (c *WSClient) stopped() bool {
	select {
	case <-c.stopCh:
		return true
	default:
		return false
	}
}

func TestStopped_Initial(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())
	assert.False(t, client.stopped(), "stopCh should be open initially")
}

func TestStopped_AfterStop(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())
	client.Stop()
	assert.True(t, client.stopped(), "stopCh should be closed after Stop()")
}

// computeNextInterval mirrors the backoff logic in Run() so it can be tested independently.
func computeNextInterval(current time.Duration, initInterval, maxInterval time.Duration, multiplier float64) time.Duration {
	return min(time.Duration(float64(current)*multiplier), maxInterval)
}

func TestComputeNextInterval_ExponentialGrowth(t *testing.T) {
	const (
		initInterval = 1 * time.Second
		maxInterval  = 30 * time.Second
		multiplier   = 2.0
	)

	tests := []struct {
		name     string
		current  time.Duration
		expected time.Duration
	}{
		{"1s doubles to 2s", 1 * time.Second, 2 * time.Second},
		{"2s doubles to 4s", 2 * time.Second, 4 * time.Second},
		{"4s doubles to 8s", 4 * time.Second, 8 * time.Second},
		{"8s doubles to 16s", 8 * time.Second, 16 * time.Second},
		{"16s doubles to 30s (capped)", 16 * time.Second, 30 * time.Second},
		{"30s stays at 30s (max)", 30 * time.Second, 30 * time.Second},
		{"31s stays at 30s (over max)", 31 * time.Second, 30 * time.Second},
		{"60s stays at 30s (over max)", 60 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeNextInterval(tt.current, initInterval, maxInterval, multiplier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeNextInterval_DifferentMultipliers(t *testing.T) {
	const (
		initInterval = 1 * time.Second
		maxInterval  = 100 * time.Second
		multiplier   = 1.5
	)

	assert.Equal(t, 1500*time.Millisecond, computeNextInterval(1*time.Second, initInterval, maxInterval, multiplier))
	assert.Equal(t, 2250*time.Millisecond, computeNextInterval(1500*time.Millisecond, initInterval, maxInterval, multiplier))
}

func TestComputeNextInterval_OneSecondMultiplier(t *testing.T) {
	const (
		initInterval = 5 * time.Second
		maxInterval  = 30 * time.Second
		multiplier   = 1.0
	)

	// With multiplier 1.0, interval never grows
	assert.Equal(t, 5*time.Second, computeNextInterval(5*time.Second, initInterval, maxInterval, multiplier))
	assert.Equal(t, 5*time.Second, computeNextInterval(5*time.Second, initInterval, maxInterval, multiplier))
}

func TestComputeNextInterval_SmallMaxInterval(t *testing.T) {
	const (
		initInterval = 1 * time.Second
		maxInterval  = 2 * time.Second
		multiplier   = 2.0
	)

	assert.Equal(t, 2*time.Second, computeNextInterval(1*time.Second, initInterval, maxInterval, multiplier))
	assert.Equal(t, 2*time.Second, computeNextInterval(2*time.Second, initInterval, maxInterval, multiplier))
	assert.Equal(t, 2*time.Second, computeNextInterval(4*time.Second, initInterval, maxInterval, multiplier))
}
