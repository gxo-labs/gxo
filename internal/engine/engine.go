package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/metrics"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/secrets"
	gxov1state "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
	gxotracing "github.com/gxo-labs/gxo/pkg/gxo/v1/tracing"

	"github.com/gxo-labs/gxo/internal/config"
	intEvents "github.com/gxo-labs/gxo/internal/events"
	intMetrics "github.com/gxo-labs/gxo/internal/metrics"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/retry"
	intSecrets "github.com/gxo-labs/gxo/internal/secrets"
	intState "github.com/gxo-labs/gxo/internal/state"
	"github.com/gxo-labs/gxo/internal/template"
	intTracing "github.com/gxo-labs/gxo/internal/tracing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	codes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	StateKeyGxoTasksPrefix = template.GxoStateKeyPrefix + ".tasks"
	tracerName             = "gxo-engine"
)

// Engine is the core orchestration component of GXO.
type Engine struct {
	// Core Services & Providers
	stateManager    gxov1state.Store
	secretsProvider secrets.Provider
	eventBus        events.Bus
	pluginRegistry  plugin.Registry
	metricsProvider metrics.RegistryProvider
	tracerProvider  gxotracing.TracerProvider
	log             gxolog.Logger
	channelManager  *ChannelManager
	retryHelper     *retry.Helper
	taskRunner      *TaskRunner
	hooks           []module.ExecutionHook

	// Configuration & Policies
	workerPoolSize        int
	defaultChannelPolicy  *config.ChannelPolicy
	defaultTimeout        time.Duration
	redactedKeywords      map[string]struct{}
	redactedKeywordsSlice []string
	stallPolicy           *config.StallPolicy

	// Runtime State
	workQueue        chan string
	dag              *DAG
	totalTasks       int32
	completedTasks   atomic.Int32
	activeWorkers    atomic.Int32
	readyChan        chan string
	taskStatuses     map[string]TaskStatus
	statusMu         sync.RWMutex
	taskTimings      map[string]taskTiming
	timingsMu        sync.RWMutex
	taskErrors       map[string]error
	errorsMu         sync.Mutex

	// Metrics Collectors
	playbookCounter        *prometheus.CounterVec
	playbookDuration       prometheus.Histogram
	taskDuration           *prometheus.HistogramVec
	taskCounter            *prometheus.CounterVec
	activeWorkersGauge     prometheus.Gauge
	secretsAccessEvents    prometheus.Counter
	secretsRedactedCounter prometheus.Counter
}

type taskTiming struct {
	start time.Time
	end   time.Time
}

var _ gxo.EngineV1 = (*Engine)(nil)

func NewEngine(log gxolog.Logger, opts ...gxo.EngineOption) (*Engine, error) {
	if log == nil {
		return nil, gxoerrors.NewConfigError("logger cannot be nil", nil)
	}

	e := &Engine{
		log:              log,
		taskStatuses:     make(map[string]TaskStatus),
		taskTimings:      make(map[string]taskTiming),
		taskErrors:       make(map[string]error),
		hooks:            []module.ExecutionHook{},
		workerPoolSize:   runtime.NumCPU(),
		redactedKeywords: make(map[string]struct{}),
		defaultTimeout:   0,
		stallPolicy: &config.StallPolicy{
			Interval:  1 * time.Second,
			Tolerance: 5,
		},
	}

	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, gxoerrors.NewConfigError(fmt.Sprintf("failed to apply engine option: %v", err), err)
		}
	}

	if e.stateManager == nil {
		e.log.Warnf("No state store provided, using default in-memory store.")
		e.stateManager = intState.NewMemoryStateStore()
	}
	if e.secretsProvider == nil {
		e.log.Warnf("No secrets provider provided, using default environment provider.")
		e.secretsProvider = intSecrets.NewEnvProvider()
	}
	if e.eventBus == nil {
		e.log.Warnf("No event bus provided, using default NoOp bus.")
		e.eventBus = intEvents.NewNoOpEventBus()
	}
	if e.pluginRegistry == nil {
		e.log.Warnf("No plugin registry provided, using default static registry.")
		e.pluginRegistry = module.DefaultStaticRegistryGetter
	}
	if e.metricsProvider == nil {
		e.log.Warnf("No metrics provider provided, using default Prometheus provider.")
		e.metricsProvider = intMetrics.NewPrometheusRegistryProvider()
	}
	if e.tracerProvider == nil {
		e.log.Warnf("No tracer provider provided, using default NoOp provider.")
		tp, err := intTracing.NewNoOpProvider()
		if err != nil {
			return nil, gxoerrors.NewConfigError("failed to create default NoOp tracer provider", err)
		}
		e.tracerProvider = tp
	}

	if len(e.redactedKeywords) == 0 && len(e.redactedKeywordsSlice) > 0 {
		_ = e.SetRedactedKeywords(e.redactedKeywordsSlice)
	}

	e.channelManager = NewChannelManager(e.defaultChannelPolicy)
	e.retryHelper = retry.NewHelper(e.log)
	e.retryHelper.SetRedactedKeywords(e.redactedKeywords)

	e.taskRunner = NewTaskRunner(
		e.stateManager,
		e.pluginRegistry,
		e.log,
		e.channelManager,
		e.retryHelper,
		e.eventBus,
		e.hooks,
		e.secretsProvider,
		e.tracerProvider,
		e.redactedKeywords,
		e.defaultTimeout,
	)

	e.initMetrics()
	e.taskRunner.secretsRedactedCounter = e.secretsRedactedCounter

	return e, nil
}

func (e *Engine) GetSecretAccessCounter() prometheus.Counter {
	return e.secretsAccessEvents
}

func (e *Engine) initMetrics() {
	if e.metricsProvider == nil {
		e.log.Warnf("Metrics provider is nil, skipping metrics initialization.")
		return
	}
	reg := e.metricsProvider.Registry()
	if reg == nil {
		e.log.Errorf("Metrics provider returned a nil registry, cannot initialize metrics.")
		return
	}

	e.playbookCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gxo_playbook_runs_total", Help: "Total number of playbook runs completed or failed."},
		[]string{"playbook_name", "status"},
	)
	reg.MustRegister(e.playbookCounter)

	e.playbookDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "gxo_playbook_run_duration_seconds", Help: "Duration of playbook runs in seconds.", Buckets: prometheus.DefBuckets},
	)
	reg.MustRegister(e.playbookDuration)

	e.taskDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "gxo_task_run_duration_seconds", Help: "Duration of individual task executions in seconds.", Buckets: prometheus.DefBuckets},
		[]string{"playbook_name", "task_name", "task_type"},
	)
	reg.MustRegister(e.taskDuration)

	e.taskCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gxo_task_runs_total", Help: "Total number of task executions by final status."},
		[]string{"playbook_name", "task_name", "task_type", "status"},
	)
	reg.MustRegister(e.taskCounter)

	e.activeWorkersGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "gxo_engine_active_workers", Help: "Number of currently active task execution workers."},
	)
	reg.MustRegister(e.activeWorkersGauge)

	e.secretsAccessEvents = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "gxo_secrets_accessed_total", Help: "Total number of secrets accessed via the 'secret' template function."},
	)
	if e.secretsProvider != nil {
		err := reg.Register(e.secretsAccessEvents)
		if err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				e.log.Warnf("Failed to register secretsAccessEvents metric collector: %v", err)
			} else {
				e.log.Debugf("secretsAccessEvents metric collector already registered.")
			}
		}
	}

	e.secretsRedactedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "gxo_secrets_redacted_total", Help: "Total number of secrets automatically redacted from task summaries before registration."},
	)
	err := reg.Register(e.secretsRedactedCounter)
	if err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			e.log.Warnf("Failed to register secretsRedactedCounter metric collector: %v", err)
		} else {
			e.log.Debugf("secretsRedactedCounter metric collector already registered.")
		}
	}

	e.log.Debugf("Prometheus metrics initialized and registered.")
}

func (e *Engine) RunPlaybook(ctx context.Context, playbookYAML []byte) (finalReport *gxo.ExecutionReport, finalErr error) {
	tracer := e.tracerProvider.GetTracer(tracerName)
	runCtx, span := tracer.Start(ctx, "gxo.playbook.run")
	defer span.End()

	startTime := time.Now()
	var playbook *config.Playbook
	var loadErr error

	defer func() {
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		pbName := ""
		if playbook != nil {
			pbName = playbook.Name
		}

		if finalReport == nil {
			finalReport = e.generateReport(pbName, startTime, endTime, finalErr)
		} else {
			finalReport.StartTime = startTime
			finalReport.EndTime = endTime
			finalReport.Duration = duration
			if finalReport.Error == "" && finalErr != nil {
				finalReport.Error = template.RedactSecretsInError(finalErr, e.redactedKeywords).Error()
				finalReport.OverallStatus = "Failed"
			}
		}

		status := "Failed"
		if finalErr == nil && finalReport.OverallStatus == "Completed" {
			status = "Completed"
		} else if finalReport.OverallStatus != "" {
			status = finalReport.OverallStatus
		}

		if e.playbookDuration != nil {
			e.playbookDuration.Observe(duration.Seconds())
		}
		if e.playbookCounter != nil {
			e.playbookCounter.WithLabelValues(pbName, status).Inc()
		}

		span.SetAttributes(
			attribute.String("gxo.playbook.name", pbName),
			attribute.String("gxo.playbook.status", status),
			attribute.Int64("gxo.playbook.duration_ms", duration.Milliseconds()),
			attribute.Int("gxo.playbook.total_tasks", finalReport.TotalTasks),
			attribute.Int("gxo.playbook.completed_tasks", finalReport.CompletedTasks),
			attribute.Int("gxo.playbook.failed_tasks", finalReport.FailedTasks),
			attribute.Int("gxo.playbook.skipped_tasks", finalReport.SkippedTasks),
		)
		if finalErr != nil {
			intTracing.RecordErrorWithContext(span, finalErr, e.redactedKeywords)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		e.emitFinalEvents(finalReport)
		e.log.Infof("Playbook execution finished.")
	}()

	playbook, loadErr = config.LoadPlaybook(playbookYAML, "playbook.yaml")
	if loadErr != nil {
		e.log.Errorf("Failed to load or validate playbook: %v", loadErr)
		finalErr = loadErr
		intTracing.RecordErrorWithContext(span, finalErr, e.redactedKeywords)
		span.SetStatus(codes.Error, "Playbook load/validation failed")
		return nil, finalErr
	}
	e.log.Infof("Starting playbook execution: %s (Schema: %s)", playbook.Name, playbook.SchemaVersion)
	span.SetAttributes(attribute.String("gxo.playbook.name", playbook.Name))

	e.eventBus.Emit(events.Event{Type: events.PlaybookStart, Timestamp: startTime, PlaybookName: playbook.Name, Payload: map[string]interface{}{"playbook_name": playbook.Name}})

	e.taskStatuses = make(map[string]TaskStatus)
	e.taskTimings = make(map[string]taskTiming)
	e.taskErrors = make(map[string]error)
	e.completedTasks.Store(0)
	e.activeWorkers.Store(0)

	runCtx, cancelRun := context.WithCancel(runCtx)
	defer cancelRun()

	if err := e.stateManager.Load(playbook.Vars); err != nil {
		e.log.Errorf("Failed to load initial playbook variables: %v", err)
		finalErr = fmt.Errorf("failed to load initial vars: %w", err)
		intTracing.RecordErrorWithContext(span, finalErr, e.redactedKeywords)
		return nil, finalErr
	}
	e.log.Debugf("Loaded initial variables into state.")

	e.log.Infof("Building execution DAG...")
	dummyRendererForDAG := template.NewGoRenderer(e.secretsProvider, e.eventBus, nil)
	var initialReadyNodes []*Node
	var buildDagErr error
	e.dag, initialReadyNodes, buildDagErr = BuildDAG(playbook, e.stateManager, dummyRendererForDAG)
	if buildDagErr != nil {
		e.log.Errorf("Failed to build DAG: %v", buildDagErr)
		finalErr = fmt.Errorf("failed to build DAG: %w", buildDagErr)
		intTracing.RecordErrorWithContext(span, finalErr, e.redactedKeywords)
		return nil, finalErr
	}
	e.totalTasks = int32(len(e.dag.Nodes))
	if e.totalTasks == 0 {
		e.log.Infof("Playbook has no tasks to execute.")
		span.SetAttributes(attribute.Int("gxo.playbook.total_tasks", 0))
		return nil, nil
	}
	e.log.Infof("DAG built successfully. Found %d tasks. Initial ready: %d", e.totalTasks, len(initialReadyNodes))
	span.SetAttributes(attribute.Int("gxo.playbook.total_tasks", int(e.totalTasks)))

	if err := e.channelManager.CreateChannels(e.dag); err != nil {
		e.log.Errorf("Failed to create execution channels: %v", err)
		finalErr = fmt.Errorf("failed to create channels: %w", err)
		intTracing.RecordErrorWithContext(span, finalErr, e.redactedKeywords)
		return nil, finalErr
	}

	readyChanBufferSize := int(e.totalTasks) + e.workerPoolSize
	workQueueBufferSize := e.workerPoolSize * 2
	fatalErrChan := make(chan error, 1)

	e.readyChan = make(chan string, readyChanBufferSize)
	e.workQueue = make(chan string, workQueueBufferSize)

	e.statusMu.Lock()
	e.timingsMu.Lock()
	for id := range e.dag.Nodes {
		e.taskStatuses[id] = StatusPending
		e.taskTimings[id] = taskTiming{}
		if writeErr := e.writeTaskStatus(context.Background(), id, StatusPending); writeErr != nil {
			e.log.Errorf("Failed to write initial pending status for task %s: %v", id, writeErr)
		}
	}
	e.timingsMu.Unlock()
	e.statusMu.Unlock()

	var workerWg sync.WaitGroup
	workerWg.Add(e.workerPoolSize)
	var runningTasksWg sync.WaitGroup
	e.log.Infof("Starting %d execution workers...", e.workerPoolSize)
	for i := 0; i < e.workerPoolSize; i++ {
		go e.worker(runCtx, &workerWg, &runningTasksWg, fatalErrChan, i)
	}
	defer func() {
		close(e.workQueue)
		workerWg.Wait()
		e.log.Debugf("Worker pool shutdown complete.")
	}()

	if len(initialReadyNodes) == 0 && e.totalTasks > 0 {
		finalErr = gxoerrors.NewConfigError("no initially ready tasks found in non-empty DAG (check for cycles or dependency issues)", nil)
		e.log.Errorf(finalErr.Error())
		intTracing.RecordErrorWithContext(span, finalErr, e.redactedKeywords)
		return nil, finalErr
	}
	for _, node := range initialReadyNodes {
		e.log.Debugf("Seeding ready queue with initial task: %s", node.ID)
		e.readyChan <- node.ID
	}

	var firstFatalError error
	dispatchedTasks := make(map[string]bool)
	var dispatchMu sync.Mutex
	lastAccountedForCount := int32(-1)
	stallChecks := 0
	tasksAccountedFor := int32(0)

	ticker := time.NewTicker(e.stallPolicy.Interval)
	defer ticker.Stop()

	if e.activeWorkersGauge != nil {
		e.activeWorkersGauge.Set(0)
	}

SchedulingLoop:
	for tasksAccountedFor < e.totalTasks {
		if e.activeWorkersGauge != nil {
			e.activeWorkersGauge.Set(float64(e.activeWorkers.Load()))
		}

		select {
		case taskID := <-e.readyChan:
			stallChecks = 0
			dispatchMu.Lock()
			if dispatchedTasks[taskID] {
				dispatchMu.Unlock()
				e.log.Debugf("Task %s received from readyChan but already dispatched. Ignoring duplicate.", taskID)
				continue
			}

			e.statusMu.Lock()
			if e.taskStatuses[taskID] != StatusPending {
				e.log.Warnf("Task %s was ready but status is now %s, not dispatching.", taskID, e.taskStatuses[taskID])
				e.statusMu.Unlock()
				dispatchMu.Unlock()
				continue
			}

			dispatchedTasks[taskID] = true
			e.taskStatuses[taskID] = StatusRunning
			e.timingsMu.Lock()
			e.taskTimings[taskID] = taskTiming{start: time.Now()}
			e.timingsMu.Unlock()
			if writeErr := e.writeTaskStatus(runCtx, taskID, StatusRunning); writeErr != nil {
				e.log.Errorf("Failed to write running status for task %s: %v", taskID, writeErr)
			}
			e.statusMu.Unlock()
			dispatchMu.Unlock()

			e.log.Infof("Dispatching task to worker queue: %s", taskID)
			node := e.dag.Nodes[taskID]
			e.signalStreamDependents(node)
			runningTasksWg.Add(1)

			select {
			case e.workQueue <- taskID:
			case <-runCtx.Done():
				e.log.Warnf("Context cancelled while trying to dispatch task %s", taskID)
				runningTasksWg.Done()
				e.handleTaskCompletion(runCtx, taskID, StatusFailed, runCtx.Err(), fatalErrChan, true)
			}

		case err := <-fatalErrChan:
			stallChecks = 0
			redactedErr := template.RedactSecretsInError(err, e.redactedKeywords)
			e.log.Errorf("Received fatal error signal: %v. Initiating cancellation.", redactedErr)
			if firstFatalError == nil {
				firstFatalError = err
			}
			cancelRun()

		case <-runCtx.Done():
			e.log.Warnf("Playbook context cancelled (%v), terminating scheduling loop.", runCtx.Err())
			if firstFatalError == nil {
				firstFatalError = runCtx.Err()
			}
			break SchedulingLoop

		case <-ticker.C:
			currentAccounted := e.countTerminalTasks()
			tasksAccountedFor = currentAccounted

			if tasksAccountedFor >= e.totalTasks {
				continue SchedulingLoop
			}

			activeWorkersCount := e.activeWorkers.Load()
			runnablePendingTasksCount := e.countRunnablePendingTasks()

			if activeWorkersCount == 0 && runnablePendingTasksCount == 0 {
				if e.hasPendingTasks() {
					e.log.Infof("Execution stable: No active workers or runnable pending tasks. Blocked tasks remain.")
					break SchedulingLoop
				}
			}

			if currentAccounted == lastAccountedForCount {
				stallChecks++
				if stallChecks >= e.stallPolicy.Tolerance {
					if !(activeWorkersCount == 0 && runnablePendingTasksCount == 0 && e.hasPendingTasks()) {
						stallMsg := fmt.Sprintf("playbook execution stalled: %d/%d tasks accounted for, %d active workers, %d runnable tasks. No progress for %v.",
							currentAccounted, e.totalTasks, activeWorkersCount, runnablePendingTasksCount, time.Duration(stallChecks)*e.stallPolicy.Interval)
						e.log.Log(slog.LevelError, stallMsg)
						if firstFatalError == nil {
							firstFatalError = errors.New("playbook execution stalled")
						}
						cancelRun()
						break SchedulingLoop
					} else {
						stallChecks = 0
					}
				}
			} else {
				stallChecks = 0
			}
			lastAccountedForCount = currentAccounted
		}
	}

	e.log.Debugf("Main scheduling loop finished. Tasks accounted for: %d/%d", tasksAccountedFor, e.totalTasks)

	waitChan := make(chan struct{})
	go func() {
		runningTasksWg.Wait()
		close(waitChan)
	}()
	waitTimeout := 10 * time.Second
	select {
	case <-waitChan:
		e.log.Debugf("All dispatched tasks WaitGroup finished.")
	case <-time.After(waitTimeout):
		e.log.Errorf("Timeout (%v) waiting for running tasks WaitGroup after main loop exit.", waitTimeout)
		if firstFatalError == nil {
			firstFatalError = fmt.Errorf("timeout waiting for running tasks WaitGroup")
		}
	case <-runCtx.Done():
		e.log.Warnf("Context cancelled while waiting for running tasks WaitGroup: %v", runCtx.Err())
		if firstFatalError == nil {
			firstFatalError = fmt.Errorf("cancelled while waiting for running tasks WaitGroup: %w", runCtx.Err())
		}
	}

	finalErr = e.determineFinalOutcome(firstFatalError)

	return finalReport, finalErr
}

func (e *Engine) worker(
	ctx context.Context,
	workerWg *sync.WaitGroup,
	runningTasksWg *sync.WaitGroup,
	fatalErrChan chan<- error,
	workerID int,
) {
	defer workerWg.Done()
	workerLogger := e.log.With("worker_id", workerID)
	workerLogger.Debugf("Worker started.")
	tracer := e.tracerProvider.GetTracer(tracerName)

	for {
		select {
		case taskID, ok := <-e.workQueue:
			if !ok {
				workerLogger.Debugf("Work queue closed, worker exiting.")
				return
			}

			func() {
				e.activeWorkers.Add(1)
				defer e.activeWorkers.Add(-1)
				defer runningTasksWg.Done()

				node, exists := e.dag.Nodes[taskID]
				if !exists {
					workerLogger.Errorf("Worker received unknown task ID from queue: %s", taskID)
					e.handleTaskCompletion(ctx, taskID, StatusFailed, fmt.Errorf("task %s definition not found in DAG", taskID), fatalErrChan, true)
					return
				}

				taskLogger := workerLogger
				if node.Task.Name != "" {
					taskLogger = taskLogger.With("task_name", node.Task.Name)
				}
				taskLogger = taskLogger.With("task_id", taskID)

				taskExecCtx := ctx

				if taskExecCtx.Err() != nil {
					taskLogger.Warnf("Context cancelled/timed out before worker could start task: %v", taskExecCtx.Err())
					e.handleTaskCompletion(taskExecCtx, taskID, StatusFailed, taskExecCtx.Err(), fatalErrChan, false)
					return
				}

				taskLogger.Debugf("Worker picked up task")
				e.runTaskAndHandleCompletion(taskExecCtx, node, taskLogger, tracer, fatalErrChan)
			}()

		case <-ctx.Done():
			workerLogger.Debugf("Worker context cancelled (%v), exiting.", ctx.Err())
			return
		}
	}
}

func (e *Engine) runTaskAndHandleCompletion(
	ctx context.Context,
	node *Node,
	taskLogger gxolog.Logger,
	tracer oteltrace.Tracer,
	fatalErrChan chan<- error,
) {
	task := node.Task
	taskID := node.ID
	taskFinalStatus := StatusFailed

	aggregatedErrChan := make(chan error, 10)

	e.eventBus.Emit(events.Event{
		Type:      events.TaskStart,
		Timestamp: time.Now(),
		TaskName:  task.Name,
		TaskID:    taskID,
		Payload:   map[string]interface{}{"task_id": taskID, "task_name": task.Name},
	})

	// Create the GoRenderer instance here, unique to this task execution,
	// and pass it to the TaskRunner.
	secretTracker := intSecrets.NewSecretTracker()
	taskInstanceRenderer := template.NewGoRenderer(e.secretsProvider, e.eventBus, secretTracker)

	// Correctly handle the two return values from ExecuteTask.
	_, taskErr := e.taskRunner.ExecuteTask(ctx, task, node, taskLogger, tracer, aggregatedErrChan, taskInstanceRenderer, secretTracker)

	if taskErr == nil {
		taskFinalStatus = StatusCompleted
	} else if gxoerrors.IsSkipped(taskErr) {
		taskFinalStatus = StatusSkipped
		taskLogger.Infof("Task skipped: %v", taskErr)
	} else {
		redactedErr := template.RedactSecretsInError(taskErr, e.redactedKeywords)
		if errors.Is(taskErr, context.Canceled) || errors.Is(taskErr, context.DeadlineExceeded) {
			taskLogger.Warnf("Task execution failed: %v", redactedErr)
		} else {
			taskLogger.Errorf("Task execution failed fatally: %v", redactedErr)
		}
	}

	e.eventBus.Emit(events.Event{
		Type:      events.TaskEnd,
		Timestamp: time.Now(),
		TaskName:  task.Name,
		TaskID:    taskID,
		Payload: map[string]interface{}{
			"task_id":      taskID,
			"task_name":    task.Name,
			"final_status": string(taskFinalStatus),
			"error":        taskErr,
		},
	})

	e.handleTaskCompletion(ctx, taskID, taskFinalStatus, taskErr, fatalErrChan, false)
}

func (e *Engine) handleTaskCompletion(
	ctx context.Context,
	taskID string,
	finalStatus TaskStatus,
	taskErr error,
	fatalErrChan chan<- error,
	synthetic bool,
) {
	e.statusMu.Lock()
	taskName := taskID
	node := e.dag.Nodes[taskID]
	if node != nil && node.Task != nil && node.Task.Name != "" {
		taskName = node.Task.Name
	}

	currentStatus := e.taskStatuses[taskID]
	isAlreadyTerminal := currentStatus == StatusCompleted || currentStatus == StatusFailed || currentStatus == StatusSkipped

	if !synthetic && isAlreadyTerminal {
		e.statusMu.Unlock()
		e.log.Debugf("Ignoring duplicate completion signal for task %s ('%s'). Already in terminal state %s.", taskID, taskName, currentStatus)
		return
	}

	var taskDuration time.Duration
	e.timingsMu.Lock()
	timing := e.taskTimings[taskID]
	if timing.end.IsZero() {
		timing.end = time.Now()
		e.taskTimings[taskID] = timing
	}
	if !timing.start.IsZero() {
		taskDuration = timing.end.Sub(timing.start)
	}
	e.timingsMu.Unlock()

	oldStatus := currentStatus
	e.taskStatuses[taskID] = finalStatus
	e.errorsMu.Lock()
	if taskErr != nil {
		e.taskErrors[taskID] = template.RedactSecretsInError(taskErr, e.redactedKeywords)
	} else {
		delete(e.taskErrors, taskID)
	}
	e.errorsMu.Unlock()
	e.statusMu.Unlock()

	e.eventBus.Emit(events.Event{
		Type: events.TaskStatusChanged, Timestamp: time.Now(),
		TaskName: taskName, TaskID: taskID,
		Payload: map[string]interface{}{
			"task_id": taskID, "task_name": taskName,
			"old_status": string(oldStatus), "new_status": string(finalStatus),
		},
	})

	if writeErr := e.writeTaskStatus(ctx, taskID, finalStatus); writeErr != nil {
		e.log.LogCtx(ctx, slog.LevelError, "Failed to write final task status to state store",
			"task_id", taskID, "task_name", taskName, "status", finalStatus, "error", writeErr)
	}

	taskType := ""
	pbName := ""
	if node != nil && node.Task != nil {
		taskType = node.Task.Type
	}
	if e.taskCounter != nil {
		e.taskCounter.WithLabelValues(pbName, taskName, taskType, string(finalStatus)).Inc()
	}
	if e.taskDuration != nil && taskDuration > 0 {
		e.taskDuration.WithLabelValues(pbName, taskName, taskType).Observe(taskDuration.Seconds())
	}

	if oldStatus == StatusRunning || oldStatus == StatusPending {
		completedCount := e.completedTasks.Add(1)
		e.log.Debugf("Task %s ('%s') finished with status %s. Completed count: %d/%d", taskID, taskName, finalStatus, completedCount, e.totalTasks)
	} else {
		e.log.Warnf("Task %s ('%s') completion handler called but old status was already terminal (%s). Completed count: %d/%d.", taskID, taskName, oldStatus, e.completedTasks.Load(), e.totalTasks)
	}

	if node == nil {
		if !synthetic {
			e.log.Errorf("Internal error: Node %s not found during completion handling.", taskID)
		}
		return
	}

	if finalStatus == StatusCompleted || finalStatus == StatusSkipped {
		e.signalStateDependents(node)
	}

	if finalStatus == StatusFailed && !node.Task.IgnoreErrors {
		if taskErr != nil && !errors.Is(taskErr, context.Canceled) && !errors.Is(taskErr, context.DeadlineExceeded) && !gxoerrors.IsSkipped(taskErr) {
			select {
			case fatalErrChan <- taskErr:
				e.log.Warnf("Task %s ('%s') failed fatally (unignored), signaling playbook halt.", taskID, taskName)
			default:
				e.log.Debugf("Task %s ('%s') failed fatally, but halt already signaled.", taskID, taskName)
			}
		}
	}
}

func (e *Engine) isTaskReady(node *Node) bool {
	return node.StreamDepsRemaining.Load() == 0 && node.StateDepsRemaining.Load() == 0
}

func (e *Engine) signalStreamDependents(node *Node) {
	for _, dependentNode := range node.RequiredBy {
		if _, isStreamDep := dependentNode.StreamDependsOn[node.ID]; isStreamDep {
			if dependentNode.StreamDepsRemaining.Add(-1) == 0 {
				e.log.Debugf("Task %s stream dependencies met.", dependentNode.ID)
				if e.isTaskReady(dependentNode) {
					e.readyChan <- dependentNode.ID
				}
			}
		}
	}
}

func (e *Engine) signalStateDependents(node *Node) {
	for _, dependentNode := range node.RequiredBy {
		if _, isStateDep := dependentNode.StateDependsOn[node.ID]; isStateDep {
			if dependentNode.StateDepsRemaining.Add(-1) == 0 {
				e.log.Debugf("Task %s state dependencies met.", dependentNode.ID)
				if e.isTaskReady(dependentNode) {
					e.readyChan <- dependentNode.ID
				}
			}
		}
	}
}

func (e *Engine) writeTaskStatus(ctx context.Context, taskID string, status TaskStatus) error {
	taskName := taskID
	if e.dag != nil {
		if node, ok := e.dag.Nodes[taskID]; ok && node.Task != nil && node.Task.Name != "" {
			taskName = node.Task.Name
		}
	}
	stateKey := fmt.Sprintf("%s.%s.status", StateKeyGxoTasksPrefix, taskName)
	err := e.stateManager.Set(stateKey, string(status))
	if err != nil {
		e.log.LogCtx(ctx, slog.LevelError, "Failed to write task status to state", "key", stateKey, "status", status, "error", err)
	}
	return err
}

func (e *Engine) countTerminalTasks() int32 {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	count := int32(0)
	for _, status := range e.taskStatuses {
		if status == StatusCompleted || status == StatusFailed || status == StatusSkipped {
			count++
		}
	}
	return count
}

func (e *Engine) countRunnablePendingTasks() int {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	count := 0
	if e.dag == nil {
		e.log.Warnf("countRunnablePendingTasks called with nil DAG")
		return 0
	}
	for id, status := range e.taskStatuses {
		if status == StatusPending {
			if node, ok := e.dag.Nodes[id]; ok && node != nil {
				if e.isTaskReady(node) {
					count++
				}
			} else {
				e.log.Warnf("Inconsistency: Status found for task %s but node missing in DAG during runnable check.", id)
			}
		}
	}
	return count
}

func (e *Engine) hasPendingTasks() bool {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	for _, status := range e.taskStatuses {
		if status == StatusPending {
			return true
		}
	}
	return false
}

func (e *Engine) determineFinalOutcome(firstFatalError error) error {
	if firstFatalError != nil {
		if errors.Is(firstFatalError, context.Canceled) || errors.Is(firstFatalError, context.DeadlineExceeded) {
			return firstFatalError
		}
		if firstFatalError.Error() == "playbook execution stalled" {
			return firstFatalError
		}
		return gxoerrors.NewConfigError("playbook finished due to fatal error", template.RedactSecretsInError(firstFatalError, e.redactedKeywords))
	}

	hasFailedTasks := false
	hasUnexpectedPending := false

	e.statusMu.RLock()
	defer e.statusMu.RUnlock()

	for id, status := range e.taskStatuses {
		if status == StatusFailed {
			hasFailedTasks = true
		}
		if status == StatusPending {
			isExpectedPending := false
			if e.dag != nil {
				if node, nodeExists := e.dag.Nodes[id]; nodeExists && node != nil {
					if node.StateDepsRemaining.Load() > 0 {
						isExpectedPending = true
					}
				} else if e.totalTasks > 0 {
					e.log.Warnf("Task %s has status but node not found in DAG during final outcome check.", id)
					hasUnexpectedPending = true
				}
			} else if e.totalTasks > 0 {
				e.log.Log(slog.LevelError, "Internal consistency error: DAG is nil but totalTasks > 0 during final outcome check.")
				hasUnexpectedPending = true
			}
			if !isExpectedPending {
				hasUnexpectedPending = true
				e.log.Warnf("Task %s remains Pending unexpectedly at end of execution.", id)
			}
		}
	}

	if hasUnexpectedPending {
		return gxoerrors.NewConfigError("playbook finished with unexpected pending tasks (potential deadlock or internal error)", nil)
	}
	if hasFailedTasks {
		return gxoerrors.NewConfigError("playbook finished with one or more failed tasks", nil)
	}

	return nil
}

func (e *Engine) generateReport(playbookName string, start, end time.Time, finalExecError error) *gxo.ExecutionReport {
	report := &gxo.ExecutionReport{
		PlaybookName:  playbookName,
		StartTime:     start,
		EndTime:       end,
		Duration:      end.Sub(start),
		TaskResults:   make(map[string]gxo.TaskResult),
		OverallStatus: "Completed",
	}

	if finalExecError != nil {
		report.OverallStatus = "Failed"
		report.Error = template.RedactSecretsInError(finalExecError, e.redactedKeywords).Error()
	}

	e.statusMu.RLock()
	e.timingsMu.RLock()
	e.errorsMu.Lock()
	defer e.errorsMu.Unlock()
	defer e.timingsMu.RUnlock()
	defer e.statusMu.RUnlock()

	for id, status := range e.taskStatuses {
		timing := e.taskTimings[id]
		taskErrStr := ""
		if taskErr, exists := e.taskErrors[id]; exists && taskErr != nil {
			taskErrStr = taskErr.Error()
		}

		taskDuration := time.Duration(0)
		if !timing.start.IsZero() && !timing.end.IsZero() {
			taskDuration = timing.end.Sub(timing.start)
		}

		switch status {
		case StatusFailed:
			report.FailedTasks++
			if taskErrStr == "" {
				taskErrStr = "Task failed (unknown error)"
			}
		case StatusCompleted:
			report.CompletedTasks++
		case StatusSkipped:
			report.SkippedTasks++
			if taskErr, exists := e.taskErrors[id]; exists && gxoerrors.IsSkipped(taskErr) {
				taskErrStr = taskErr.Error()
			}
		case StatusPending, StatusRunning:
			e.log.Warnf("Task %s found in non-terminal state (%s) during report generation.", id, status)
			if report.OverallStatus == "Completed" {
				report.OverallStatus = "Failed"
				if report.Error == "" {
					report.Error = "Playbook finished with non-terminal tasks"
				}
			}
			if status == StatusPending {
				taskErrStr = "Task remained in Pending state"
			} else {
				taskErrStr = "Task remained in Running state"
			}
			report.FailedTasks++
		}

		report.TaskResults[id] = gxo.TaskResult{
			Status:    string(status),
			Error:     taskErrStr,
			StartTime: timing.start,
			EndTime:   timing.end,
			Duration:  taskDuration,
		}
	}
	report.TotalTasks = len(e.taskStatuses)

	if report.FailedTasks > 0 && report.OverallStatus == "Completed" {
		report.OverallStatus = "Failed"
		if report.Error == "" {
			report.Error = "Playbook finished with one or more failed tasks"
		}
	}

	return report
}

func (e *Engine) emitFinalEvents(report *gxo.ExecutionReport) {
	if report == nil || e.eventBus == nil {
		return
	}
	payload := map[string]interface{}{
		"playbook_name": report.PlaybookName, "duration_ms": report.Duration.Milliseconds(),
		"status": report.OverallStatus, "total_tasks": report.TotalTasks,
		"completed": report.CompletedTasks, "failed": report.FailedTasks, "skipped": report.SkippedTasks,
		"error_message": report.Error,
	}
	e.eventBus.Emit(events.Event{Type: events.PlaybookEnd, Timestamp: report.EndTime, PlaybookName: report.PlaybookName, Payload: payload})
}

func (e *Engine) MetricsRegistryProvider() metrics.RegistryProvider { return e.metricsProvider }
func (e *Engine) TracerProvider() gxotracing.TracerProvider          { return e.tracerProvider }

func (e *Engine) SetStateStore(store gxov1state.Store) error {
	if store == nil {
		return gxoerrors.NewConfigError("state store cannot be nil", nil)
	}
	e.stateManager = store
	if e.taskRunner != nil {
		e.taskRunner.stateManager = store
	}
	return nil
}

func (e *Engine) SetSecretsProvider(provider secrets.Provider) error {
	if provider == nil {
		return gxoerrors.NewConfigError("secrets provider cannot be nil", nil)
	}
	e.secretsProvider = provider
	if e.taskRunner != nil {
		e.taskRunner.secretsProvider = provider
	}
	return nil
}

func (e *Engine) SetEventBus(bus events.Bus) error {
	if bus == nil {
		return gxoerrors.NewConfigError("event bus cannot be nil", nil)
	}
	e.eventBus = bus
	if e.taskRunner != nil {
		e.taskRunner.eventBus = bus
	}
	return nil
}

func (e *Engine) SetPluginRegistry(registry plugin.Registry) error {
	if registry == nil {
		return gxoerrors.NewConfigError("plugin registry cannot be nil", nil)
	}
	e.pluginRegistry = registry
	if e.taskRunner != nil {
		e.taskRunner.pluginRegistry = registry
	}
	return nil
}

func (e *Engine) SetMetricsRegistryProvider(provider metrics.RegistryProvider) error {
	if provider == nil {
		return gxoerrors.NewConfigError("metrics registry provider cannot be nil", nil)
	}
	e.metricsProvider = provider
	e.initMetrics()
	return nil
}

func (e *Engine) SetTracerProvider(provider gxotracing.TracerProvider) error {
	if provider == nil {
		return gxoerrors.NewConfigError("tracer provider cannot be nil", nil)
	}
	e.tracerProvider = provider
	if e.taskRunner != nil {
		e.taskRunner.tracerProvider = provider
	}
	return nil
}

func (e *Engine) SetDefaultTimeout(timeout time.Duration) error {
	if timeout < 0 {
		return gxoerrors.NewConfigError("default timeout cannot be negative", nil)
	}
	e.defaultTimeout = timeout
	if e.taskRunner != nil {
		e.taskRunner.defaultTimeout = timeout
	}
	return nil
}

func (e *Engine) SetWorkerPoolSize(size int) error {
	if size <= 0 {
		return gxoerrors.NewConfigError("worker pool size must be positive", nil)
	}
	e.workerPoolSize = size
	return nil
}

func (e *Engine) SetDefaultChannelPolicy(policy gxo.ChannelPolicy) error {
	internalPolicy := &config.ChannelPolicy{BufferSize: policy.BufferSize, OverflowStrategy: policy.OverflowStrategy}
	switch internalPolicy.OverflowStrategy {
	case "", config.OverflowBlock, config.OverflowDropNew, config.OverflowDropOldest, config.OverflowError:
	default:
		return gxoerrors.NewConfigError(fmt.Sprintf("invalid default channel policy overflow_strategy: '%s'", internalPolicy.OverflowStrategy), nil)
	}
	if internalPolicy.BufferSize != nil && *internalPolicy.BufferSize < 0 {
		return gxoerrors.NewConfigError("default channel policy buffer_size cannot be negative", nil)
	}
	e.defaultChannelPolicy = internalPolicy
	if e.channelManager != nil {
		e.channelManager = NewChannelManager(e.defaultChannelPolicy)
	}
	return nil
}

func (e *Engine) SetStallPolicy(policy *config.StallPolicy) error {
	if policy == nil {
		return gxoerrors.NewConfigError("stall policy cannot be nil", nil)
	}
	if policy.Interval <= 0 {
		return gxoerrors.NewConfigError("stall policy interval must be positive", nil)
	}
	if policy.Tolerance <= 0 {
		return gxoerrors.NewConfigError("stall policy tolerance must be positive", nil)
	}
	e.stallPolicy = policy
	return nil
}

func (e *Engine) SetRedactedKeywords(keywords []string) error {
	e.redactedKeywordsSlice = keywords
	newMap := make(map[string]struct{})
	for _, k := range keywords {
		keyLower := strings.ToLower(strings.TrimSpace(k))
		if keyLower != "" {
			newMap[keyLower] = struct{}{}
		}
	}
	e.redactedKeywords = newMap
	if e.taskRunner != nil {
		e.taskRunner.redactedKeywords = e.redactedKeywords
	}
	if e.retryHelper != nil {
		e.retryHelper.SetRedactedKeywords(e.redactedKeywords)
	}
	return nil
}