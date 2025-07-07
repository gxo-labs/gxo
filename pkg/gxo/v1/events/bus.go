package events

import "time"

// EventType represents the type of a GXO engine event.
type EventType string

// Standard GXO Event Types
const (
	PlaybookStart        EventType = "PlaybookStart"
	PlaybookEnd          EventType = "PlaybookEnd"
	TaskStatusChanged    EventType = "TaskStatusChanged" // Final status set
	TaskStart            EventType = "TaskStart"         // TaskRunner begins processing
	TaskEnd              EventType = "TaskEnd"           // TaskRunner finishes processing (after loops/retries)
	ModuleExecutionStart EventType = "ModuleExecutionStart" // Before Module.Perform call
	ModuleExecutionEnd   EventType = "ModuleExecutionEnd"   // After Module.Perform returns
	RecordErrorOccurred  EventType = "RecordErrorOccurred"  // Non-fatal error from module errChan
	FatalErrorOccurred   EventType = "FatalErrorOccurred"   // Fatal error from Perform or engine logic
	SecretAccessed       EventType = "SecretAccessed"       // A secret value was accessed via template func
)

// Event represents a significant occurrence within the GXO engine.
type Event struct {
	// Type categorizes the event.
	Type EventType `json:"type"`
	// Timestamp marks when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// PlaybookName identifies the playbook context, if applicable.
	PlaybookName string `json:"playbook_name,omitempty"`
	// TaskName identifies the task context (user-defined name), if applicable.
	TaskName string `json:"task_name,omitempty"`
	// TaskID identifies the task context using its internal ID, if applicable.
	TaskID string `json:"task_id,omitempty"`
	// Payload contains event-specific data. Sensitive information (like secret values)
	// MUST NOT be included in the payload. Secret keys accessed might be included
	// if necessary for auditing (e.g., in SecretAccessed event).
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// Bus defines the interface for publishing events within the GXO engine.
// Implementations could include logging, sending to message queues, etc.
type Bus interface {
	// Emit publishes an event to the bus.
	// Implementations should be non-blocking or handle blocking carefully
	// to avoid slowing down the engine core.
	// Sensitive information (like secret values) MUST NOT be included in the event payload.
	Emit(event Event)
}
