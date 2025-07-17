// Package config provides functionality to load and validate
// environment-backed configuration for the channelog service.
package config

import (
	"fmt"
	"os"
	"strconv"

	"channelog/constants"

	"github.com/rs/zerolog/log"
)

// Config holds all of the application’s settings sourced from environment variables.
type Config struct {
	// AMQPURL is the connection string used to dial RabbitMQ.
	AMQPURL string
	// QueueName is the name of the RabbitMQ queue to which tasks will be published.
	QueueName string
	// MaxChannelPool is the maximum number of channels in the RabbitManager’s pool.
	MaxChannelPool int
	// Location is the geographic/logical location identifier for multi-region deployments.
	// This enables region-specific processing and routing in distributed setups.
	// Examples: "Delhi", "Mumbai", "Chennai", etc.
	Location string
}

// LoadConfig reads required environment variables, applies defaults,
// and validates their values before returning a Config instance.
// Returns an error if any required variable is missing or malformed.
func LoadConfig() (*Config, error) {
	// 1) RABBITMQ_URL must be set to connect to the broker.
	broker := os.Getenv("RABBITMQ_URL")
	if broker == "" {
		log.Error().Msg("RABBITMQ_URL is required")
		return nil, fmt.Errorf("RABBITMQ_URL is required")
	}

	// 2) DEFAULT_RABBITMQ_QUEUE must be set to specify the target queue.
	queue := os.Getenv("DEFAULT_RABBITMQ_QUEUE")
	if queue == "" {
		log.Error().Msg("DEFAULT_RABBITMQ_QUEUE is required")
		return nil, fmt.Errorf("DEFAULT_RABBITMQ_QUEUE is required")
	}

	// 3) MAX_CHANNEL_POOL_SIZE is optional; default to constants.DefaultMaxPool if unset.
	poolStr := os.Getenv("MAX_CHANNEL_POOL_SIZE")
	if poolStr == "" {
		poolStr = strconv.Itoa(constants.DefaultMaxPool)
	}

	// 4) Convert the pool size string to an integer.
	pool, err := strconv.Atoi(poolStr)
	if err != nil {
		log.Error().Msgf("invalid MAX_CHANNEL_POOL_SIZE %q: %v", poolStr, err)
		return nil, fmt.Errorf("invalid MAX_CHANNEL_POOL_SIZE %q: %w", poolStr, err)
	}

	// 5) LOCATION is required for multi-region deployments.
	// This identifier helps downstream consumers route and process tasks
	// according to their geographic or logical region.
	location := os.Getenv("LOCATION")
	if location == "" {
		log.Error().Msg("LOCATION is required")
		return nil, fmt.Errorf("LOCATION is required")
	}

	// 5) Return the populated Config struct.
	return &Config{
		AMQPURL:        broker,
		QueueName:      queue,
		MaxChannelPool: pool,
		Location:       location,
	}, nil
}
