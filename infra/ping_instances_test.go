/**
 * @file ping_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/elastic/go-elasticsearch/v9"
)

// PingS3/PingElastic must resolve a name without contacting a backend when
// the instance has no probeable state, and must report a missing name
// explicitly.
func TestPingNamedInstanceBranches(t *testing.T) {
	ClearS3All()
	ClearElasticAll()
	t.Cleanup(func() {
		ClearS3All()
		ClearElasticAll()
	})

	if err := PingS3(context.Background(), "missing"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("PingS3(missing) = %v, want ErrInstanceNotFound", err)
	}

	s3Instances.putFirst("empty", s3Instance{client: new(s3.Client)})
	if err := PingS3(context.Background(), "empty"); err != nil {
		t.Fatalf("PingS3(empty) = %v, want nil without a bucket", err)
	}

	if err := PingElastic(context.Background(), "missing"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("PingElastic(missing) = %v, want ErrInstanceNotFound", err)
	}

	elasticInstances.putFirst("empty", nil)
	if err := PingElastic(context.Background(), "empty"); err != nil {
		t.Fatalf("PingElastic(empty) = %v, want nil for a nil client", err)
	}

	// No default configured: both default probes are healthy-by-absence.
	if err := PingS3(context.Background()); err != nil {
		t.Fatalf("PingS3() = %v, want nil", err)
	}

	if err := PingElastic(context.Background()); err != nil {
		t.Fatalf("PingElastic() = %v, want nil", err)
	}
}

func TestPingNamedInstanceAgainstFakeBackends(t *testing.T) {
	ClearS3All()
	ClearElasticAll()
	t.Cleanup(func() {
		ClearS3All()
		ClearElasticAll()
	})

	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer func() {
		s3Server.CloseClientConnections()
		s3Server.Close()
	}()

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("key", "secret", "")),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		endpoint := s3Server.URL
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
	s3Instances.putFirst("gallery", s3Instance{client: client, bucket: "gallery"})

	if err := PingS3(context.Background(), "gallery"); err != nil {
		t.Fatalf("PingS3(gallery) = %v, want nil against the fake bucket", err)
	}

	elasticServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"fake","version":{"number":"9.0.0"}}`))
	}))
	defer func() {
		elasticServer.CloseClientConnections()
		elasticServer.Close()
	}()

	elasticClient, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{elasticServer.URL}})
	if err != nil {
		t.Fatalf("new elastic client: %v", err)
	}

	elasticInstances.putFirst("search", elasticClient)

	if err := PingElastic(context.Background(), "search"); err != nil {
		t.Fatalf("PingElastic(search) = %v, want nil against the fake cluster", err)
	}

	_ = elasticClient.Close(context.Background())
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
