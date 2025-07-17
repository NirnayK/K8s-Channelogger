// Package validation provides functions to validate Kubernetes AdmissionRequests
// for various resource types and trigger tasks based on status changes.
package validation

import (
	"fmt"
	"channelog/helpers"

	admissionv1 "k8s.io/api/admission/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

// ValidateInferenceHpaStatus inspects an AdmissionRequest for a HorizontalPodAutoscaler
// and returns true if the HPA's CurrentReplicas count has changed or if either
// the old or new replica count is zero, indicating a scale event of interest.
//
// It returns a boolean indicating whether to enqueue a task, and an error if validation fails.
func ValidateInferenceHpaStatus(request *admissionv1.AdmissionRequest) (bool, error) {

	// Decode old and new objects from the AdmissionRequest
	oldObj, newObj, err := helpers.GetObjects(request)
	if err != nil {
		return false, fmt.Errorf("get objects: %w", err)
	}

	// Both old and new versions must be present
	if oldObj == nil || newObj == nil {
		return false, fmt.Errorf("old or new object missing")
	}

	// Assert that the objects are HorizontalPodAutoscaler types
	oldHpaObj, ok1 := oldObj.(*autoscalingv2.HorizontalPodAutoscaler)
	newHpaObj, ok2 := newObj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok1 || oldHpaObj == nil || !ok2 || newHpaObj == nil {
		return false, fmt.Errorf("object is not a HorizontalPodAutoscaler")
	}

	// Trigger if the replica count has changed
	if oldHpaObj.Status.CurrentReplicas != newHpaObj.Status.CurrentReplicas {
		return true, nil
	}

	// Also trigger if either old or new replica count is zero (scale-to-zero or scale-from-zero)
	if oldHpaObj.Status.CurrentReplicas == 0 || newHpaObj.Status.CurrentReplicas == 0 {
		return true, nil
	}

	// No relevant change detected
	return false, nil
}
