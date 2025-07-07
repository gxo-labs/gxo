package events

import "github.com/gxo-labs/gxo/pkg/gxo/v1/events" // Use public events interface and type

// NoOpEventBus is a default implementation of the public events.Bus interface.
// It performs no actions when its Emit method is called. This implementation
// is used as a fallback when no specific event handling mechanism (like logging
// or sending to a message queue) is configured for the engine. It ensures that
// components emitting events do not encounter nil pointer errors even if event
// handling is effectively disabled.
type NoOpEventBus struct{}

// NewNoOpEventBus creates a new instance of the NoOpEventBus.
// It returns a value satisfying the public events.Bus interface.
func NewNoOpEventBus() events.Bus {
	return &NoOpEventBus{}
}

// Emit implements the events.Bus interface method.
// In this No-Operation implementation, the method simply returns without
// processing or forwarding the event in any way.
func (n *NoOpEventBus) Emit(event events.Event) {
	// Intentionally does nothing.
}

// Ensure NoOpEventBus implements the public events.Bus interface at compile time.
// This helps catch interface implementation errors early during development.
var _ events.Bus = (*NoOpEventBus)(nil)