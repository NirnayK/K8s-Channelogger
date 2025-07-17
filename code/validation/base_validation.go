// Package validation provides functions to validate Kubernetes admission requests for various tasks.
package validation

import (
	admissionv1 "k8s.io/api/admission/v1"
)

// ValidTask determines whether the provided AdmissionRequest
// should be processed as a valid task.
//
// It returns a boolean indicating validity and an error if validation fails.
//
// NOTE: This always returns true with no error.
func ValidTask(request *admissionv1.AdmissionRequest) (bool, error) {
	return true, nil
}
