// Package helpers provides utility functions for admission channelog handlers,
// including object decoding, panic recovery, and request parsing.
package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	"channelog/constants"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// decoder deserializes raw Kubernetes objects using the client-go scheme.
	decoder = serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDeserializer()
)

// GetUrlType derives a simple mode string from the request path by removing
// the leading “/”. For example, "/pods" becomes "pods".
func GetUrlType(c *fiber.Ctx) string {
	path := c.Path()
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return path
}

// GetObjects decodes the OldObject and Object raw JSON bytes from an AdmissionRequest
// into Kubernetes runtime.Objects. It returns two objects (old, new) and an error
// if any decoding fails.
func GetObjects(request *admissionv1.AdmissionRequest) (runtime.Object, runtime.Object, error) {
	var (
		oldObj runtime.Object
		newObj runtime.Object
		err    error
	)

	// Decode the OldObject if present.
	if raw := request.OldObject.Raw; len(raw) > 0 {
		oldObj, _, err = decoder.Decode(raw, nil, nil)
		if err != nil {
			log.Error().Err(err).Msg("failed to decode old object")
			return nil, nil, fmt.Errorf("failed to decode old object: %w", err)
		}
	}

	// Decode the new Object if present.
	if raw := request.Object.Raw; len(raw) > 0 {
		newObj, _, err = decoder.Decode(raw, nil, nil)
		if err != nil {
			log.Error().Err(err).Msg("failed to decode new object")
			return nil, nil, fmt.Errorf("failed to decode new object: %w", err)
		}
	}

	return oldObj, newObj, nil
}

// RecoverPanic logs any panic that occurs within a deferred context,
// tagging it with the provided context string.
//
// Panic recovery is critical for channelog reliability because:
// - Panics in channelog handlers can cause the entire channelog to crash
// - A crashed channelog blocks all Kubernetes operations that require validation
// - Recovery ensures the channelog continues processing other requests
// - Comprehensive logging helps debug issues without affecting service availability
func RecoverPanic(context string) {
	if r := recover(); r != nil {
		log.Error().
			Interface("panic", r).
			Str("context", context).
			Stack().
			Msg("recovered panic")
	}
}

// PanicCatcher returns a deferred function that will catch and log any panic.
//
// Usage example: defer PanicCatcher("MyFuncName")()
//
// Why panic recovery is better than failing:
// - Prevents a single malformed request from crashing the entire channelog service
// - Maintains channelog availability for other Kubernetes operations
// - Allows graceful degradation while logging errors for debugging
// - Essential for admission webhooks that control cluster operations
func PanicCatcher(context string) func() {
	return func() {
		RecoverPanic(context)
	}
}

// GetOperation parses an AdmissionReview from the HTTP request body
// and returns the contained AdmissionRequest.Operation. It logs and returns
// an error if the body cannot be unmarshaled.
func GetOperation(c *fiber.Ctx) (admissionv1.Operation, error) {
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(c.Body(), &review); err != nil {
		log.Error().Err(err).Msg("could not unmarshal AdmissionReview request")
		return "", fmt.Errorf("could not unmarshal request: %w", err)
	}
	return review.Request.Operation, nil
}

// RemoveAttributes deletes any keys in constants.RemoveAttrs, including nested ones via dot-paths.
func RemoveAttributes(obj map[string]interface{}) {
	for _, path := range constants.RemoveAttrs {
		parts := strings.Split(path, ".")
		removeAtPath(obj, parts)
	}
}

// removeAtPath deletes the key at parts from the given map.
// If parts has length >1, it descends into nested maps.
func removeAtPath(m map[string]interface{}, parts []string) {
	if len(parts) == 0 {
		return
	}
	key := parts[0]
	if len(parts) == 1 {
		// base case: delete this key
		delete(m, key)
		return
	}
	// recursive case: descend
	next, ok := m[key].(map[string]interface{})
	if !ok {
		return
	}
	removeAtPath(next, parts[1:])
	// delete empty maps
	if len(next) == 0 {
		delete(m, key)
	}
}
