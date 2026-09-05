package grpc

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnaryRecoveryInterceptor(t *testing.T) {
	iv := NewRecoveryInterceptor()
	_, err := iv(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/svc/Panic"},
		func(ctx context.Context, req any) (any, error) {
			panic("secret-credential-boom")
		})
	if err == nil {
		t.Fatal("panic must convert to error")
	}

	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("code = %v, want Internal", got)
	}

	// Neither the panic value nor the stack trace may reach the client.
	msg := status.Convert(err).Message()
	if strings.Contains(msg, "secret-credential-boom") {
		t.Fatalf("panic value leaked to client: %q", msg)
	}

	if strings.Contains(msg, "goroutine") || strings.Contains(msg, "recovery.go") {
		t.Fatalf("stack trace leaked to client: %q", msg)
	}

	// Non-panicking handler passes through untouched.
	resp, err := iv(context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/OK"},
		func(ctx context.Context, req any) (any, error) { return "resp", nil })
	if err != nil || resp != "resp" {
		t.Fatalf("passthrough broken: resp=%v err=%v", resp, err)
	}
}

func TestStreamRecoveryInterceptor(t *testing.T) {
	iv := NewStreamRecoveryInterceptor()
	err := iv(nil, &stubServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/svc/Panic"},
		func(srv any, ss grpc.ServerStream) error {
			panic("secret-credential-boom")
		})
	if err == nil {
		t.Fatal("panic must convert to error")
	}

	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("code = %v, want Internal", got)
	}

	msg := status.Convert(err).Message()
	if strings.Contains(msg, "secret-credential-boom") {
		t.Fatalf("panic value leaked to client: %q", msg)
	}

	if strings.Contains(msg, "goroutine") || strings.Contains(msg, "recovery.go") {
		t.Fatalf("stack trace leaked to client: %q", msg)
	}
}
