package tracing

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// Import the public TracerProvider interface definition it implements.
	gxotracing "github.com/gxo-labs/gxo/pkg/gxo/v1/tracing"

	// Import necessary OpenTelemetry packages for implementation details.
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0" // Use specific semantic convention version
	// Import trace package for interface types and NoopTracerProvider
	"go.opentelemetry.io/otel/trace"
	// Import gRPC packages for OTLP/gRPC exporter options
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

// defaultCollectorEndpoint specifies the default OTLP gRPC endpoint if not provided via environment variables.
const defaultCollectorEndpoint = "localhost:4317"

// OtelTracerProvider implements the public gxotracing.TracerProvider interface
// using the OpenTelemetry SDK for actual tracing or the official NoOp provider
// if tracing is disabled or configuration fails.
type OtelTracerProvider struct {
	// provider holds either an SDK provider or the NoOp provider, accessed via the OTel trace.TracerProvider interface.
	provider trace.TracerProvider
	// exporter holds the configured OTLP exporter (gRPC or HTTP) if SDK tracing is enabled. Needed for Shutdown.
	exporter sdktrace.SpanExporter
	// sdkProvider holds the concrete OpenTelemetry SDK *sdktrace.TracerProvider if tracing is enabled.
	// This allows calling SDK-specific methods like Shutdown. It's nil if using the NoOp provider.
	sdkProvider *sdktrace.TracerProvider
}

// NewNoOpProvider creates a TracerProvider instance that performs no tracing operations.
// It utilizes the official OpenTelemetry NoOp implementation.
func NewNoOpProvider() (*OtelTracerProvider, error) {
	// Use the official NoOp provider from the OTel trace package.
	noopTP := trace.NewNoopTracerProvider()
	// Return the wrapper struct containing the NoOp provider.
	return &OtelTracerProvider{
		provider:    noopTP, // Store the NoOp provider interface.
		exporter:    nil,    // No exporter for NoOp.
		sdkProvider: nil,    // No underlying SDK provider for NoOp.
	}, nil
}

// NewProviderFromEnv creates an OtelTracerProvider configured using standard
// OpenTelemetry environment variables (OTEL_*).
// If tracing is disabled (OTEL_SDK_DISABLED=true) or essential configuration
// (like endpoint) is missing or invalid, it falls back to using a NoOp provider.
// This function does *not* set the global OTel provider.
func NewProviderFromEnv(ctx context.Context) (*OtelTracerProvider, error) {
	// Check if tracing is explicitly disabled via environment variable.
	if strings.ToLower(os.Getenv("OTEL_SDK_DISABLED")) == "true" {
		fmt.Println("Info: OpenTelemetry tracing disabled via OTEL_SDK_DISABLED.")
		return NewNoOpProvider()
	}

	// Attempt to create a resource description using environment variables and system info.
	// This adds metadata like service name, host, OS, etc., to traces.
	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL), // Specify schema URL for compatibility.
		resource.WithAttributes(semconv.ServiceNameKey.String(otelServiceName())), // Set service name.
		// Automatically detect process, OS, container, and host information.
		resource.WithProcess(), resource.WithOS(), resource.WithContainer(), resource.WithHost(),
	)
	if err != nil {
		// Use a default resource if detection fails, but log a warning.
		res = resource.Default()
		fmt.Fprintf(os.Stderr, "Warning: Failed to create OTel resource: %v. Using default.\n", err)
	}

	// Create the appropriate OTLP exporter (gRPC or HTTP) based on environment configuration.
	exporter, err := createExporter(ctx)
	if err != nil {
		// If exporter creation fails (e.g., invalid config), log warning and use NoOp.
		fmt.Fprintf(os.Stderr, "Warning: Failed to create OTLP exporter from environment: %v. Using NoOp tracer.\n", err)
		return NewNoOpProvider()
	}
	// If no endpoint was configured, createExporter returns nil. Use NoOp in this case.
	if exporter == nil {
		fmt.Println("Info: OpenTelemetry endpoint not configured (e.g., OTEL_EXPORTER_OTLP_ENDPOINT not set). Using NoOp tracer.")
		return NewNoOpProvider()
	}

	// --- Configure and create the OpenTelemetry SDK TracerProvider ---
	// Use a BatchSpanProcessor for better performance by batching spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	// Create the SDK TracerProvider instance.
	sdkTP := sdktrace.NewTracerProvider(
		// Configure sampling strategy. AlwaysSample is used here for simplicity.
		// Production environments might use ParentBased(TraceIDRatioBased) or other samplers.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		// Associate the resource information with the provider.
		sdktrace.WithResource(res),
		// Register the BatchSpanProcessor to handle exported spans.
		sdktrace.WithSpanProcessor(bsp),
	)

	fmt.Println("Info: OpenTelemetry SDK provider configured based on environment.")
	// Return the wrapper struct containing the configured SDK provider and exporter.
	return &OtelTracerProvider{
		provider:    sdkTP, // Store the SDK provider as the trace.TracerProvider interface.
		exporter:    exporter,
		sdkProvider: sdkTP, // Store the concrete SDK provider type for Shutdown.
	}, nil
}

// createExporter determines the OTLP protocol (gRPC or HTTP) and endpoint from
// environment variables and creates the corresponding span exporter instance.
// Returns nil if no endpoint is configured, or an error for invalid configurations.
func createExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	// Determine protocol (default to grpc).
	protocol := strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	if protocol == "" {
		protocol = "grpc"
	}

	// Get endpoint. If not set, default based on protocol or return nil if unknown.
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	endpointSource := "environment"
	if endpoint == "" {
		endpointSource = "default"
		switch protocol {
		case "grpc":
			endpoint = defaultCollectorEndpoint
		case "http", "http/protobuf":
			endpoint = "localhost:4318" // Default HTTP endpoint
		default:
			// No explicit endpoint and unsupported protocol requires no exporter.
			return nil, nil
		}
		fmt.Printf("Info: OTEL_EXPORTER_OTLP_ENDPOINT not set, using %s endpoint: %s\n", strings.ToUpper(protocol), endpoint)
	}

	// Parse common OTLP environment configurations.
	headers := parseHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	timeout := parseTimeout(os.Getenv("OTEL_EXPORTER_OTLP_TIMEOUT"), 10*time.Second) // 10s default timeout
	compression := os.Getenv("OTEL_EXPORTER_OTLP_COMPRESSION")                       // e.g., "gzip"
	// Check OTEL_EXPORTER_OTLP_INSECURE or OTEL_EXPORTER_OTLP_TRACES_INSECURE
	grpcInsecure := isInsecure(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"), os.Getenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE"))
	httpInsecure := isInsecure(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"), os.Getenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE"))

	// Create exporter based on protocol.
	switch protocol {
	case "grpc":
		// Configure gRPC exporter options.
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithHeaders(headers),
			otlptracegrpc.WithTimeout(timeout),
		}
		if grpcInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure()) // Use plaintext connection.
		} else {
			// Use default TLS credentials from system CA pool if not insecure.
			// More advanced TLS config (custom CAs, client certs) can be added here.
			opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
		}
		if strings.ToLower(compression) == "gzip" {
			opts = append(opts, otlptracegrpc.WithCompressor(gzip.Name))
		}
		fmt.Printf("Info: Configuring OTLP gRPC exporter (endpoint: %s [%s], insecure: %t, compression: %s)\n", endpoint, endpointSource, grpcInsecure, compression)
		// Create and return the gRPC exporter.
		return otlptracegrpc.New(ctx, opts...)

	case "http", "http/protobuf":
		// Configure HTTP exporter options.
		// Determine the specific URL path for the traces endpoint.
		httpPath := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
		if httpPath == "" {
			httpPath = "/v1/traces" // Default path for OTLP/HTTP traces.
		}
		// Construct the base URL, ensuring no double slashes if path is absolute.
		// Prefer WithEndpointOption for robustness.
		baseURL := endpoint // Use the resolved endpoint.

		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(baseURL), // Provide the base endpoint URL.
			otlptracehttp.WithURLPath(httpPath), // Specify the path for traces.
			otlptracehttp.WithHeaders(headers),
			otlptracehttp.WithTimeout(timeout),
		}
		if httpInsecure {
			opts = append(opts, otlptracehttp.WithInsecure()) // Use HTTP instead of HTTPS.
		}
		// Configure compression if specified.
		if strings.ToLower(compression) == "gzip" {
			opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
		}
		fmt.Printf("Info: Configuring OTLP HTTP exporter (endpoint: %s%s [%s], insecure: %t, compression: %s)\n", baseURL, httpPath, endpointSource, httpInsecure, compression)
		// Create and return the HTTP exporter.
		return otlptracehttp.New(ctx, opts...)

	default:
		// Return error for unsupported protocols specified in the environment.
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

// GetTracer returns a named tracer instance using the stored OpenTelemetry provider.
// This method implements the public gxotracing.TracerProvider interface.
// It returns either an SDK tracer or a NoOp tracer depending on initialization.
func (p *OtelTracerProvider) GetTracer(name string, opts ...trace.TracerOption) trace.Tracer {
	// If the internal provider is somehow nil, fallback safely to a NoOp tracer.
	if p.provider == nil {
		return trace.NewNoopTracerProvider().Tracer(name, opts...)
	}
	// Return a tracer from the stored provider (either SDK or NoOp).
	return p.provider.Tracer(name, opts...)
}

// Shutdown gracefully stops the underlying SDK TracerProvider and its associated exporter,
// ensuring buffered spans are flushed before exiting. It respects the provided context's deadline.
// This method implements the public gxotracing.TracerProvider interface.
// It is a no-op if the provider is the NoOp provider.
func (p *OtelTracerProvider) Shutdown(ctx context.Context) error {
	var firstError error

	// Only attempt shutdown if an actual SDK provider is configured.
	if p.sdkProvider != nil {
		fmt.Println("Info: Shutting down OpenTelemetry SDK tracer provider...")
		// Attempt to shut down the SDK provider.
		if err := p.sdkProvider.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down OTel tracer provider: %v\n", err)
			firstError = err // Record the first error encountered.
		}
	}

	// Only attempt shutdown if an actual exporter is configured.
	if p.exporter != nil {
		fmt.Println("Info: Shutting down OpenTelemetry exporter...")
		// Attempt to shut down the exporter.
		if expErr := p.exporter.Shutdown(ctx); expErr != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down OTel exporter: %v\n", expErr)
			// Record exporter error only if no provider error occurred previously.
			if firstError == nil {
				firstError = expErr
			}
		} else {
			fmt.Println("Info: OpenTelemetry exporter shut down successfully.")
		}
	}

	// Log overall success only if resources were actually shut down without error.
	if firstError == nil && (p.sdkProvider != nil || p.exporter != nil) {
		fmt.Println("Info: OpenTelemetry tracing shut down successfully.")
	} else if p.sdkProvider == nil && p.exporter == nil {
		// Log nothing explicit if it was NoOp from the start.
	}
	// Return the first error encountered during shutdown, or nil if successful.
	return firstError
}

// IsEffectivelyNoOp checks if this provider instance is configured to be NoOp.
// This is primarily used internally by the engine to skip span creation.
func (p *OtelTracerProvider) IsEffectivelyNoOp() bool {
	// If sdkProvider is nil, it means we initialized with the NoOp provider.
	return p.sdkProvider == nil
}

// otelServiceName determines the service name, prioritizing OTEL_SERVICE_NAME env var.
func otelServiceName() string {
	name := os.Getenv("OTEL_SERVICE_NAME")
	if name == "" {
		name = "gxo" // Default service name if not set.
	}
	return name
}

// parseHeaders converts a comma-separated key=value string (from OTLP env vars) into a map.
func parseHeaders(headerStr string) map[string]string {
	headers := make(map[string]string)
	if headerStr == "" {
		return headers
	}
	pairs := strings.Split(headerStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2) // Trim spaces around pair
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			if key != "" { // Allow empty values? OTel spec might clarify. Assume yes.
				headers[key] = value
			}
		}
	}
	return headers
}

// parseTimeout converts an OTLP timeout string (milliseconds or Go duration format)
// into a time.Duration, using a default if parsing fails.
func parseTimeout(timeoutStr string, defaultTimeout time.Duration) time.Duration {
	if timeoutStr == "" {
		return defaultTimeout
	}
	// Try parsing as integer milliseconds first (standard OTLP format).
	if timeoutMsInt, err := strconv.ParseInt(timeoutStr, 10, 64); err == nil {
		if timeoutMsInt < 0 { // Ensure non-negative
			return defaultTimeout
		}
		return time.Duration(timeoutMsInt) * time.Millisecond
	}
	// Fallback to parsing as Go duration string (e.g., "5s", "100ms").
	if duration, err := time.ParseDuration(timeoutStr); err == nil {
		if duration < 0 { // Ensure non-negative
			return defaultTimeout
		}
		return duration
	}
	// If all parsing fails, log a warning and return the default.
	fmt.Fprintf(os.Stderr, "Warning: Invalid OTLP timeout format '%s', using default %v\n", timeoutStr, defaultTimeout)
	return defaultTimeout
}

// isInsecure checks common OTLP environment variables to determine if insecure connections are requested.
// It checks both the general insecure flag and the traces-specific flag.
func isInsecure(insecureFlag ...string) bool {
	for _, flag := range insecureFlag {
		if strings.ToLower(strings.TrimSpace(flag)) == "true" {
			return true
		}
	}
	return false
}

// Compile-time check to ensure OtelTracerProvider implements the public TracerProvider interface.
var _ gxotracing.TracerProvider = (*OtelTracerProvider)(nil)