package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"

	"github.com/gxo-labs/gxo/internal/config"
	"github.com/gxo-labs/gxo/internal/engine"
	"github.com/gxo-labs/gxo/internal/events"
	"github.com/gxo-labs/gxo/internal/logger"
	"github.com/gxo-labs/gxo/internal/metrics"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/secrets"
	"github.com/gxo-labs/gxo/internal/state"
	"github.com/gxo-labs/gxo/internal/tracing"

	_ "github.com/gxo-labs/gxo/modules/exec"
	_ "github.com/gxo-labs/gxo/modules/passthrough"
)

const (
	ExitSuccess              = 0
	ExitFailure              = 1
	ExitUsageError           = 2
	ExitTimeout              = 124
	ExitSigIntBase           = 128
	ExitSigInt               = ExitSigIntBase + int(syscall.SIGINT)
	ExitSigTerm              = ExitSigIntBase + int(syscall.SIGTERM)
	DefaultLogLevel          = "info"
	DefaultLogFmt            = "text"
	DefaultChannelBufferSize = 100
	DefaultEventBusSize      = 256
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "validate" {
		runValidateCommand(os.Args[2:])
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		printVersion()
		os.Exit(ExitSuccess)
	}
	exitCode := runExecuteCommand(os.Args[1:])
	os.Exit(exitCode)
}

func printVersion() {
	fmt.Printf("gxo version %s\n", version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("built: %s\n", buildDate)
	fmt.Printf("go version: %s\n", runtime.Version())
	fmt.Printf("os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func runValidateCommand(args []string) {
	validateFlags := flag.NewFlagSet("validate", flag.ExitOnError)
	playbookPath := validateFlags.String("playbook", "", "Path to the playbook YAML file to validate (required)")
	logLevel := validateFlags.String("log-level", DefaultLogLevel, "Log level for validation output (debug, info, warn, error)")

	validateFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s validate -playbook <path> [flags...]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Validates the structure and schema compatibility of a GXO playbook.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		validateFlags.PrintDefaults()
	}

	if err := validateFlags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing validate flags: %v\n", err)
		os.Exit(ExitUsageError)
	}

	if *playbookPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -playbook flag is required for validation")
		validateFlags.Usage()
		os.Exit(ExitUsageError)
	}

	log := logger.NewLogger(*logLevel, "text", os.Stderr)
	log.Infof("Validating playbook: %s", *playbookPath)

	playbookBytes, err := os.ReadFile(*playbookPath)
	if err != nil {
		log.Errorf("Failed to read playbook file '%s': %v", *playbookPath, err)
		os.Exit(ExitFailure)
	}

	_, err = config.LoadPlaybook(playbookBytes, *playbookPath)
	if err != nil {
		var validationErr *gxoerrors.ValidationError
		var configErr *gxoerrors.ConfigError
		if errors.As(err, &validationErr) {
			log.Errorf("Playbook validation failed:\n%s", validationErr.Error())
		} else if errors.As(err, &configErr) {
			log.Errorf("Playbook configuration error:\n%s", configErr.Error())
		} else {
			log.Errorf("Failed to load or validate playbook: %v", err)
		}
		os.Exit(ExitFailure)
	}

	log.Infof("Playbook validation successful: %s", *playbookPath)
	os.Exit(ExitSuccess)
}

func runExecuteCommand(args []string) int {
	execFlags := flag.NewFlagSet("gxo", flag.ExitOnError)
	playbookPath := execFlags.String("playbook", "", "Path to the main playbook YAML file (required)")
	logLevel := execFlags.String("log-level", DefaultLogLevel, "Log level (debug, info, warn, error)")
	logFormat := execFlags.String("log-format", DefaultLogFmt, "Log format (text, json)")
	dryRun := execFlags.Bool("dry-run", false, "Execute playbook in dry-run mode (simulate actions)")
	workerPoolSize := execFlags.Int("worker-pool-size", runtime.NumCPU(), "Number of task execution workers")
	defaultChannelBufferSize := execFlags.Int("channel-buffer-size", DefaultChannelBufferSize, "Default buffer size for streaming channels")
	versionFlag := execFlags.Bool("version", false, "Print version information and exit")

	execFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags...] -playbook <path>\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Executes a GXO playbook.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		execFlags.PrintDefaults()
	}

	if err := execFlags.Parse(args); err != nil {
		return ExitUsageError
	}

	if *versionFlag {
		printVersion()
		return ExitSuccess
	}

	if *playbookPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -playbook flag is required")
		execFlags.Usage()
		return ExitUsageError
	}
	if *logFormat != "text" && *logFormat != "json" {
		fmt.Fprintln(os.Stderr, "Error: -log-format must be 'text' or 'json'")
		return ExitUsageError
	}
	if *workerPoolSize <= 0 {
		*workerPoolSize = runtime.NumCPU()
		fmt.Fprintf(os.Stderr, "Warning: -worker-pool-size must be positive, defaulting to %d\n", *workerPoolSize)
	}
	if *defaultChannelBufferSize < 0 {
		fmt.Fprintf(os.Stderr, "Error: -channel-buffer-size cannot be negative. Using default %d.\n", DefaultChannelBufferSize)
		*defaultChannelBufferSize = DefaultChannelBufferSize
	}

	var logWriter io.Writer = os.Stderr
	log := logger.NewLogger(*logLevel, *logFormat, logWriter)
	log = log.With("gxo_version", version)

	log.Infof("GXO Automation Kernel v%s starting...", version)
	log.Debugf("Log level: %s", *logLevel)
	log.Debugf("Log format: %s", *logFormat)
	log.Debugf("Worker pool size: %d", *workerPoolSize)
	log.Debugf("Default channel buffer size: %d", *defaultChannelBufferSize)

	stateStore := state.NewMemoryStateStore()
	eventBus := events.NewChannelEventBus(DefaultEventBusSize, log)
	defer eventBus.Close()
	secretsProvider := secrets.NewEnvProvider()
	pluginRegistry := module.DefaultStaticRegistryGetter
	metricsProvider := metrics.NewPrometheusRegistryProvider()
	tracerProvider, err := tracing.NewProviderFromEnv(context.Background())
	if err != nil {
		log.Warnf("Failed to initialize tracing from environment: %v. Using NoOp tracer.", err)
		tracerProvider, _ = tracing.NewNoOpProvider()
	}

	defaultChanPolicy := gxo.ChannelPolicy{
		BufferSize: defaultChannelBufferSize,
	}

	engineOpts := []gxo.EngineOption{
		gxo.WithStateStore(stateStore),
		gxo.WithEventBus(eventBus),
		gxo.WithSecretsProvider(secretsProvider),
		gxo.WithPluginRegistry(pluginRegistry),
		gxo.WithTracerProvider(tracerProvider),
		gxo.WithMetricsRegistryProvider(metricsProvider),
		gxo.WithWorkerPoolSize(*workerPoolSize),
		gxo.WithDefaultChannelPolicy(defaultChanPolicy),
		gxo.WithRedactedKeywords([]string{"password", "token", "secret", "apikey", "privatekey", "authorization", "bearer"}),
	}

	ctx := context.Background()
	if *dryRun {
		ctx = context.WithValue(ctx, module.DryRunKey{}, true)
		log.Infof("Dry run mode enabled.")
	}

	internalEngine, err := engine.NewEngine(log, engineOpts...)
	if err != nil {
		log.Errorf("Failed to create GXO engine: %v", err)
		return ExitFailure
	}
	var gxoEngine gxo.EngineV1 = internalEngine

	log.Infof("Loading playbook: %s", *playbookPath)
	playbookBytes, err := os.ReadFile(*playbookPath)
	if err != nil {
		log.Errorf("Failed to read playbook file '%s': %v", *playbookPath, err)
		return ExitFailure
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	listener := events.NewMetricsEventListener(eventBus, internalEngine.GetSecretAccessCounter(), log)
	go listener.Start(runCtx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	defer close(sigChan)

	var receivedSignal os.Signal
	var sigMu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case sig := <-sigChan:
			log.Warnf("Received signal: %v. Initiating graceful shutdown...", sig)
			sigMu.Lock()
			receivedSignal = sig
			sigMu.Unlock()
			cancelRun()
		case <-runCtx.Done():
			log.Debugf("Signal handler exiting because run context is done.")
		}
	}()
	defer wg.Wait()

	log.Infof("Starting playbook execution...")
	report, execErr := gxoEngine.RunPlaybook(runCtx, playbookBytes)

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if shutdownErr := tracerProvider.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Warnf("Error shutting down tracer provider: %v", shutdownErr)
	}

	printReportSummary(log, report, execErr)

	sigMu.Lock()
	finalSignal := receivedSignal
	sigMu.Unlock()
	exitCode := determineExitCode(report, execErr, finalSignal, log)
	return exitCode
}

func printReportSummary(log gxolog.Logger, report *gxo.ExecutionReport, execErr error) {
	if report == nil {
		log.Warnf("Execution finished but no report was generated (likely due to early failure).")
		if execErr != nil {
			logExecutionErrorReason(log, execErr)
		}
		return
	}

	statusLine := fmt.Sprintf("Playbook '%s' finished. Status: %s", report.PlaybookName, report.OverallStatus)
	duration := report.Duration.Truncate(time.Millisecond)
	summaryLine := fmt.Sprintf("Duration: %v. Tasks: Total=%d, Completed=%d, Failed=%d, Skipped=%d",
		duration,
		report.TotalTasks, report.CompletedTasks, report.FailedTasks, report.SkippedTasks)

	if report.OverallStatus == "Failed" || execErr != nil {
		log.Errorf("%s. %s", statusLine, summaryLine)
		if report.Error != "" {
			log.Errorf("Overall Error: %s", report.Error)
		} else if execErr != nil {
			logExecutionErrorReason(log, execErr)
		}
		logFailedTasks(log, report)
	} else {
		log.Infof("%s. %s", statusLine, summaryLine)
	}
}

func logExecutionErrorReason(log gxolog.Logger, execErr error) {
	if errors.Is(execErr, context.Canceled) {
		log.Warnf("Execution Reason: Cancelled.")
	} else if errors.Is(execErr, context.DeadlineExceeded) {
		log.Errorf("Execution Reason: Timeout.")
	} else {
		log.Errorf("Execution Error: %v", execErr)
	}
}

func logFailedTasks(log gxolog.Logger, report *gxo.ExecutionReport) {
	if report.FailedTasks > 0 {
		log.Warnf("Failed Task Details:")
		for taskID, result := range report.TaskResults {
			if result.Status == "Failed" {
				log.Errorf("  - Task '%s': %s", taskID, result.Error)
			}
		}
	}
}

func determineExitCode(report *gxo.ExecutionReport, execErr error, sig os.Signal, log gxolog.Logger) int {
	exitCode := ExitSuccess

	if execErr != nil {
		exitCode = ExitFailure
		if errors.Is(execErr, context.Canceled) && sig != nil {
			switch sig {
			case syscall.SIGINT:
				exitCode = ExitSigInt
				log.Warnf("Playbook execution interrupted by signal: SIGINT")
			case syscall.SIGTERM:
				exitCode = ExitSigTerm
				log.Warnf("Playbook execution terminated by signal: SIGTERM")
			default:
				log.Warnf("Playbook execution terminated by signal: %v", sig)
			}
		} else if errors.Is(execErr, context.Canceled) {
			if execErr.Error() == "playbook execution stalled" {
				log.Errorf("Playbook execution stalled.")
			} else {
				log.Warnf("Playbook execution cancelled internally.")
			}
		} else if errors.Is(execErr, context.DeadlineExceeded) {
			exitCode = ExitTimeout
			log.Errorf("Playbook execution timed out.")
		}
	} else if report != nil && report.OverallStatus == "Failed" {
		log.Errorf("Playbook finished but reported overall status as Failed.")
		exitCode = ExitFailure
	} else {
		log.Infof("Playbook completed successfully.")
		exitCode = ExitSuccess
	}
	return exitCode
}