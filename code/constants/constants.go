// Package constants defines shared values and patterns used across the channelog services.
package constants

import "time"

const (
	// DefaultMaxPool is the default maximum number of channels in the RabbitMQ channel pool.
	DefaultMaxPool = 50

	// BackoffMax is the maximum duration to wait between reconnection attempts
	// when dialing RabbitMQ, after which the backoff stops growing.
	// Used with jittered exponential backoff to prevent thundering herd problems.
	BackoffMax = 30 * time.Second

	RabbitMQConnectionError = "connection is not open"

	NodeRequestKind = "Node"
)

// RemoveAttrs defines Kubernetes object attributes that are stripped from payloads
// before sending to downstream consumers. These attributes are removed to:
// - Reduce payload size and network overhead
// - Remove unnecessary data that downstream consumers don't need
var RemoveAttrs = []string{
	"metadata.managedFields", // Verbose field management metadata
	"spec.affinity",          // Pod scheduling preferences (not needed for billing/inventory)
	"spec.containers",        // Container specifications (too verbose for most use cases)
	"spec.securityContext",   // Security context details (not needed for tracking)
	"spec.tolerations",       // Pod toleration rules (not needed for billing/inventory)
}
