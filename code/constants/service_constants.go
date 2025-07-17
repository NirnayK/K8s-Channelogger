// Package constants defines the Celery task names for various admission channelog events.
package constants

const (
	// DummyTask is placeholder for events that do not need to be pushed to Celery.
	DummyTask = "dummy_hook"

	// NodeAddTask is the Celery task name for Node addition events.
	NodeAddTask = "node_add_hook"

	// NodeDeleteTask is the Celery task name for Node deletion events.
	NodeDeleteTask = "node_delete_hook"

	// KedaTask is the Celery task name for KEDA-related admission events.
	KedaTask = "keda_hook"

	// PodStatusTask is the Celery task name for Pod status change events.
	PodStatusTask = "pod_status_hook"

	// PodNodeBindingTask is the Celery task name for Pod node-binding events.
	PodNodeBindingTask = "pod_node_binding_hook"

	// PodDeletionTask is the Celery task name for Pod deletion events.
	PodDeletionTask = "pod_delete_hook"

	// WorkflowTask is the Celery task name for workflow phase change events.
	WorkflowTask = "workflow_event_hook"

	// WorkflowDeleteTask is the Celery task name for workflow deletion events.
	WorkflowDeleteTask = "workflow_delete_event_hook"

	// InferenceHpaCreateTask is the Celery task name for HorizontalPodAutoscaler creation events.
	InferenceHpaCreateTask = "hpa_create_hook"

	// InferenceHpaUpdateTask is the Celery task name for HorizontalPodAutoscaler update events.
	InferenceHpaUpdateTask = "hpa_update_hook"

	// InferenceHpaDeleteTask is the Celery task name for HorizontalPodAutoscaler deletion events.
	InferenceHpaDeleteTask = "hpa_delete_hook"

	// Run Identification Key
	RunPodIdentificaionLabel = "pipeline/runid"
)

// OnlyOldObjectEvents defines tasks that should use the OLDobject instead of the new object.
// This is critical for deletion events where:
// - The new object is typically empty or null during deletion
// - The old object contains the full state before deletion
// - Downstream consumers need the complete object data for cleanup/audit purposes
var OnlyOldObjectEvents = map[string]bool{
	PodDeletionTask:        true, // Need pod details for billing/cleanup
	WorkflowDeleteTask:     true, // Need workflow state for audit/history
	InferenceHpaDeleteTask: true, // Need HPA configuration for resource tracking
}
