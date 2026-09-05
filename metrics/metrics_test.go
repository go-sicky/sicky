package metrics

import (
	"errors"
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestMetricsConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_ = Get("sicky_server_requests_total")
			_ = GetAll()
		})
	}

	wg.Wait()
}

func TestGetAllReturnsCopy(t *testing.T) {
	all := GetAll()
	n := len(all)
	delete(all, "sicky_server_requests_total")
	if len(GetAll()) != n {
		t.Fatal("GetAll exposed internal map")
	}
}

func TestVecChildrenWritable(t *testing.T) {
	ServerRequestsTotal.WithLabelValues("grpc", "/svc/M", "/svc/M", "0").Inc()
	ServerRequestDuration.WithLabelValues("grpc", "/svc/M", "/svc/M").Observe(0.001)
	ClientRequestsTotal.WithLabelValues("grpc", "/svc/M", "host", "0").Inc()
	InfraOpsTotal.WithLabelValues("redis", "get", "ok").Inc()

	m := &dto.Metric{}
	if err := ServerRequestsTotal.WithLabelValues("grpc", "/svc/M", "/svc/M", "0").Write(m); err != nil {
		t.Fatalf("write vec child: %v", err)
	}

	if m.GetCounter().GetValue() < 1 {
		t.Fatal("vec child counter must be >= 1")
	}
}

func TestObserveHelpers(t *testing.T) {
	ObserveInfraOp("bun", "query", time.Now(), nil)
	ObserveServerRequest("http", "GET", "/route", "200", time.Millisecond)
	ObserveClientRequest("http", "GET", "example", "200", time.Millisecond)
	ObserveBrokerPublish("nats", "topic", time.Now(), nil)
	ObserveBrokerHandler("nats", "topic", "ok", time.Millisecond)
	ObserveJobRun("cron", "task", "ok", time.Millisecond)
	ObserveRegistryOp("local", "register", time.Now(), nil)
	SetInfraUp("redis", true)
	CountInfraInit("redis", nil)

	if ResultOf(nil) != "ok" || ResultOf(errors.New("boom")) != "error" {
		t.Fatal("ResultOf mapping broken")
	}
}
