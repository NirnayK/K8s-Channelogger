package filters

import admissionv1 "k8s.io/api/admission/v1"

// ValidateValidRequest determines whether the admission request should be
// skipped for changelog processing. It returns true when the request should
// not be processed further, for example when the object kind is Pod.
func ValidateValidRequest(review admissionv1.AdmissionReview) bool {
	if review.Request.Kind.Kind == "Pod" {
		// Skip Pod objects entirely.
		return true
	}

	return false
}
