// modules/generate/from_list/from_list.go
package fromlist

import (
	"context"
	"fmt"
	"sync"

	"github.com/gxo-labs/gxo/internal/logger"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/paramutil"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

func init() {
	module.Register("generate:from_list", NewFromListModule)
}

// FromListModule generates a stream of records from a list.
type FromListModule struct{}

// NewFromListModule is the factory for FromListModule.
func NewFromListModule() plugin.Module {
	return &FromListModule{}
}

// Perform implements the module's logic.
func (m *FromListModule) Perform(
	ctx context.Context,
	params map[string]interface{},
	stateReader state.StateReader,
	inputs map[string]<-chan map[string]interface{},
	outputChans []chan<- map[string]interface{},
	errChan chan<- error,
) (interface{}, error) {
	log := logger.NewDefaultLogger("debug").With("module", "generate:from_list")
	// This is a producer; it drains any unexpected inputs.
	var drainWg sync.WaitGroup
	drainWg.Add(len(inputs))
	for _, inputChan := range inputs {
		go func(ch <-chan map[string]interface{}) {
			defer drainWg.Done()
			for range ch {
			}
		}(inputChan)
	}

	items, err := paramutil.GetRequiredSlice(params, "items")
	if err != nil {
		drainWg.Wait()
		return nil, err
	}
	log.Debugf("Received items to generate (count: %d): %#v", len(items), items)

	var recordsGenerated int64

	// This function now blocks until all records are sent OR the context is cancelled.
	for i, item := range items {
		log.Debugf("Processing item at index %d: %#v", i, item)
		var record map[string]interface{}
		var conversionErr error

		switch v := item.(type) {
		case map[string]interface{}:
			record = v
		case map[interface{}]interface{}:
			record, conversionErr = convertMap(v)
			if conversionErr != nil {
				conversionErr = fmt.Errorf("item at index %d has non-string keys: %w", i, conversionErr)
			}
		default:
			record = map[string]interface{}{"item": v}
			log.Debugf("Converted non-map item (type %T) to record: %#v", item, record)
		}

		if conversionErr != nil {
			log.Warnf("Conversion error for item at index %d: %v", i, conversionErr)
			err := gxoerrors.NewRecordProcessingError(
				"generate:from_list",
				fmt.Sprintf("item at index %d", i),
				conversionErr,
			)
			select {
			case errChan <- err:
			case <-ctx.Done():
				drainWg.Wait()
				return nil, ctx.Err()
			}
			continue
		}
		log.Debugf("Successfully converted item at index %d to record: %#v", i, record)

		// Write the record to all output channels (fan-out).
		for j, out := range outputChans {
			log.Debugf("Sending record to output channel %d.", j)
			select {
			case out <- record:
				log.Debugf("Successfully sent record to output channel %d.", j)
			case <-ctx.Done():
				drainWg.Wait()
				return nil, ctx.Err()
			}
		}
		recordsGenerated++
	}

	// After sending all records, the Perform method should return.
	// The engine will see the successful return and automatically close the
	// output channels, signaling End-of-Stream to downstream consumers.
	log.Debugf("All records sent. Producer task is now finishing.")

	drainWg.Wait() // Final wait before returning.
	return map[string]interface{}{"records_generated": recordsGenerated}, nil
}

// convertMap robustly converts a map[interface{}]interface{} to map[string]interface{}.
func convertMap(mii map[interface{}]interface{}) (map[string]interface{}, error) {
	msi := make(map[string]interface{}, len(mii))
	for k, v := range mii {
		ks, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("key is not a string: %T", k)
		}
		msi[ks] = v
	}
	return msi, nil
}