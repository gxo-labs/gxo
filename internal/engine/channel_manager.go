package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/gxo-labs/gxo/internal/config"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
)

// ChannelManager is responsible for the creation and lifecycle management of the
// data channels used for streaming between tasks. It also orchestrates the
// complex synchronization required to ensure producers do not terminate before
// their consumers have finished processing the stream.
type ChannelManager struct {
	defaultPolicy config.ChannelPolicy

	// producerChannels maps a producer's task ID to a slice of all its output channels.
	// This is used for fanning out data to multiple consumers.
	producerChannels map[string][]*managedChannel

	// consumerProducerToChannel maps a consumer's task ID to a map of its producers'
	// task IDs to the specific input channel for that producer. This provides the
	// deterministic, named input streams for consumer modules.
	consumerProducerToChannel map[string]map[string]*managedChannel

	// producerWaitGroups holds a sync.WaitGroup for each producer task. Consumers
	// increment this WaitGroup, and the producer's TaskRunner waits on it after
	// sending all data. This is the core mechanism that prevents a producer from
	// being marked "Completed" before its data is fully consumed.
	producerWaitGroups map[string]*sync.WaitGroup

	// consumerToProducerWGs maps a consumer's task ID to the WaitGroups of all
	// its producers. The consumer's TaskRunner uses this to signal "Done" to each
	// producer once it has finished processing, regardless of success or failure.
	consumerToProducerWGs map[string][]*sync.WaitGroup

	mu sync.RWMutex
}

// managedChannel wraps a standard Go channel with its associated policy,
// enabling behaviors like overflow strategies.
type managedChannel struct {
	channel chan map[string]interface{}
	policy  config.ChannelPolicy
	mu      sync.Mutex // Protects write operations, especially for drop_oldest logic.
}

// NewChannelManager creates a new, initialized ChannelManager.
func NewChannelManager(defaultPolicy *config.ChannelPolicy) *ChannelManager {
	// Establish a safe, effective default policy.
	effectiveDefaultPolicy := config.ChannelPolicy{
		BufferSize:       new(int),
		OverflowStrategy: config.OverflowBlock,
	}
	*effectiveDefaultPolicy.BufferSize = 100 // Default buffer size.

	if defaultPolicy != nil {
		if defaultPolicy.BufferSize != nil {
			effectiveDefaultPolicy.BufferSize = defaultPolicy.BufferSize
		}
		if defaultPolicy.OverflowStrategy != "" {
			effectiveDefaultPolicy.OverflowStrategy = defaultPolicy.OverflowStrategy
		}
	}

	// Sanitize final policy values.
	if *effectiveDefaultPolicy.BufferSize < 0 {
		*effectiveDefaultPolicy.BufferSize = 0
	}
	switch effectiveDefaultPolicy.OverflowStrategy {
	case config.OverflowBlock, config.OverflowDropNew, config.OverflowDropOldest, config.OverflowError:
		// Valid strategy, do nothing.
	default:
		// Fallback to the safest strategy if an invalid one was provided.
		effectiveDefaultPolicy.OverflowStrategy = config.OverflowBlock
	}

	return &ChannelManager{
		defaultPolicy:             effectiveDefaultPolicy,
		producerChannels:          make(map[string][]*managedChannel),
		consumerProducerToChannel: make(map[string]map[string]*managedChannel),
		producerWaitGroups:        make(map[string]*sync.WaitGroup),
		consumerToProducerWGs:     make(map[string][]*sync.WaitGroup),
	}
}

// CreateChannels inspects the DAG and builds the entire streaming topology,
// including data channels and synchronization WaitGroups. This must be called
// after the DAG is built and before execution begins.
func (cm *ChannelManager) CreateChannels(dag *DAG) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Reset all internal maps to ensure a clean state for the new playbook run.
	cm.producerChannels = make(map[string][]*managedChannel)
	cm.consumerProducerToChannel = make(map[string]map[string]*managedChannel)
	cm.producerWaitGroups = make(map[string]*sync.WaitGroup)
	cm.consumerToProducerWGs = make(map[string][]*sync.WaitGroup)

	for consumerID, consumerNode := range dag.Nodes {
		if len(consumerNode.Task.StreamInputs) == 0 {
			continue
		}

		// Initialize maps for this specific consumer.
		cm.consumerProducerToChannel[consumerID] = make(map[string]*managedChannel)
		cm.consumerToProducerWGs[consumerID] = make([]*sync.WaitGroup, 0, len(consumerNode.Task.StreamInputs))

		for _, producerName := range consumerNode.Task.StreamInputs {
			producerNode, producerFound := findNodeByName(dag, producerName)
			if !producerFound {
				return fmt.Errorf("consistency error: producer task '%s' for consumer '%s' not found in DAG", producerName, consumerNode.Task.Name)
			}
			producerID := producerNode.ID

			// Get or create the WaitGroup for the producer.
			producerWg, exists := cm.producerWaitGroups[producerID]
			if !exists {
				producerWg = &sync.WaitGroup{}
				cm.producerWaitGroups[producerID] = producerWg
			}

			// CRITICAL: The consumer registers its dependency on the producer's stream.
			// The producer's TaskRunner will wait for this signal before full completion.
			producerWg.Add(1)
			cm.consumerToProducerWGs[consumerID] = append(cm.consumerToProducerWGs[consumerID], producerWg)

			// Create the physical channel with the effective policy.
			policy := cm.defaultPolicy // Start with the default. (Future: could be overridden by task config).
			bufferSize := *policy.BufferSize
			if bufferSize < 0 {
				bufferSize = 0
			}

			managedChan := &managedChannel{
				channel: make(chan map[string]interface{}, bufferSize),
				policy:  policy,
			}

			// Wire the channel into the topology maps.
			cm.producerChannels[producerID] = append(cm.producerChannels[producerID], managedChan)
			cm.consumerProducerToChannel[consumerID][producerID] = managedChan
		}
	}

	return nil
}

// GetProducerWaitGroup retrieves the WaitGroup a producer task must wait on.
func (cm *ChannelManager) GetProducerWaitGroup(taskID string) (*sync.WaitGroup, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	wg, exists := cm.producerWaitGroups[taskID]
	return wg, exists
}

// GetConsumerProducerWaitGroups retrieves all WaitGroups a consumer task must signal upon completion.
func (cm *ChannelManager) GetConsumerProducerWaitGroups(taskID string) ([]*sync.WaitGroup, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	wgs, exists := cm.consumerToProducerWGs[taskID]
	return wgs, exists
}

// GetInputChannelMap provides the map of producer IDs to their corresponding input channels for a consumer.
func (cm *ChannelManager) GetInputChannelMap(taskID string) (map[string]<-chan map[string]interface{}, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	producerMap, exists := cm.consumerProducerToChannel[taskID]
	if !exists {
		return nil, false
	}

	// Create a new map with the correct read-only channel type for the module interface.
	resultMap := make(map[string]<-chan map[string]interface{}, len(producerMap))
	for producerID, mc := range producerMap {
		resultMap[producerID] = mc.channel
	}
	return resultMap, true
}

// GetOutputManagedChannels retrieves the raw managed output channels for a producer task.
func (cm *ChannelManager) GetOutputManagedChannels(taskID string) ([]*managedChannel, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	channels, exists := cm.producerChannels[taskID]
	return channels, exists
}

// Write sends data to a managed channel, respecting its overflow policy.
func (mc *managedChannel) Write(ctx context.Context, data map[string]interface{}) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	switch mc.policy.OverflowStrategy {
	case config.OverflowDropNew:
		// Non-blocking send; drop if full.
		select {
		case mc.channel <- data:
			return nil
		default:
			return gxoerrors.NewPolicyViolationError("ChannelPolicy", "channel full, dropped new message (OverflowDropNew)", nil)
		}

	case config.OverflowDropOldest:
		// Attempt to send; if it would block, drop the oldest item to make space.
		select {
		case mc.channel <- data:
			return nil
		default:
			// Buffer is full. Try to make space by dropping the oldest item.
			select {
			case <-mc.channel:
				// An old item was successfully dropped. Now try to send the new item again.
				select {
				case mc.channel <- data:
					// Succeeded after dropping. Inform the caller via a specific error.
					return gxoerrors.NewPolicyViolationError("ChannelPolicy", "channel full, dropped oldest message (OverflowDropOldest)", nil)
				default:
					// This case is unlikely but indicates the channel was filled again
					// between the drop and the send by another goroutine (not possible with current mutex).
					return fmt.Errorf("internal channel error: failed to send immediately after dropping oldest")
				}
			default:
				// The channel was full, but we couldn't even drop an old item (it became empty).
				// This indicates a race condition; we try one last time to send.
				select {
				case mc.channel <- data:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				default:
					return gxoerrors.NewPolicyViolationError("ChannelPolicy", "channel full, failed to drop oldest and send new (OverflowDropOldest)", nil)
				}
			}
		}

	case config.OverflowError:
		// Non-blocking send; return an error if full.
		select {
		case mc.channel <- data:
			return nil
		default:
			return gxoerrors.NewPolicyViolationError("ChannelPolicy", "channel full, overflow strategy is 'error' (OverflowError)", nil)
		}

	default: // config.OverflowBlock
		// Standard blocking send, but respects context cancellation.
		select {
		case mc.channel <- data:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CloseOutputChannels finds all output channels for a given producer task and closes them.
// This is the signal for "End of Stream" to all downstream consumers.
func (cm *ChannelManager) CloseOutputChannels(taskID string) {
	cm.mu.RLock()
	channels, exists := cm.producerChannels[taskID]
	cm.mu.RUnlock()

	if !exists {
		return
	}

	for _, mc := range channels {
		mc.mu.Lock()
		// It's safe to close a channel multiple times if guarded, but we
		// can also check if it's already closed. Here, the logic ensures
		// Close is only called once per channel by the TaskRunner.
		close(mc.channel)
		mc.mu.Unlock()
	}
}

// findNodeByName is a utility to find a DAG node by its user-defined task name.
func findNodeByName(dag *DAG, name string) (*Node, bool) {
	if name == "" {
		return nil, false
	}
	// This linear scan is acceptable as DAGs are typically not excessively large,
	// and this is only done during the initial setup phase.
	for _, node := range dag.Nodes {
		if node.Task.Name == name {
			return node, true
		}
	}
	return nil, false
}