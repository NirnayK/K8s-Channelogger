// Package config provides functionality to load and validate
// environment-backed configuration for the channelog service.
package config

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

// Config holds all of the application’s settings sourced from environment variables.
type Config struct {
	// Location is the geographic/logical location identifier for multi-region deployments.
	// This enables region-specific processing and routing in distributed setups.
	// Examples: "Delhi", "Mumbai", "Chennai", etc.
	Location string
}

// LoadConfig reads required environment variables, applies defaults,
// and validates their values before returning a Config instance.
// Returns an error if any required variable is missing or malformed.
func LoadConfig() (*Config, error) {
	// 1) LOCATION is required for multi-region deployments.
	// This identifier helps downstream consumers route and process tasks
	// according to their geographic or logical region.
	location := os.Getenv("LOCATION")
	if location == "" {
		log.Error().Msg("LOCATION is required")
		return nil, fmt.Errorf("LOCATION is required")
	}

	// 5) Return the populated Config struct.
	return &Config{
		Location:       location,
	}, nil
}
