package rabbit

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"channelog/config"
	"channelog/constants"
	"channelog/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

// local random source for jittered backoff to prevent thundering herd problems
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

const (
	// defaultExchange is the RabbitMQ exchange to publish to (empty = default vhost).
	defaultExchange = ""
	// defaultMandatory controls whether messages must be routed or returned.
	defaultMandatory = false
	// defaultImmediate controls whether to return if there is no live consumer.
	defaultImmediate = false
	// maxPublishAttempts is the total number of times we'll try to publish before giving up.
	maxPublishAttempts = 2
)

type RabbitManager struct {
	cfg  *config.Config
	conn *amqp.Connection
	pool chan *amqp.Channel
	mu   sync.RWMutex

	// badChannels tracks channels that have encountered errors and should be discarded.
	// This prevents reuse of stale or broken channels that could cause publish failures.
	// Channels become "bad" when they receive close notifications from RabbitMQ.
	badChannels map[*amqp.Channel]struct{}
	badChanMu   sync.Mutex

	done chan struct{}
}

// NewRabbitManager creates a connection manager with channel pooling for high-performance publishing.
//
// Channel pooling is implemented because:
// - Opening/closing channels for each publish is expensive
// - Reusing channels improves throughput and reduces latency
// - Pool management handles connection failures gracefully
// - Bad channel tracking prevents reuse of broken channels
func NewRabbitManager(cfg *config.Config) *RabbitManager {
	return &RabbitManager{
		cfg:         cfg,
		pool:        make(chan *amqp.Channel, cfg.MaxChannelPool),
		badChannels: make(map[*amqp.Channel]struct{}),
		done:        make(chan struct{}),
	}
}

// Start kicks off the reconnection + pool-management loop.
func (r *RabbitManager) Start() {
	go r.reconnectLoop()
}

// Stop shuts everything down.
func (r *RabbitManager) Stop() {
	close(r.done)

	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close()
	}
	// draining the pool
	close(r.pool)
	for ch := range r.pool {
		ch.Close()
	}
	r.mu.Unlock()
}

// handleConnectionError closes the current AMQP connection when we receive the
// canonical “connection is not open” error.  This triggers NotifyClose(), which
// the reconnectLoop is already listening to, so the loop will establish a new
// connection and rebuild the channel pool.
func (r *RabbitManager) handleConnectionError(err error) {
	if err == amqp.ErrClosed || strings.Contains(err.Error(), constants.RabbitMQConnectionError) {
		log.Warn().Err(err).Msg("AMQP connection appears dead – forcing reconnect")
		r.mu.Lock()
		if r.conn != nil {
			_ = r.conn.Close() // idempotent; safe if already closed
		}
		r.mu.Unlock()
	}
}

// Acquire returns a healthy channel from the pool or opens a new one.
// If the underlying connection has silently died, it invokes handleConnectionError
// to trigger the reconnect loop, then surfaces the original error to the caller.
func (r *RabbitManager) Acquire() (*amqp.Channel, error) {
	defer helpers.PanicCatcher("RabbitManager.Acquire")()

	for {
		select {
		case ch := <-r.pool:
			// skip channels that were marked bad by handleChannelClose()
			r.badChanMu.Lock()
			if _, isBad := r.badChannels[ch]; isBad {
				delete(r.badChannels, ch)
				r.badChanMu.Unlock()
				ch.Close()
				continue // Try to get another channel from the pool
			}
			r.badChanMu.Unlock()
			return ch, nil

		default:
			// no pooled channel available, open a fresh one
			r.mu.RLock()
			conn := r.conn
			r.mu.RUnlock()

			if conn == nil || conn.IsClosed() {
				// Let reconnectLoop do its job; brief pause prevents hot-looping
				time.Sleep(150 * time.Millisecond)
				continue
			}

			ch, err := conn.Channel()
			if err != nil {
				// connection may be stale – force reconnect and bubble up the error
				r.handleConnectionError(err)
				return nil, err
			}

			// ensure queue exists
			if _, err = ch.QueueDeclare(
				r.cfg.QueueName, true, false, false, false, nil,
			); err != nil {
				log.Error().Err(err).Msg("queue declare on new channel failed")
				ch.Close()
				return nil, err
			}

			// watch for channel-level closures
			notify := ch.NotifyClose(make(chan *amqp.Error, 1))
			go r.handleChannelClose(ch, notify)

			return ch, nil
		}
	}
}

// Release returns the channel to the pool—or closes it if it’s bad or the pool is full.
func (r *RabbitManager) Release(ch *amqp.Channel) {
	defer helpers.PanicCatcher("RabbitManager.Release")()
	// if marked bad, drop it
	r.badChanMu.Lock()
	if _, isBad := r.badChannels[ch]; isBad {
		delete(r.badChannels, ch)
		r.badChanMu.Unlock()
		ch.Close()
		return
	}
	r.badChanMu.Unlock()

	select {
	case r.pool <- ch:
		// done
	default:
		// pool full
		ch.Close()
	}
}

// handleChannelClose watches for a channel error and flags it.
func (r *RabbitManager) handleChannelClose(ch *amqp.Channel, notify <-chan *amqp.Error) {
	if err, ok := <-notify; ok {
		log.Warn().Err(err).Msg("channel closed, marking as bad")
		r.badChanMu.Lock()
		r.badChannels[ch] = struct{}{}
		r.badChanMu.Unlock()
	}
}

// reconnectLoop maintains one persistent connection, recreates the channel pool on reconnect.
func (r *RabbitManager) reconnectLoop() {
	defer helpers.PanicCatcher("RabbitManager.reconnectLoop")()

	backoff := time.Second

	for {
		select {
		case <-r.done:
			return
		default:
		}

		cfg := amqp.Config{
			Heartbeat: 30 * time.Second,
			Dial:      amqp.DefaultDial(5 * time.Second), // TCP timeout
		}
		conn, err := amqp.DialConfig(r.cfg.AMQPURL, cfg)
		if err != nil {
			log.Error().Err(err).Msg("AMQP dial failed, retrying with backoff")

			// Jittered exponential backoff prevents thundering herd problems when multiple
			// channelog instances try to reconnect simultaneously. The jitter spreads out
			// reconnection attempts across time to reduce server load.
			jitter := time.Duration(rng.Int63n(1000)-500) * time.Millisecond
			time.Sleep(backoff + jitter)
			if backoff < constants.BackoffMax {
				backoff *= 2 // Exponential backoff
			}
			continue
		}

		// reset backoff on success
		backoff = time.Second

		// swap in new connection & fresh pool
		r.mu.Lock()
		oldConn := r.conn
		oldPool := r.pool

		r.conn = conn
		r.pool = make(chan *amqp.Channel, r.cfg.MaxChannelPool)
		r.mu.Unlock()

		// asynchronously warm up the channel pool
		go r.warmUpChannels(conn)

		log.Info().
			Str("url", r.cfg.AMQPURL).
			Int("poolSize", r.cfg.MaxChannelPool).
			Msg("connected and initializing channel pool")

		// close old resources
		if oldConn != nil {
			oldConn.Close()
		}
		close(oldPool)
		for ch := range oldPool {
			ch.Close()
		}

		// wait for connection loss or stop
		notifyConn := conn.NotifyClose(make(chan *amqp.Error, 1))
		select {
		case err := <-notifyConn:
			log.Warn().Err(err).Msg("AMQP connection closed, will reconnect")
		case <-r.done:
			conn.Close()
			return
		}
	}
}

// warmUpChannels gradually fills the channel pool in the background.
func (r *RabbitManager) warmUpChannels(conn *amqp.Connection) {
	for i := 0; i < r.cfg.MaxChannelPool; i++ {
		ch, err := conn.Channel()
		if err != nil {
			log.Error().Err(err).Msg("opening channel during warm-up failed")
			continue
		}

		if _, err = ch.QueueDeclare(
			r.cfg.QueueName, true, false, false, false, nil,
		); err != nil {
			log.Error().Err(err).Msg("queue declare during warm-up failed")
			ch.Close()
			continue
		}

		notify := ch.NotifyClose(make(chan *amqp.Error, 1))
		go r.handleChannelClose(ch, notify)

		// non-blocking send: if the pool is full, close the channel
		select {
		case r.pool <- ch:
		default:
			ch.Close()
		}
	}
}

// PublishWithRetry acquires a channel, publishes the message with up to
// maxPublishAttempts retries, and then releases the channel.
func (r *RabbitManager) PublishWithRetry(exchange, routingKey string, pub amqp.Publishing) error {
	// 1) Acquire a channel from the pool
	ch, err := r.Acquire()
	if err != nil {
		return fmt.Errorf("no channel available: %w", err)
	}
	// Ensure the channel is always returned
	defer r.Release(ch)

	// 2) Attempt to publish, retrying on failure
	for attempt := 1; attempt <= maxPublishAttempts; attempt++ {
		if err = ch.Publish(
			exchange,
			routingKey,
			defaultMandatory,
			defaultImmediate,
			pub,
		); err == nil {
			// success
			return nil
		}

		// log and decide whether to retry
		log.Error().
			Err(err).
			Int("attempt", attempt).
			Msg("failed to publish task, retrying")

		if attempt == maxPublishAttempts {
			return fmt.Errorf("failed to publish after %d attempts: %w", attempt, err)
		}

		// recycle the channel and retry
		r.Release(ch)
		ch, err = r.Acquire()
		if err != nil {
			return fmt.Errorf("no channel available on retry: %w", err)
		}
	}

	// shouldn't reach here
	return fmt.Errorf("unhandled publish retry loop exit")
}

// CheckRabbitMQ dials the given AMQP URL with a short timeout and returns
// an error if the connection cannot be established. It is used by
// health checks to verify broker availability.
func CheckRabbitMQ(url string) error {
	cfg := amqp.Config{
		Heartbeat: 5 * time.Second,
		Dial:      amqp.DefaultDial(5 * time.Second),
	}
	conn, err := amqp.DialConfig(url, cfg)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
