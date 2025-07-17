// Package helpers provides utility functions for admission channelog handlers,
// including object decoding, panic recovery, and request parsing.
package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	admissionv1 "k8s.io/api/admission/v1"
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

// ObjectDiff compares two objects and returns a git-style diff string.
// It takes two map[string]interface{} objects representing the old and new versions,
// converts them to YAML files in a temporary directory, and uses git diff to generate
// a proper git-style diff output.
func ObjectDiff(oldObj, newObj map[string]interface{}) (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "object-diff-*")
	if err != nil {
		log.Error().Err(err).Msg("failed to create temp directory")
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Convert objects to YAML and write to temp files
	oldFile := filepath.Join(tempDir, "old.yaml")
	newFile := filepath.Join(tempDir, "new.yaml")

	if err := writeObjectAsYAML(oldObj, oldFile); err != nil {
		log.Error().Err(err).Msg("failed to write old object as YAML")
		return "", fmt.Errorf("failed to write old object as YAML: %w", err)
	}

	if err := writeObjectAsYAML(newObj, newFile); err != nil {
		log.Error().Err(err).Msg("failed to write new object as YAML")
		return "", fmt.Errorf("failed to write new object as YAML: %w", err)
	}

	// Run git diff --no-index to compare the files
	cmd := exec.Command("git", "diff", "--no-index", "--no-prefix", oldFile, newFile)
	output, err := cmd.Output()

	// git diff returns exit code 1 when files differ, which is expected
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			// This is expected when files differ
		} else {
			log.Error().Err(err).Msg("git diff command failed")
			return "", fmt.Errorf("git diff command failed: %w", err)
		}
	}

	diffOutput := string(output)
	if diffOutput == "" {
		return "No differences found", nil
	}

	// Clean up the diff output to make it more readable
	return cleanDiffOutput(diffOutput), nil
}

// writeObjectAsYAML converts a map[string]interface{} to YAML and writes it to a file.
func writeObjectAsYAML(obj map[string]interface{}, filename string) error {
	yamlData, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object to YAML: %w", err)
	}

	return os.WriteFile(filename, yamlData, 0644)
}

// cleanDiffOutput processes the git diff output to make it more readable
// by removing file paths and keeping only the meaningful diff content.
func cleanDiffOutput(diffOutput string) string {
	lines := strings.Split(diffOutput, "\n")
	var cleanedLines []string

	for _, line := range lines {
		// Skip file header lines that show temp file paths
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			(strings.HasPrefix(line, "---") && strings.Contains(line, "/tmp/")) ||
			(strings.HasPrefix(line, "+++") && strings.Contains(line, "/tmp/")) {
			continue
		}

		// Replace temp file references with meaningful names
		if strings.HasPrefix(line, "--- ") {
			line = "--- old"
		} else if strings.HasPrefix(line, "+++ ") {
			line = "+++ new"
		}

		cleanedLines = append(cleanedLines, line)
	}

	return strings.Join(cleanedLines, "\n")
}
