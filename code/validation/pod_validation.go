// Package validation provides functions to validate Kubernetes AdmissionRequests
// for Pod binding and status change events, deciding when to enqueue tasks.
package validation

import (
	"fmt"
	"channelog/constants"
	"channelog/helpers"

	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
)

// ValidateBindingPod returns true once a Pod Binding admission has been
// assigned a node, indicating the scheduler set the .Spec.NodeName.
//
// Steps:
//  1. Decode the new Binding object from the AdmissionRequest.
//  2. Assert the object type is *corev1.Binding.
//  3. Return true if Binding.Target.Name (node) is non-empty.
func ValidateBindingPod(request *admissionv1.AdmissionRequest) (bool, error) {
	// Extract only the new object; ignore the old version
	_, newObj, err := helpers.GetObjects(request)
	if err != nil {
		return false, fmt.Errorf("get objects: %w", err)
	}

	// Must have a new object to validate
	if newObj == nil {
		return false, fmt.Errorf("no new object to validate")
	}

	// Ensure the new object is a corev1.Binding
	binding, ok := newObj.(*corev1.Binding)
	if !ok {
		return false, fmt.Errorf("object is not a Binding resource")
	}

	// Log the node name for debugging
	log.Info().Msgf("Binding.Target.Name: %s", binding.Target.Name)

	// Return true when a node has been assigned
	return binding.Target.Name != "", nil
}

// ValidatePodStatusChange checks for Pod deletion or readiness transitions
// and returns a task name string when a significant event occurs.
//
// Steps:
//  1. Decode old and new Pod objects.
//  2. Assert types and existence.
//  3. If DeletionTimestamp set on newPod, return PodDeletionTask.
//  4. If PodReady condition transitions true, return PodStatusTask.
//  5. Otherwise, return empty string.
func ValidatePodStatusChange(request *admissionv1.AdmissionRequest) (bool, error) {
	// Decode both old and new objects
	oldObj, newObj, err := helpers.GetObjects(request)
	if err != nil {
		return false, fmt.Errorf("get objects: %w", err)
	}

	// Both old and new must be present
	if oldObj == nil || newObj == nil {
		return false, fmt.Errorf("old or new object missing")
	}

	// Assert that both are *corev1.Pod
	oldPod, okOld := oldObj.(*corev1.Pod)
	newPod, okNew := newObj.(*corev1.Pod)
	if !okOld || !okNew {
		return false, fmt.Errorf("object is not a Pod")
	}

	// Check if pod has pipeline/runid label
	if newPod.ObjectMeta.Labels != nil {
		if _, exists := newPod.ObjectMeta.Labels[constants.RunPodIdentificaionLabel]; exists {
			return true, nil
		}
	}

	// Check for readiness transition
	if (podPhaseChange(oldPod, newPod) || podReadyCondition(oldPod, newPod)) && oldPod.DeletionTimestamp == nil && newPod.DeletionTimestamp == nil {
		log.Info().Msgf("Pod %s state has transitioned", oldPod.Name)
		return true, nil
	}

	// No relevant change
	return false, nil
}

// isPodReady returns true if the PodReady condition is true in the status.
func isPodReady(status corev1.PodStatus) bool {
	for _, cond := range status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func podPhaseChange(oldPod, newPod *corev1.Pod) bool {
	if oldPod.Status.Phase != newPod.Status.Phase {
		log.Info().Msgf("Pod %s phase changed from %s to %s", oldPod.Name, oldPod.Status.Phase, newPod.Status.Phase)
		return true
	}
	return false
}

// podReadyCondition returns true when PodReady status differs between old and new pods.
func podReadyCondition(oldPod, newPod *corev1.Pod) bool {
	oldReady := isPodReady(oldPod.Status)
	newReady := isPodReady(newPod.Status)
	log.Info().Msgf("PodName: %s, oldReady: %v, newReady: %v", oldPod.Name, oldReady, newReady)
	return oldReady != newReady
}
