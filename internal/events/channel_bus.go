package events

import (
	// Import the public events interface definition and types.
	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	// Import the public logger interface definition.
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
)

// ChannelEventBus implements the public events.Bus interface using a buffered Go channel.
// It provides a simple, in-process, decoupled event distribution mechanism suitable for
// scenarios where listeners run within the same process as the engine.
// Its primary characteristic is non-blocking emission of events.
type ChannelEventBus struct {
	// channel is the buffered Go channel that holds events pending delivery.
	// It uses the public events.Event type.
	channel chan events.Event
	// log is used for internal operational messages, such as warning about dropped events
	// when the channel buffer is full. It uses the public logger interface.
	log gxolog.Logger // Uses the public interface type
}

// NewChannelEventBus creates a new ChannelEventBus with the specified buffer size.
// If bufferSize is non-positive, a default buffer size (e.g., 100) is used.
// A non-nil logger instance (implementing gxolog.Logger) is required.
// Panics if the provided logger is nil.
func NewChannelEventBus(bufferSize int, log gxolog.Logger) *ChannelEventBus { // Accepts public interface type
	// Set a reasonable default buffer size if an invalid one is provided.
	const defaultBufferSize = 100
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	// Enforce that a valid logger must be provided.
	if log == nil {
		// Cannot operate without a logger. Fail fast during setup.
		panic("ChannelEventBus requires a non-nil logger")
	}

	// Initialize the struct with the buffered channel and logger.
	// Use the logger's With method (which returns gxolog.Logger) to add context.
	bus := &ChannelEventBus{
		channel: make(chan events.Event, bufferSize),
		log:     log.With("component", "ChannelEventBus"), // Add component context to logs.
	}
	// Log initialization details for debugging/visibility.
	bus.log.Debugf("ChannelEventBus initialized with buffer size %d", bufferSize)
	return bus
}

// Emit sends an event onto the internal buffered channel.
// To prevent blocking the caller (the engine core), this operation is non-blocking.
// If the channel buffer is full at the time of the call, the event is dropped,
// and a warning is logged using the configured logger.
// This implements the events.Bus interface method.
func (c *ChannelEventBus) Emit(event events.Event) {
	// Attempt a non-blocking send to the channel using a select statement.
	select {
	case c.channel <- event:
		// Event successfully sent to the buffer.
		c.log.Debugf("Emitted event type '%s'", event.Type) // Using public logger interface method
	default:
		// The channel buffer is full; the send would block.
		// Log a warning indicating that the event is being dropped.
		c.log.Warnf("Event channel buffer full, dropping event type '%s'", event.Type) // Using public logger interface method
		// Consider incrementing a metric here (e.g., 'gxo_events_dropped_total')
		// if detailed monitoring of event drops is required.
	}
}

// GetChannel returns the underlying event channel for consumers.
// This method is specific to the ChannelEventBus implementation and is NOT part
// of the public events.Bus interface. It allows external components within the
// same process (like dedicated event listeners or exporters) to directly consume
// events from the channel. The returned channel is read-only (`<-chan`).
func (c *ChannelEventBus) GetChannel() <-chan events.Event {
	return c.channel
}

// Close closes the underlying event channel.
// This signals to consumers reading from GetChannel() that no more events will be sent.
// This method is specific to the ChannelEventBus implementation.
func (c *ChannelEventBus) Close() {
	c.log.Debugf("Closing ChannelEventBus channel.") // Using public logger interface method
	close(c.channel)
}

// Ensure ChannelEventBus implements the public events.Bus interface at compile time.
var _ events.Bus = (*ChannelEventBus)(nil)