package observability

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	ottrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

const (
	defaultServiceName  = "backend-challenge-api"
	defaultOTLPEndpoint = "otel-collector:4317"

	shutdownTimeout = 10 * time.Second
)

type Config struct {
	ServiceName string
	Endpoint    string
	Insecure    bool
}

type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
}

func NewConfig() Config {
	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = defaultOTLPEndpoint
	}

	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	insecure := true

	if value := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE")); value != "" {
		insecure = !strings.EqualFold(value, "false")
	}

	return Config{
		ServiceName: serviceName,
		Endpoint:    endpoint,
		Insecure:    insecure,
	}
}

func New(lc fx.Lifecycle, cfg Config) (*Providers, error) {
	ctx := context.Background()

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("deployment.environment", environment()),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExporterOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		traceExporterOptions = append(
			traceExporterOptions,
			otlptracegrpc.WithInsecure(),
		)
	}

	traceExporter, err := otlptracegrpc.New(
		ctx,
		traceExporterOptions...,
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
	)

	metricExporterOptions := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		metricExporterOptions = append(
			metricExporterOptions,
			otlpmetricgrpc.WithInsecure(),
		)
	}

	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		metricExporterOptions...,
	)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(5*time.Second),
			),
		),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	providers := &Providers{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(
				ctx,
				shutdownTimeout,
			)
			defer cancel()

			var shutdownErr error

			if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
				shutdownErr = err
			}

			if err := meterProvider.Shutdown(shutdownCtx); err != nil {
				if shutdownErr == nil {
					shutdownErr = err
				}
			}

			return shutdownErr
		},
	})

	return providers, nil
}

func Tracer(providers *Providers) ottrace.Tracer {
	if providers == nil || providers.TracerProvider == nil {
		return otel.Tracer(defaultServiceName)
	}

	return providers.TracerProvider.Tracer(
		"backend-challenge-api",
	)
}

func Meter(providers *Providers) otmetric.Meter {
	if providers == nil || providers.MeterProvider == nil {
		return otel.Meter(defaultServiceName)
	}

	return providers.MeterProvider.Meter(
		"backend-challenge-api",
	)
}

func environment() string {
	value := strings.TrimSpace(
		os.Getenv("OTEL_RESOURCE_ATTRIBUTES"),
	)

	for _, attributeValue := range strings.Split(value, ",") {
		parts := strings.SplitN(attributeValue, "=", 2)
		if len(parts) != 2 {
			continue
		}

		if strings.TrimSpace(parts[0]) == "deployment.environment" {
			environment := strings.TrimSpace(parts[1])
			if environment != "" {
				return environment
			}
		}
	}

	return "local"
}
