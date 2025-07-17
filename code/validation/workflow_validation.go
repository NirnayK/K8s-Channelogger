// Package validation provides functions to validate Kubernetes AdmissionRequests
// for various resources and determine whether tasks should be enqueued based
// on state transitions or conditions.
package validation

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"
)

// IsValidWorkflowTask inspects an AdmissionRequest for an Argo Workflow
// and returns true if the Workflow has entered any phase (i.e., its Status.Phase is non-empty).
//
// Steps:
//  1. Decode the old and new objects from the AdmissionRequest via helpers.GetObjects.
//  2. Assert that the new object is an *argo.Workflow.
//  3. Check the Workflow's Status.Phase field.
//     - If non-empty, return true (enqueue a task).
//     - If empty, return false with an error indicating missing phase.
func IsValidWorkflowTask(request *admissionv1.AdmissionRequest) (bool, error) {

	// Decode new object;
	var rawObject map[string]interface{}
	if err := json.Unmarshal(request.Object.Raw, &rawObject); err != nil {
		// If parsing fails, log the error and proceed (raw JSON may still be included).
		log.Error().Err(err).Msg("could not parse raw JSON into map")
	}

	// Ensure the decoded object is an Argo Workflow
	status, ok := rawObject["status"]
	if !ok {
		return false, fmt.Errorf("workflow status not found")
	}

	// Extract the Workflow's current phase
	statusMap, ok := status.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("workflow status is not a valid object")
	}

	phase, ok := statusMap["phase"]
	if !ok {
		return false, fmt.Errorf("workflow phase not found in status")
	}

	phaseStr, ok := phase.(string)
	if !ok {
		return false, fmt.Errorf("workflow phase is not a string")
	}

	// Phase set indicates the workflow has progressed
	if phaseStr != "" {
		return true, nil
	}

	// No phase set means workflow may not have started
	return false, fmt.Errorf("workflow phase is empty")
}
