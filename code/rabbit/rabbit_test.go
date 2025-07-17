// rabbit/manager_test.go
// Unit tests for RabbitManager's Acquire and Release methods using pool-based channels.
package rabbit

import (
	"testing"

	"channelog/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestAcquireFromPool ensures that Acquire returns a pre-existing channel from the pool.
func TestAcquireFromPool(t *testing.T) {
	// Set up a manager with a small pool
	cfg := &config.Config{MaxChannelPool: 2}
	mgr := NewRabbitManager(cfg)

	// Push a dummy channel into the pool
	ch := &amqp.Channel{}
	mgr.pool <- ch

	// Acquire should return the same channel without error
	got, err := mgr.Acquire()
	if err != nil {
		t.Fatalf("Acquire() error = %v; want no error", err)
	}
	if got != ch {
		t.Errorf("Acquire() = %p; want %p", got, ch)
	}
}

// TestReleaseToPool ensures that Release returns a healthy channel to the pool.
func TestReleaseToPool(t *testing.T) {
	cfg := &config.Config{MaxChannelPool: 1}
	mgr := NewRabbitManager(cfg)

	// Create a dummy channel and release it
	ch := &amqp.Channel{}
	mgr.Release(ch)

	// Pool should contain exactly that channel
	select {
	case got := <-mgr.pool:
		if got != ch {
			t.Errorf("Release() pushed = %p; want %p", got, ch)
		}
	default:
		t.Fatal("Release() did not push channel into pool")
	}
}

// TestReleaseBadChannel ensures that Release drops channels marked as bad,
// cleaning them from badChannels without returning them to the pool.
func TestReleaseBadChannel(t *testing.T) {
	cfg := &config.Config{MaxChannelPool: 1}
	mgr := NewRabbitManager(cfg)

	// Create and mark a dummy channel as bad
	ch := &amqp.Channel{}
	mgr.badChanMu.Lock()
	mgr.badChannels[ch] = struct{}{}
	mgr.badChanMu.Unlock()

	// Call Release and recover from potential panic in ch.Close()
	func() {
		defer func() { recover() }()
		mgr.Release(ch)
	}()

	// After Release, the pool should remain empty
	select {
	case got := <-mgr.pool:
		t.Fatalf("Release() pushed bad channel %p; want pool empty", got)
	default:
	}

	// The badChannels entry for ch should have been removed
	mgr.badChanMu.Lock()
	if _, exists := mgr.badChannels[ch]; exists {
		t.Errorf("Release() did not delete bad channel entry")
	}
	mgr.badChanMu.Unlock()
}
