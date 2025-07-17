// Package validation provides functions to validate Kubernetes AdmissionRequests
// and determine whether KEDA-related scaling tasks should be enqueued.
package validation

import (
	"fmt"
	"channelog/helpers"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
)

// IsValidKedaTask inspects an AdmissionRequest for a Deployment resource and
// returns true if a KEDA scaling task should be triggered based on:
//  1. A new generation of the Deployment has been observed.
//  2. The new Deployment has zero ready replicas (indicating scale-out or startup).
//
// It returns false (no task) when the generation hasn't changed or replicas exist,
// and returns an error if object decoding fails or the resource is not a Deployment.
func IsValidKedaTask(review *admissionv1.AdmissionRequest) (bool, error) {
	// Decode the old and new object versions from the AdmissionRequest
	oldObj, newObj, err := helpers.GetObjects(review)
	if err != nil {
		return false, fmt.Errorf("get objects: %w", err)
	}

	// Assert that both old and new objects are Deployments
	oldDep, okOld := oldObj.(*appsv1.Deployment)
	newDep, okNew := newObj.(*appsv1.Deployment)
	if !okOld || oldDep == nil || !okNew || newDep == nil {
		return false, fmt.Errorf("object is not a Deployment")
	}

	// Do not enqueue a task if there's no new generation to process
	if newDep.Generation <= oldDep.Generation {
		return false, nil
	}

	// Enqueue a task if the new Deployment has zero ready replicas
	// (e.g., just created or scaling up)
	if newDep.Status.ReadyReplicas < 1 {
		return true, nil
	}

	// Otherwise, no task is necessary
	return false, nil
}
