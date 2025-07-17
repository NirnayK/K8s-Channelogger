// Package task provides functionality to wrap Kubernetes AdmissionRequests
// into Celery-compatible messages and publish them to RabbitMQ.
package task

import (
	"encoding/json"
	"time"

	"channelog/config"
	"channelog/constants"
	"channelog/helpers"
	"channelog/rabbit"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// defaultExchange is the RabbitMQ exchange to publish to (empty = default vhost).
	defaultExchange = ""
	// defaultMandatory controls whether messages must be routed or returned.
	defaultMandatory = false
	// defaultImmediate controls whether to return if there is no live consumer.
	defaultImmediate = false
	// defaultContentType is the MIME type for the message payload.
	defaultContentType = "application/json"
	// defaultContentEncoding is the character encoding for the payload.
	defaultContentEncoding = "utf-8"
	// defaultDeliveryMode instructs RabbitMQ to persist messages to disk.
	defaultDeliveryMode = amqp.Persistent
	// defaultPriority is the message priority (0 = normal).
	defaultPriority = 0

	// maxPublishAttempts is the total number of times we'll try to publish before giving up.
	maxPublishAttempts = 2
)

// celeryMessage is the JSON shape Celery expects when using the "json" serializer.
type celeryMessage struct {
	// ID is a unique identifier for this task invocation.
	ID string `json:"id"`
	// Task is the name of the Celery task to execute.
	Task string `json:"task"`
	// Args are positional arguments (unused in our implementation).
	Args []interface{} `json:"args"`
	// Kwargs are keyword arguments containing metadata and the raw object.
	Kwargs map[string]interface{} `json:"kwargs"`
	// Retries is the number of times this task has been retried (starts at 0).
	Retries int `json:"retries"`
	// ETA can be used to schedule execution in the future (nil = immediate).
	ETA interface{} `json:"eta"`
}

// buildGenericPayload constructs a celeryMessage for generic Kubernetes objects.
//
// It unmarshals req.Object.Raw into a map[string]interface{} so that the
// JSON payload embeds the full Kubernetes object, along with metadata fields.
func buildGenericPayload(req *admissionv1.AdmissionRequest, eventObject runtime.RawExtension, taskName, location string) celeryMessage {
	// Attempt to parse the raw JSON into a generic map.
	var rawObject map[string]interface{}
	if err := json.Unmarshal(eventObject.Raw, &rawObject); err != nil {
		// If parsing fails, log the error and proceed (raw JSON may still be included).
		log.Error().Err(err).Msg("could not parse raw JSON into map")
	}

	helpers.RemoveAttributes(rawObject)

	// Build the kwargs payload with standard metadata.
	inner := map[string]interface{}{
		"uid":        string(req.UID),
		"taskName":   taskName,
		"namespace":  req.Namespace,
		"name":       req.Name,
		"resource":   req.Resource,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"raw_object": rawObject,
		"location":   location,
	}

	return celeryMessage{
		ID:      uuid.NewString(),
		Task:    taskName,
		Args:    []interface{}{}, // always non-nil for JSON serialization
		Kwargs:  inner,
		Retries: 0,
		ETA:     nil,
	}
}

// buildNodePayload constructs a celeryMessage for Node requests.
//
// It uses the Node name as the task name and includes metadata fields.
// The raw object is not included in the payload, as it is not needed for Node tasks.
func buildNodePayload(req *admissionv1.AdmissionRequest, taskName, location string) celeryMessage {

	// Build the kwargs payload with standard metadata.
	inner := map[string]interface{}{
		"uid":       string(req.UID),
		"taskName":  taskName,
		"namespace": req.Namespace,
		"name":      req.Name,
		"resource":  req.Resource,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"node_name": req.Name,
		"location":  location,
	}

	return celeryMessage{
		ID:      uuid.NewString(),
		Task:    taskName,
		Args:    []interface{}{}, // always non-nil for JSON serialization
		Kwargs:  inner,
		Retries: 0,
		ETA:     nil,
	}
}

// buildPayload constructs a celeryMessage based on the type of Kubernetes object.
//
// Payload structure varies by use case:
//
// - OnlyOldObjectEvents: Use req.OldObject for deletion events where new object is empty
//
// - Node requests: Have special kind of payloads
//
// - Default: Use req.Object with full object data for standard processing
//
// Downstream Celery consumers determine payload format based on task name and
// implement corresponding parsing logic for each payload type.
func buildPayload(req *admissionv1.AdmissionRequest, taskName string, location string) celeryMessage {
	if exists := constants.OnlyOldObjectEvents[taskName]; exists {
		return buildGenericPayload(req, req.OldObject, taskName, location)
	}

	switch req.Kind.Kind {
	case constants.NodeRequestKind:
		return buildNodePayload(req, taskName, location)
	default:
		return buildGenericPayload(req, req.Object, taskName, location)
	}
}

// toPublishing serializes a celeryMessage to JSON and wraps it in amqp.Publishing.
//
// Returns an error if JSON marshaling fails.
func toPublishing(msg celeryMessage) (amqp.Publishing, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return amqp.Publishing{}, err
	}
	return amqp.Publishing{
		ContentType:     defaultContentType,
		ContentEncoding: defaultContentEncoding,
		DeliveryMode:    defaultDeliveryMode,
		Priority:        defaultPriority,
		Body:            body,
	}, nil
}

// PushTask builds and publishes a Celery task to RabbitMQ.
//
//	ar:        the AdmissionReview containing the AdmissionRequest
//	taskName: the Celery task name to enqueue
//	rm:       the RabbitManager used to acquire and release channels
//	cfg:      configuration including QueueName
//
// This function recovers from panics via helpers.PanicCatcher to ensure
// that no panic propagates out of the channelog handler.
func PushTask(ar *admissionv1.AdmissionReview, taskName string, rm *rabbit.RabbitManager, cfg *config.Config) {
	// Ensure any panic is caught and logged with context "PushTask".
	defer helpers.PanicCatcher("PushTask")()

	req := ar.Request

	// 1) Build the Celery message envelope.
	celeryMsg := buildPayload(req, taskName, cfg.Location)

	// 2) Convert the message into an AMQP Publishing.
	pub, err := toPublishing(celeryMsg)
	if err != nil {
		log.Error().
			Err(err).
			Str("uid", string(req.UID)).
			Msg("failed to marshal celery message")
		return
	}

	// 3) Publish the message to RabbitMQ.
	if err := rm.PublishWithRetry(
		defaultExchange,
		cfg.QueueName,
		pub,
	); err != nil {
		log.Error().
			Err(err).
			Str("uid", string(req.UID)).
			Msg("failed to enqueue task")
		return
	}

	// 4) Log successful enqueue for auditing/debugging.
	objectToPrint := req.Object.Raw
	if exists := constants.OnlyOldObjectEvents[taskName]; exists {
		objectToPrint = req.OldObject.Raw
	}
	log.Info().
		Msgf("enqueued task_id: %s | task: %s | raw_object: %s",
			celeryMsg.ID, taskName, string(objectToPrint))
}
