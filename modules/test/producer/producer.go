package producer

import (
	"context"
	"fmt"
	"sync"

	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/paramutil"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

func init() {
	module.Register("test:producer", NewTestProducerModule)
}

// TestProducerModule is a module specifically for testing streaming pipelines.
// It takes a list of records from its 'records' parameter and writes them to
// its output channels. It correctly blocks until all records are sent,
// ensuring proper synchronization with downstream consumers.
type TestProducerModule struct{}

// NewTestProducerModule is the factory for TestProducerModule.
func NewTestProducerModule() plugin.Module {
	return &TestProducerModule{}
}

// Perform implements the module's logic.
func (m *TestProducerModule) Perform(
	ctx context.Context,
	params map[string]interface{},
	stateReader state.StateReader,
	inputs map[string]<-chan map[string]interface{},
	outputChans []chan<- map[string]interface{},
	errChan chan<- error,
) (interface{}, error) {
	// This is a producer, so it drains any unexpected inputs.
	var drainWg sync.WaitGroup
	drainWg.Add(len(inputs))
	for _, inputChan := range inputs {
		go func(ch <-chan map[string]interface{}) {
			defer drainWg.Done()
			for range ch {
			}
		}(inputChan)
	}

	rawRecords, err := paramutil.GetRequiredSlice(params, "records")
	if err != nil {
		drainWg.Wait()
		return nil, err
	}

	var recordsSent int64

	// Perform the send operation directly. This is a blocking call.
	for i, item := range rawRecords {
		record, ok := item.(map[string]interface{})
		if !ok {
			nonFatalErr := gxoerrors.NewRecordProcessingError(
				"test:producer",
				fmt.Sprintf("item at index %d", i),
				fmt.Errorf("item is not a map[string]interface{}, but a %T", item),
			)
			select {
			case errChan <- nonFatalErr:
			case <-ctx.Done():
				drainWg.Wait()
				return nil, ctx.Err()
			}
			continue
		}

		// Write the record to all output channels (fan-out).
		for _, out := range outputChans {
			select {
			case out <- record:
			case <-ctx.Done():
				drainWg.Wait()
				return nil, ctx.Err()
			}
		}
		recordsSent++
	}

	drainWg.Wait()
	return map[string]interface{}{"records_sent": recordsSent}, nil
}