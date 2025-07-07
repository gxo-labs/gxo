package engine

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/gxo-labs/gxo/internal/config"
	"github.com/gxo-labs/gxo/internal/template"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

// TaskStatus represents the execution status of a task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "Pending"
	StatusReady     TaskStatus = "Ready" // An internal status, not externally visible
	StatusRunning   TaskStatus = "Running"
	StatusCompleted TaskStatus = "Completed"
	StatusFailed    TaskStatus = "Failed"
	StatusSkipped   TaskStatus = "Skipped"
)

// Node represents a single task within the execution graph (DAG).
// It holds the task configuration, its dependencies, and its resolved policies.
type Node struct {
	Task *config.Task
	ID   string

	// Dependency tracking
	StreamDependsOn map[string]*Node
	StateDependsOn  map[string]*Node
	RequiredBy      map[string]*Node

	// Counters for dependency resolution
	StreamDepsRemaining atomic.Int32
	StateDepsRemaining  atomic.Int32

	Status atomic.Value

	// Resolved policies for this specific task
	TaskPolicy  *config.TaskPolicy
	StatePolicy *config.StatePolicy
}

// DAG represents the entire Directed Acyclic Graph for a playbook.
type DAG struct {
	Nodes map[string]*Node
}

// BuildDAG constructs the execution graph from a playbook, resolving all
// dependencies and policies for each task.
func BuildDAG(
	playbook *config.Playbook,
	stateReader state.StateReader,
	renderer template.Renderer,
) (*DAG, []*Node, error) {

	tasks := playbook.Tasks
	if len(tasks) == 0 {
		return &DAG{Nodes: make(map[string]*Node)}, []*Node{}, nil
	}

	dag := &DAG{
		Nodes: make(map[string]*Node, len(tasks)),
	}

	nameToTaskID := make(map[string]string)
	registeredVarsToTaskID := make(map[string]string)

	// Pass 1: Create nodes and resolve policies for each task.
	for i := range tasks {
		task := &tasks[i]
		if task.InternalID == "" {
			return nil, nil, fmt.Errorf("internal error: task at index %d has no InternalID during DAG build", i)
		}

		// --- Policy Resolution ---
		resolvedTaskPolicy := &config.TaskPolicy{SkipOnNoInput: new(bool)}
		*resolvedTaskPolicy.SkipOnNoInput = false
		resolvedStatePolicy := &config.StatePolicy{AccessMode: config.StateAccessDeepCopy}

		if playbook.TaskPolicy != nil && playbook.TaskPolicy.SkipOnNoInput != nil {
			resolvedTaskPolicy.SkipOnNoInput = playbook.TaskPolicy.SkipOnNoInput
		}
		if playbook.StatePolicy != nil && playbook.StatePolicy.AccessMode != "" {
			resolvedStatePolicy.AccessMode = playbook.StatePolicy.AccessMode
		}

		if task.Policy != nil && task.Policy.SkipOnNoInput != nil {
			resolvedTaskPolicy.SkipOnNoInput = task.Policy.SkipOnNoInput
		}
		if task.StatePolicy != nil && task.StatePolicy.AccessMode != "" {
			resolvedStatePolicy.AccessMode = task.StatePolicy.AccessMode
		}

		node := &Node{
			Task:            task,
			ID:              task.InternalID,
			StreamDependsOn: make(map[string]*Node),
			StateDependsOn:  make(map[string]*Node),
			RequiredBy:      make(map[string]*Node),
			TaskPolicy:      resolvedTaskPolicy,
			StatePolicy:     resolvedStatePolicy,
		}
		node.Status.Store(StatusPending)
		dag.Nodes[task.InternalID] = node

		if task.Name != "" {
			nameToTaskID[task.Name] = task.InternalID
		}
		if task.Register != "" {
			registeredVarsToTaskID[task.Register] = task.InternalID
		}
	}

	// Pass 2: Add dependency edges based on stream inputs and template variables.
	for _, node := range dag.Nodes {
		for _, producerName := range node.Task.StreamInputs {
			producerID, exists := nameToTaskID[producerName]
			if !exists {
				return nil, nil, gxoerrors.NewConfigError(fmt.Sprintf("task '%s' has stream_inputs referencing undefined task '%s'", node.Task.Name, producerName), nil)
			}
			addStreamEdge(dag, producerID, node.ID)
		}

		templatesToScan := collectTemplatesToScan(node.Task)
		for _, tmplStr := range templatesToScan {
			if tmplStr == "" {
				continue
			}
			vars, err := renderer.ExtractVariables(tmplStr)
			if err != nil {
				continue
			}

			for _, fullVarPath := range vars {
				var producerID string
				var exists bool
				if strings.HasPrefix(fullVarPath, template.GxoStateKeyPrefix+".tasks.") {
					parts := strings.Split(fullVarPath, ".")
					if len(parts) == 4 {
						producerName := parts[2]
						producerID, exists = nameToTaskID[producerName]
					}
				} else {
					producerID, exists = registeredVarsToTaskID[fullVarPath]
				}

				if exists && producerID != "" && producerID != node.ID {
					addStateEdge(dag, producerID, node.ID)
				}
			}
		}
	}

	// Pass 3: Initialize dependency counters and find initial ready nodes.
	initialReadyNodes := make([]*Node, 0)
	for _, node := range dag.Nodes {
		node.StreamDepsRemaining.Store(int32(len(node.StreamDependsOn)))
		node.StateDepsRemaining.Store(int32(len(node.StateDependsOn)))
		if node.StreamDepsRemaining.Load() == 0 && node.StateDepsRemaining.Load() == 0 {
			initialReadyNodes = append(initialReadyNodes, node)
		}
	}

	if err := detectCycle(dag); err != nil {
		return nil, nil, err
	}

	return dag, initialReadyNodes, nil
}

func addStreamEdge(dag *DAG, producerID, consumerID string) {
	producerNode := dag.Nodes[producerID]
	consumerNode := dag.Nodes[consumerID]
	if _, exists := consumerNode.StreamDependsOn[producerID]; !exists {
		consumerNode.StreamDependsOn[producerID] = producerNode
		producerNode.RequiredBy[consumerID] = consumerNode
	}
}

func addStateEdge(dag *DAG, producerID, consumerID string) {
	producerNode := dag.Nodes[producerID]
	consumerNode := dag.Nodes[consumerID]
	if _, exists := consumerNode.StateDependsOn[producerID]; !exists {
		consumerNode.StateDependsOn[producerID] = producerNode
		producerNode.RequiredBy[consumerID] = consumerNode
	}
}

func detectCycle(dag *DAG) error {
	path := make(map[string]bool)
	visited := make(map[string]bool)

	for id := range dag.Nodes {
		if !visited[id] {
			if hasCycleDFS(dag, id, path, visited) {
				// The cycle error message will be implicitly more useful now,
				// as the DAG is correctly built.
				return gxoerrors.NewConfigError("cycle detected in task dependencies", nil)
			}
		}
	}
	return nil
}

func hasCycleDFS(dag *DAG, nodeID string, path map[string]bool, visited map[string]bool) bool {
	node := dag.Nodes[nodeID]
	path[nodeID] = true
	visited[nodeID] = true

	for dependentID := range node.RequiredBy {
		if path[dependentID] {
			return true
		}
		if !visited[dependentID] {
			if hasCycleDFS(dag, dependentID, path, visited) {
				return true
			}
		}
	}

	path[nodeID] = false
	return false
}

// collectTemplatesToScan now correctly includes the 'when' clause.
func collectTemplatesToScan(task *config.Task) []string {
	templates := []string{}
	if task.When != "" {
		templates = append(templates, task.When)
	}
	if loopStr, ok := task.Loop.(string); ok && loopStr != "" {
		templates = append(templates, loopStr)
	}
	for _, paramValue := range task.Params {
		if strValue, ok := paramValue.(string); ok {
			if strings.Contains(strValue, "{{") && strings.Contains(strValue, "}}") {
				templates = append(templates, strValue)
			}
		}
	}
	return templates
}