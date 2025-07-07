package events

import (
	"context"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
	"github.com/prometheus/client_golang/prometheus"
)

// MetricsEventListener subscribes to a GXO event bus and updates Prometheus
// metrics based on the events it receives.
type MetricsEventListener struct {
	bus                  *ChannelEventBus
	log                  gxolog.Logger
	secretsAccessCounter prometheus.Counter
}

// NewMetricsEventListener creates a new listener.
// It requires a ChannelEventBus to subscribe to, and the specific Prometheus
// counter it needs to increment.
func NewMetricsEventListener(bus *ChannelEventBus, secretsCounter prometheus.Counter, log gxolog.Logger) *MetricsEventListener {
	if bus == nil || secretsCounter == nil || log == nil {
		// A nil logger would cause a panic, so we check all dependencies.
		panic("MetricsEventListener requires a non-nil ChannelEventBus, Prometheus Counter, and Logger")
	}
	return &MetricsEventListener{
		bus:                  bus,
		log:                  log.With("component", "MetricsEventListener"),
		secretsAccessCounter: secretsCounter,
	}
}

// Start begins listening for events on the bus in a new goroutine.
// The provided context is used to signal shutdown.
func (l *MetricsEventListener) Start(ctx context.Context) {
	l.log.Debugf("Starting metrics event listener...")
	// The listening loop will run until the bus channel is closed or the context is done.
	for {
		select {
		case event, ok := <-l.bus.GetChannel():
			if !ok {
				// Channel was closed, the listener should shut down.
				l.log.Debugf("Event bus channel closed, stopping listener.")
				return
			}
			// Process the received event.
			l.handleEvent(event)
		case <-ctx.Done():
			// The parent context was cancelled, signaling a shutdown.
			l.log.Debugf("Context cancelled, stopping metrics event listener.")
			return
		}
	}
}

// handleEvent processes a single event, incrementing metrics as needed.
func (l *MetricsEventListener) handleEvent(event events.Event) {
	// Use a switch to handle different event types.
	switch event.Type {
	case events.SecretAccessed:
		// When a SecretAccessed event is received, increment the counter.
		if l.secretsAccessCounter != nil {
			l.secretsAccessCounter.Inc()
			l.log.Debugf("Incremented secrets access counter.")
		}
	// Add cases for other events here if the listener needs to handle more metrics.
	// default:
	//   l.log.Debugf("Metrics listener received unhandled event type: %s", event.Type)
	}
}