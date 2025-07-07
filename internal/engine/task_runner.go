package engine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gxo-labs/gxo/internal/config"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/retry"
	"github.com/gxo-labs/gxo/internal/secrets"
	intTemplate "github.com/gxo-labs/gxo/internal/template"
	"github.com/gxo-labs/gxo/internal/util"
	intTracing "github.com/gxo-labs/gxo/internal/tracing"

	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	pkgsecrets "github.com/gxo-labs/gxo/pkg/gxo/v1/secrets"
	gxov1state "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
	gxotracing "github.com/gxo-labs/gxo/pkg/gxo/v1/tracing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	codes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type policyAwareStateReader struct {
	store      gxov1state.Store
	accessMode config.StateAccessMode
}

func (r *policyAwareStateReader) Get(key string) (interface{}, bool) {
	val, exists := r.store.Get(key)
	if !exists {
		return nil, false
	}
	if r.accessMode == config.StateAccessUnsafeDirectReference {
	}
	return val, true
}

func (r *policyAwareStateReader) GetAll() map[string]interface{} {
	return r.store.GetAll()
}

type taskExecutionContext struct {
	task       *config.Task
	state      gxov1state.StateReader
	logger     gxolog.Logger
	hookData   map[interface{}]interface{}
	hookDataMu sync.RWMutex
}

func newExecutionContext(task *config.Task, state gxov1state.StateReader, logger gxolog.Logger) *taskExecutionContext {
	return &taskExecutionContext{
		task:     task,
		state:    state,
		logger:   logger,
		hookData: make(map[interface{}]interface{}),
	}
}

func (c *taskExecutionContext) Task() *config.Task              { return c.task }
func (c *taskExecutionContext) State() gxov1state.StateReader    { return c.state }
func (c *taskExecutionContext) Logger() gxolog.Logger            { return c.logger }
func (c *taskExecutionContext) Get(key interface{}) interface{} {
	c.hookDataMu.RLock()
	defer c.hookDataMu.RUnlock()
	return c.hookData[key]
}
func (c *taskExecutionContext) Set(key interface{}, value interface{}) {
	c.hookDataMu.Lock()
	defer c.hookDataMu.Unlock()
	c.hookData[key] = value
}

var _ module.ExecutionContext = (*taskExecutionContext)(nil)

type TaskRunner struct {
	stateManager           gxov1state.Store
	pluginRegistry         plugin.Registry
	log                    gxolog.Logger
	channelManager         *ChannelManager
	retryHelper            *retry.Helper
	eventBus               events.Bus
	hooks                  []module.ExecutionHook
	secretsProvider        pkgsecrets.Provider
	tracerProvider         gxotracing.TracerProvider
	redactedKeywords       map[string]struct{}
	defaultTimeout         time.Duration
	secretsRedactedCounter prometheus.Counter
}

func NewTaskRunner(
	stateMgr gxov1state.Store,
	plugReg plugin.Registry,
	log gxolog.Logger,
	chanMgr *ChannelManager,
	retryHelper *retry.Helper,
	eventBus events.Bus,
	hooks []module.ExecutionHook,
	secretsProvider pkgsecrets.Provider,
	tracerProvider gxotracing.TracerProvider,
	redactedKeywords map[string]struct{},
	defaultTimeout time.Duration,
) *TaskRunner {
	return &TaskRunner{
		stateManager:     stateMgr,
		pluginRegistry:   plugReg,
		log:              log,
		channelManager:   chanMgr,
		retryHelper:      retryHelper,
		eventBus:         eventBus,
		hooks:            hooks,
		secretsProvider:  secretsProvider,
		tracerProvider:   tracerProvider,
		redactedKeywords: redactedKeywords,
		defaultTimeout:   defaultTimeout,
	}
}

func (r *TaskRunner) ExecuteTask(
	ctx context.Context,
	task *config.Task,
	node *Node,
	taskLogger gxolog.Logger,
	tracer oteltrace.Tracer,
	aggregatedErrChan chan<- error,
	taskInstanceRenderer intTemplate.Renderer,
	secretTracker *secrets.SecretTracker,
) (finalSummary interface{}, finalErr error) {
	defer func() {
		if producerWgs, exists := r.channelManager.GetConsumerProducerWaitGroups(task.InternalID); exists {
			for _, wg := range producerWgs {
				wg.Done()
			}
			taskLogger.Debugf("Consumer task %s signaled completion to %d producers.", task.InternalID, len(producerWgs))
		}
	}()

	isNoopTracer := r.tracerProvider == nil || r.tracerProvider.(*intTracing.OtelTracerProvider).IsEffectivelyNoOp()

	var taskSpan oteltrace.Span
	taskCtx := ctx
	if !isNoopTracer {
		taskCtx, taskSpan = tracer.Start(ctx, "gxo.task.run", oteltrace.WithAttributes(
			attribute.String("gxo.task.id", task.InternalID),
			attribute.String("gxo.task.name", task.Name),
			attribute.String("gxo.task.type", task.Type),
		))
		defer taskSpan.End()
	}

	policyReader := &policyAwareStateReader{
		store:      r.stateManager,
		accessMode: node.StatePolicy.AccessMode,
	}

	if task.When != "" {
		taskLogger.Debugf("Evaluating 'when' condition")
		conditionResult, err := taskInstanceRenderer.Render(task.When, policyReader.GetAll())
		if err != nil {
			redactedErr := intTemplate.RedactSecretsInError(err, r.redactedKeywords)
			finalErr = gxoerrors.NewSkippedError(fmt.Sprintf("'when' condition error: %v", redactedErr))
			if taskSpan != nil {
				taskSpan.SetStatus(codes.Error, finalErr.Error())
				taskSpan.SetAttributes(attribute.String("gxo.task.status", "Skipped"))
			}
			return nil, finalErr
		}
		if !evaluateConditionString(conditionResult) {
			taskLogger.Infof("Skipping task due to 'when' condition evaluating to false: [%s]", conditionResult)
			finalErr = gxoerrors.NewSkippedError("'when' condition false")
			if taskSpan != nil {
				taskSpan.SetStatus(codes.Ok, finalErr.Error())
				taskSpan.SetAttributes(attribute.String("gxo.task.status", "Skipped"))
			}
			return nil, finalErr
		}
	}

	loopItems, loopErr := r.resolveLoopItems(task.Loop, policyReader, taskInstanceRenderer)
	if loopErr != nil {
		finalErr = fmt.Errorf("failed to resolve loop items for task '%s': %w", task.InternalID, loopErr)
		if taskSpan != nil {
			intTracing.RecordErrorWithContext(taskSpan, finalErr, r.redactedKeywords)
		}
		return nil, finalErr
	}
	if task.Loop != nil && len(loopItems) == 0 {
		taskLogger.Infof("Loop resulted in zero items. Task considered completed (no-op).")
		if taskSpan != nil {
			taskSpan.SetAttributes(attribute.Int("gxo.task.loop_iterations", 0))
			taskSpan.SetStatus(codes.Ok, "")
		}
		return nil, nil
	}
	if taskSpan != nil && len(loopItems) > 0 {
		taskSpan.SetAttributes(attribute.Int("gxo.task.loop_iterations", len(loopItems)))
	}

	parallelism := task.GetLoopParallel()
	loopVarName := task.GetLoopVar()
	var finalInstanceSummary interface{}
	var finalInstanceErr error
	var loopErrMu sync.Mutex

	effectiveTimeout := r.defaultTimeout
	if taskSpecificTimeout := task.GetTimeout(); taskSpecificTimeout > 0 {
		effectiveTimeout = taskSpecificTimeout
	}

	instanceCtx := taskCtx
	if effectiveTimeout > 0 {
		var instanceCancel context.CancelFunc
		instanceCtx, instanceCancel = context.WithTimeout(instanceCtx, effectiveTimeout)
		defer instanceCancel()
	}

	if len(loopItems) > 0 {
		var loopWg sync.WaitGroup
		semaphore := make(chan struct{}, parallelism)
		for i, item := range loopItems {
			select {
			case <-instanceCtx.Done():
				loopErrMu.Lock()
				if finalInstanceErr == nil {
					finalInstanceErr = instanceCtx.Err()
				}
				loopErrMu.Unlock()
				goto LoopEnd
			case semaphore <- struct{}{}:
			}

			loopWg.Add(1)
			go func(index int, currentItem interface{}) {
				defer loopWg.Done()
				defer func() { <-semaphore }()

				if instanceCtx.Err() != nil {
					return
				}

				iterLogger := taskLogger.With("loop_iteration", index)
				iterSummary, iterErr := r.executeSingleTaskInstance(
					instanceCtx, task, node, iterLogger, policyReader,
					map[string]interface{}{loopVarName: currentItem},
					aggregatedErrChan, index, tracer, isNoopTracer,
					taskInstanceRenderer, // Pass taskInstanceRenderer
				)

				loopErrMu.Lock()
				if iterErr != nil && !gxoerrors.IsSkipped(iterErr) {
					if finalInstanceErr == nil {
						finalInstanceErr = iterErr
					}
				} else if iterErr == nil && finalInstanceErr == nil {
					finalInstanceSummary = iterSummary
				}
				loopErrMu.Unlock()
			}(i, item)
		}
	LoopEnd:
		loopWg.Wait()
	} else {
		finalInstanceSummary, finalInstanceErr = r.executeSingleTaskInstance(
			instanceCtx, task, node, taskLogger, policyReader, nil,
			aggregatedErrChan, -1, tracer, isNoopTracer,
			taskInstanceRenderer, // Pass taskInstanceRenderer
		)
	}

	if taskSpan != nil {
		status := "Completed"
		if finalInstanceErr != nil {
			if gxoerrors.IsSkipped(finalInstanceErr) {
				status = "Skipped"
				taskSpan.SetStatus(codes.Ok, "Task skipped")
			} else {
				status = "Failed"
				intTracing.RecordErrorWithContext(taskSpan, finalInstanceErr, r.redactedKeywords)
			}
		} else {
			taskSpan.SetStatus(codes.Ok, "")
		}
		taskSpan.SetAttributes(attribute.String("gxo.task.status", status))
	}

	if finalInstanceErr == nil && task.Register != "" {
		redactedSummary, wasRedacted := intTemplate.RedactTrackedSecrets(finalInstanceSummary, secretTracker)
		if wasRedacted {
			taskLogger.Warnf("SECURITY WARNING: Task '%s' summary contained one or more resolved secrets. The secret values have been redacted before registration.", task.Name)
			if r.secretsRedactedCounter != nil {
				r.secretsRedactedCounter.Inc()
			}
		}
		if regErr := r.stateManager.Set(task.Register, redactedSummary); regErr != nil {
			finalInstanceErr = fmt.Errorf("failed to register result: %w", regErr)
		} else {
			finalInstanceSummary = redactedSummary
		}
	}

	return finalInstanceSummary, finalInstanceErr
}

func (r *TaskRunner) executeSingleTaskInstance(
	ctx context.Context,
	task *config.Task,
	node *Node,
	taskLogger gxolog.Logger,
	policyReader gxov1state.StateReader,
	loopScopeData map[string]interface{},
	aggregatedErrChan chan<- error,
	loopIteration int,
	tracer oteltrace.Tracer,
	isNoopTracer bool,
	taskInstanceRenderer intTemplate.Renderer,
) (summary interface{}, err error) {
	defer func() {
		if _, exists := r.channelManager.GetOutputManagedChannels(task.InternalID); exists {
			r.channelManager.CloseOutputChannels(task.InternalID)
			taskLogger.Debugf("Producer task %s closed its output channels.", task.InternalID)
		}
	}()

	execCtx := newExecutionContext(task, policyReader, taskLogger)
	var finalErr error

	var instanceSpan oteltrace.Span
	instanceCtx := ctx
	if !isNoopTracer {
		spanName := "gxo.task.instance"
		if loopIteration >= 0 {
			spanName = fmt.Sprintf("gxo.task.instance.%d", loopIteration)
		}
		instanceCtx, instanceSpan = tracer.Start(instanceCtx, spanName)
		defer instanceSpan.End()
	}

	defer func() {
		if instanceSpan != nil {
			if finalErr != nil {
				intTracing.RecordErrorWithContext(instanceSpan, finalErr, r.redactedKeywords)
			} else {
				instanceSpan.SetStatus(codes.Ok, "")
			}
		}
		for _, hook := range r.hooks {
			if hookErr := hook.AfterExecute(execCtx, summary, finalErr); hookErr != nil {
				taskLogger.Errorf("Error running AfterExecute hook (%T): %v", hook, hookErr)
			}
		}
	}()

	for _, hook := range r.hooks {
		if hookErr := hook.BeforeExecute(execCtx); hookErr != nil {
			finalErr = fmt.Errorf("BeforeExecute hook failed: %w", hookErr)
			return nil, finalErr
		}
	}

	factory, getErr := r.pluginRegistry.Get(task.Type)
	if getErr != nil {
		return nil, getErr
	}
	pluginInstance := factory()

	templateData := policyReader.GetAll()
	for k, v := range loopScopeData {
		templateData[k] = v
	}

	renderedParams := make(map[string]interface{})
	for key, value := range task.Params {
		if strValue, ok := value.(string); ok {
			resolvedValue, renderErr := taskInstanceRenderer.Resolve(strValue, templateData)
			if renderErr != nil {
				finalErr = fmt.Errorf("parameter resolution failed for '%s': %w", key, renderErr)
				return nil, finalErr
			}
			renderedParams[key] = resolvedValue
		} else {
			renderedParams[key] = value
		}
	}

	if len(task.StreamInputs) > 0 {
		producerIDMap := make(map[string]interface{})
		for _, depNode := range node.StreamDependsOn {
			if depNode.Task != nil && depNode.Task.Name != "" {
				producerIDMap[depNode.Task.Name] = depNode.ID
			}
		}
		renderedParams[module.ProducerIDMapKey] = producerIDMap
	}

	inputChansMap, _ := r.channelManager.GetInputChannelMap(task.InternalID)
	_, managedOutputChansExist := r.channelManager.GetOutputManagedChannels(task.InternalID)

	rawOutputChans := make([]chan<- map[string]interface{}, 0)
	if managedOutputChansExist {
		managedOutputChans, _ := r.channelManager.GetOutputManagedChannels(task.InternalID)
		for _, mc := range managedOutputChans {
			rawOutputChans = append(rawOutputChans, mc.channel)
		}
	}
	moduleErrChan := make(chan error, 10)

	retryCfg := retry.Config{Attempts: task.GetRetryAttempts(), Delay: task.GetRetryDelay(), MaxDelay: task.GetRetryMaxDelay(), BackoffFactor: task.GetRetryBackoffFactor(), Jitter: task.GetRetryJitter(), OnError: task.ShouldRetryOnError(), TaskName: task.InternalID}

	performErr := r.retryHelper.Do(instanceCtx, retryCfg, func(opCtx context.Context) error {
		var performSpan oteltrace.Span
		performCtx := opCtx
		if !isNoopTracer {
			performCtx, performSpan = tracer.Start(opCtx, "gxo.plugin.perform", oteltrace.WithAttributes(attribute.String("gxo.plugin.type", task.Type)))
			defer performSpan.End()
		}

		eventPayload := map[string]interface{}{"task_id": task.InternalID, "task_name": task.Name}
		r.eventBus.Emit(events.Event{Type: events.ModuleExecutionStart, Timestamp: time.Now(), TaskName: task.Name, TaskID: task.InternalID, Payload: eventPayload})

		performSummary, performErr := pluginInstance.Perform(performCtx, renderedParams, policyReader, inputChansMap, rawOutputChans, moduleErrChan)

		if performSpan != nil {
			if performErr != nil {
				intTracing.RecordErrorWithContext(performSpan, performErr, r.redactedKeywords)
			} else {
				performSpan.SetStatus(codes.Ok, "")
			}
		}

		eventPayload["error"] = performErr
		eventPayload["summary"] = performSummary
		r.eventBus.Emit(events.Event{Type: events.ModuleExecutionEnd, Timestamp: time.Now(), TaskName: task.Name, TaskID: task.InternalID, Payload: eventPayload})

		if performErr == nil {
			summary = performSummary
		}
		return performErr
	})

	if performErr == nil && managedOutputChansExist {
		if wg, exists := r.channelManager.GetProducerWaitGroup(task.InternalID); exists {
			go func() {
				taskLogger.Debugf("Producer task %s waiting for consumers in background.", task.InternalID)
				wg.Wait()
				taskLogger.Debugf("All consumers for task %s have finished. Producer task is fully complete.", task.InternalID)
			}()
		}
	}

	close(moduleErrChan)
	for errFromChan := range moduleErrChan {
		if errFromChan == nil {
			continue
		}
		var rpe *gxoerrors.RecordProcessingError
		wrappedErr := errFromChan
		if !errors.As(errFromChan, &rpe) {
			wrappedErr = gxoerrors.NewRecordProcessingError(task.Name, nil, errFromChan)
		}
		if aggregatedErrChan != nil {
			select {
			case aggregatedErrChan <- wrappedErr:
			case <-instanceCtx.Done():
			}
		}
	}

	finalErr = performErr
	return summary, finalErr
}

func (r *TaskRunner) resolveLoopItems(loopInput interface{}, state gxov1state.StateReader, renderer intTemplate.Renderer) ([]interface{}, error) {
	if loopInput == nil {
		return nil, nil
	}
	var items interface{}
	var err error

	if loopStr, ok := loopInput.(string); ok {
		items, err = renderer.Resolve(loopStr, state.GetAll())
		if err != nil {
			return nil, fmt.Errorf("could not resolve loop variable expression '%s': %w", loopStr, err)
		}
	} else {
		items = util.DeepCopy(loopInput)
	}

	return extractItems(items)
}

func extractItems(data interface{}) ([]interface{}, error) {
	if data == nil {
		return nil, nil
	}
	val := reflect.ValueOf(data)
	kind := val.Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		count := val.Len()
		items := make([]interface{}, count)
		for i := 0; i < count; i++ {
			items[i] = val.Index(i).Interface()
		}
		return items, nil
	case reflect.Map:
		count := val.Len()
		items := make([]interface{}, 0, count)
		iter := val.MapRange()
		for iter.Next() {
			items = append(items, iter.Value().Interface())
		}
		return items, nil
	default:
		return nil, fmt.Errorf("expected slice, array, or map for loop items, got %T", data)
	}
}

func evaluateConditionString(s string) bool {
	sLower := strings.ToLower(strings.TrimSpace(s))
	switch sLower {
	case "false", "no", "off", "0", "":
		return false
	default:
		if num, err := strconv.ParseFloat(s, 64); err == nil {
			return num != 0
		}
		return true
	}
}