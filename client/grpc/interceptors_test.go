package grpc

import (
	"context"
	"testing"

	"github.com/go-sicky/sicky/metrics"
	dto "github.com/prometheus/client_model/go"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type stubClientStream struct {
	grpc.ClientStream
}

func counterValue(c interface {
	Write(*dto.Metric) error
}) float64 {
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		panic(err)
	}

	return m.GetCounter().GetValue()
}

func TestClientUnaryNilTracerPassesThrough(t *testing.T) {
	called := false
	iv := NewClientTracingInterceptor(nil)
	err := iv(context.Background(), "m", nil, nil, nil,
		func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			called = true
			return nil
		})
	if err != nil || !called {
		t.Fatalf("nil-tracer interceptor must invoke, called=%v err=%v", called, err)
	}
}

func TestClientUnaryInjectsTraceparent(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	defer func() {
		_ = provider.Shutdown(context.Background())
	}()
	iv := NewClientTracingInterceptor(provider.Tracer("test"))

	var captured context.Context
	err := iv(context.Background(), "/svc/Method", nil, nil, nil,
		func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			captured = ctx
			return nil
		})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(captured)
	if !ok {
		t.Fatalf("unary interceptor must set outgoing metadata")
	}
	if vals := md.Get("traceparent"); len(vals) == 0 || vals[0] == "" {
		t.Fatalf("outgoing metadata must carry traceparent, md=%v", md)
	}
	if vals := md.Get("x-b3-traceid"); len(vals) == 0 || vals[0] == "" {
		t.Fatalf("outgoing metadata must carry B3 trace id, md=%v", md)
	}
	if vals := md.Get("x-request-id"); len(vals) == 0 || vals[0] == "" {
		t.Fatalf("outgoing metadata must carry x-request-id, md=%v", md)
	}
}
func TestClientStreamNilTracerPassesThrough(t *testing.T) {
	called := false
	iv := NewClientStreamTracingInterceptor(nil)
	cs, err := iv(context.Background(), &grpc.StreamDesc{}, nil, "/svc/Method",
		func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			called = true
			return &stubClientStream{}, nil
		})
	if err != nil || !called || cs == nil {
		t.Fatalf("nil-tracer client stream must pass through, called=%v err=%v", called, err)
	}
}

func TestClientStreamLoggerCounts(t *testing.T) {
	before := counterValue(metrics.NumGRPCClientCallCounter)
	cc, err := grpc.NewClient("passthrough:///unused-for-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer cc.Close()
	iv := NewClientStreamLoggerInterceptor(nil)
	_, err = iv(context.Background(), &grpc.StreamDesc{}, cc, "/svc/M",
		func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return &stubClientStream{}, nil
		})
	if err != nil {
		t.Fatalf("streamer: %v", err)
	}
	if got := counterValue(metrics.NumGRPCClientCallCounter); got != before+1 {
		t.Fatalf("client stream counter want %v got %v", before+1, got)
	}
}
