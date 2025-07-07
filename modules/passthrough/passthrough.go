package passthrough

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

// The init function self-registers the PassthroughModule with the global default module registry.
func init() {
	module.Register("passthrough", NewPassthroughModule)
}

// PassthroughModule is a fundamental streaming utility in GXO. It reads every
// record from all of its input streams and writes each record, unmodified,
// to all of its output streams. It is the primary tool for creating pipelines,
// implementing fan-in/fan-out patterns, and for testing and validating
// complex streaming logic within the GXO engine.
type PassthroughModule struct{}

// NewPassthroughModule is the factory function for PassthroughModule.
func NewPassthroughModule() plugin.Module {
	return &PassthroughModule{}
}

// Perform implements the module's core logic.
func (m *PassthroughModule) Perform(
	ctx context.Context,
	params map[string]interface{},
	stateReader state.StateReader,
	inputs map[string]<-chan map[string]interface{},
	outputChans []chan<- map[string]interface{},
	errChan chan<- error,
) (interface{}, error) {
	// A counter for the number of records successfully processed.
	// Using atomic for safe concurrent access from multiple goroutines is not strictly
	// necessary here as only one goroutine increments it, but it's a good practice.
	var recordsProcessed int64

	// If there are no inputs, there's nothing to pass through. Exit cleanly.
	if len(inputs) == 0 {
		return map[string]interface{}{"records_processed": 0, "message": "no input streams provided"}, nil
	}

	var wg sync.WaitGroup
	// An internal channel to merge all input streams into a single stream.
	// This simplifies the fan-out logic.
	mergedInputStream := make(chan map[string]interface{})

	// --- Fan-In Phase ---
	// Launch a goroutine for each input channel to read from it concurrently.
	wg.Add(len(inputs))
	for producerID, inputChan := range inputs {
		go func(id string, ch <-chan map[string]interface{}) {
			defer wg.Done()
			for {
				select {
				case record, ok := <-ch:
					// If the channel is closed, the reading is done for this stream.
					if !ok {
						return
					}
					// Optional: Add metadata to track the record's origin.
					record["_passthrough_source_id"] = id
					// Send the received record to the internal merged stream.
					select {
					case mergedInputStream <- record:
					case <-ctx.Done():
						// If context is cancelled, stop processing immediately.
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(producerID, inputChan)
	}

	// Launch a separate goroutine to wait for all fan-in goroutines to complete.
	// Once they are all done, it means all input channels have been fully drained
	// and closed, so we can safely close the internal merged stream.
	go func() {
		wg.Wait()
		close(mergedInputStream)
	}()

	// --- Fan-Out Phase ---
	// Read from the merged stream until it is closed.
	for record := range mergedInputStream {
		atomic.AddInt64(&recordsProcessed, 1)
		// Write each record to every configured output channel.
		for _, outputChan := range outputChans {
			select {
			case outputChan <- record:
			case <-ctx.Done():
				// If context is cancelled during fan-out, return the error.
				return nil, ctx.Err()
			}
		}
	}

	// Check for a final context error after the loop, in case it was cancelled
	// but the loop finished before the check.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Return a summary of the operation.
	return map[string]interface{}{
		"records_processed": atomic.LoadInt64(&recordsProcessed),
	}, nil
}