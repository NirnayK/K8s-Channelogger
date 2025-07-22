package filters

// MetadataFilterCondition removes commonly changing metadata fields
type MetadataFilterCondition struct{}

// Name returns the name of the filter condition
func (mfc *MetadataFilterCondition) Name() string {
	return "metadata_filter"
}

func (mfc *MetadataFilterCondition) Apply(obj map[string]any) map[string]any {
	if obj == nil {
		return nil
	}

	// Create a deep copy to avoid modifying the original
	filtered := make(map[string]any)
	for k, v := range obj {
		filtered[k] = v
	}

	// Remove or filter metadata fields that commonly change without meaningful updates
	if metadata, exists := filtered["metadata"].(map[string]any); exists {
		filteredMetadata := make(map[string]any)
		for k, v := range metadata {
			filteredMetadata[k] = v
		}

		// Remove fields that frequently change without representing meaningful updates
		delete(filteredMetadata, "resourceVersion")     // Always changes on updates
		delete(filteredMetadata, "generation")          // Changes on spec updates but not meaningful for diffs
		delete(filteredMetadata, "managedFields")       // Server-side apply metadata
		delete(filteredMetadata, "selfLink")            // Deprecated and auto-generated
		delete(filteredMetadata, "uid")                 // Unique identifier, doesn't change
		delete(filteredMetadata, "creationTimestamp")   // Set once, doesn't change

		// Filter annotations that are commonly updated by controllers
		if annotations, exists := filteredMetadata["annotations"].(map[string]any); exists {
			filteredAnnotations := make(map[string]any)
			for k, v := range annotations {
				// Skip commonly changing controller annotations
				if k != "kubectl.kubernetes.io/last-applied-configuration" &&
					k != "deployment.kubernetes.io/revision" &&
					!isControllerAnnotation(k) {
					filteredAnnotations[k] = v
				}
			}
			if len(filteredAnnotations) > 0 {
				filteredMetadata["annotations"] = filteredAnnotations
			} else {
				delete(filteredMetadata, "annotations")
			}
		}

		filtered["metadata"] = filteredMetadata
	}

	// Remove status field as it's managed by controllers and doesn't represent user intent
	delete(filtered, "status")

	return filtered
}

// filterMetadataFields removes commonly changing metadata fields that don't represent
// meaningful configuration changes. This helps focus on actual spec/configuration changes.
// Deprecated: Use MetadataFilterCondition.Apply() instead for the new interface-based approach.
func filterMetadataFields(obj map[string]any) map[string]any {
	condition := &MetadataFilterCondition{}
	return condition.Apply(obj)
}

// isControllerAnnotation checks if an annotation key is typically managed by controllers
func isControllerAnnotation(key string) bool {
	controllerPrefixes := []string{
		"controller.",
		"operator.",
		"k8s.io/",
		"kubernetes.io/",
		"autoscaling.alpha.kubernetes.io/",
		"cluster-autoscaler.kubernetes.io/",
	}

	for _, prefix := range controllerPrefixes {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
