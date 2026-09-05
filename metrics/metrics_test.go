package metrics

import (
	"sync"
	"testing"
)

func TestMetricsConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_ = Get("num_http_server_access")
			_ = GetAll()
		})
	}

	wg.Wait()
}

func TestGetAllReturnsCopy(t *testing.T) {
	all := GetAll()
	n := len(all)
	delete(all, "num_http_server_access")
	if len(GetAll()) != n {
		t.Fatal("GetAll exposed internal map")
	}
}
