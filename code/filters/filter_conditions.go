package filters

import (
	"github.com/rs/zerolog/log"
)

// FilterCondition represents a condition that can be applied to filter objects
type FilterCondition interface {
	Apply(obj map[string]any) map[string]any
	Name() string
}

// FilterConditions manages multiple filter conditions
type FilterConditions struct {
	conditions []FilterCondition
}

// NewFilterConditions creates a new FilterConditions instance with default conditions
func NewFilterConditions() *FilterConditions {
	return &FilterConditions{
		conditions: []FilterCondition{
			&MetadataFilterCondition{},
			// Add more filter conditions here in the future
		},
	}
}

// ApplyAll applies all filter conditions to an object
func (fc *FilterConditions) ApplyAll(obj map[string]any) map[string]any {
	if obj == nil {
		return nil
	}

	filtered := obj
	for _, condition := range fc.conditions {
		filtered = condition.Apply(filtered)
		log.Debug().
			Str("condition", condition.Name()).
			Msg("applied filter condition")
	}

	return filtered
}
