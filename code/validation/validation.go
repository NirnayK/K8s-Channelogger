package validation

import admissionv1 "k8s.io/api/admission/v1"

func ValidateValidRequest(review admissionv1.AdmissionReview) bool {
	// Check if the request is valid
	// Check if the admission review is for a pod - if so, allow it and return early
	if review.Request.Kind.Kind == "Pod" {
		return false
	}
	return true
}
