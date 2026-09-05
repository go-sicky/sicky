package grpc

import (
	"context"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/go-sicky/sicky/metrics"
	sickytracer "github.com/go-sicky/sicky/tracer"
)

type stubServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *stubServerStream) Context() context.Context { return s.ctx }

func counterValue(c interface {
	Write(metric *dto.Metric) error
},
) float64 {
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		panic(err)
	}

	return m.GetCounter().GetValue()
}

func TestUnaryTracingExtractsW3C(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	defer func() {
		_ = provider.Shutdown(context.Background())
	}()
	tr := provider.Tracer("test")

	// Build an upstream W3C context and freeze it into metadata.
	upCtx, upSpan := tr.Start(context.Background(), "upstream")
	carrier := propagation.MapCarrier{}
	sickytracer.Inject(upCtx, carrier)
	upTraceID := upSpan.SpanContext().TraceID().String()
	upSpan.End()

	pairs := []string{}
	for k, v := range carrier {
		if v == "" {
			continue
		}

		pairs = append(pairs, k, v)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...))

	iv := NewTracingInterceptor(TracerConfig{Tracer: tr})
	called := false
	_, err := iv(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			called = true
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				t.Fatalf("handler ctx must carry incoming metadata")
			}

			// New span values must shadow client-supplied ones and
			// W3C traceparent must be re-injected downstream.
			if vals := md.Get("traceparent"); len(vals) == 0 || vals[0] == "" {
				t.Fatalf("downstream metadata must carry traceparent, md=%v", md)
			}

			// Child span must continue the upstream trace.
			if vals := md.Get("x-b3-traceid"); len(vals) == 0 || vals[0] != upTraceID {
				t.Fatalf("trace not continued: x-b3-traceid=%v want %v", vals, upTraceID)
			}

			return nil, nil
		})
	if err != nil || !called {
		t.Fatalf("interceptor must call handler, called=%v err=%v", called, err)
	}
}

func TestServerStreamNilTracerPassesThrough(t *testing.T) {
	called := false
	iv := NewStreamTracingInterceptor(TracerConfig{})
	err := iv(nil, &stubServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/svc/Method"},
		func(srv any, ss grpc.ServerStream) error {
			called = true

			return nil
		})
	if err != nil || !called {
		t.Fatalf("nil-tracer stream interceptor must call handler, called=%v err=%v", called, err)
	}
}

func TestStreamInterceptorsCount(t *testing.T) {
	before := counterValue(metrics.NumGRPCServerAccessCounter)

	siv := NewStreamAccessLoggerInterceptor()
	_ = siv(nil, &stubServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/svc/M"},
		func(srv any, ss grpc.ServerStream) error { return nil })
	if got := counterValue(metrics.NumGRPCServerAccessCounter); got != before+1 {
		t.Fatalf("server stream counter want %v got %v", before+1, got)
	}
}
