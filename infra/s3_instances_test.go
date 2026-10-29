/**
 * @file s3_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestS3ConfigInstancesRequireCompleteConfig(t *testing.T) {
	cfg := &S3Config{
		Region: "us-east-1",
		Instances: map[string]*S3Config{
			"gallery": {Bucket: "gallery"},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrS3RegionEmpty) {
		t.Fatalf("Validate() = %v, want ErrS3RegionEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.gallery") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestS3ConfigRejectsNestedInstances(t *testing.T) {
	cfg := &S3Config{
		Region: "us-east-1",
		Instances: map[string]*S3Config{
			"gallery": {
				Region:    "us-east-1",
				Instances: map[string]*S3Config{"nested": {Region: "us-east-1"}},
			},
		},
	}

	if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
		t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
	}
}

func TestS3DefaultSelection(t *testing.T) {
	instances := []string{"avatars", "gallery"}

	selection, err := resolveDefaultSelection(false, "gallery", instances, true, nil)
	if err != nil {
		t.Fatalf("complete top-level: %v", err)
	}

	if !selection.UseTopLevel || selection.Ignored != "gallery" {
		t.Fatalf("complete top-level selection = %+v, want UseTopLevel and ignored gallery", selection)
	}

	selection, err = resolveDefaultSelection(false, "gallery", instances, false, ErrS3RegionEmpty)
	if err != nil {
		t.Fatalf("explicit instance: %v", err)
	}

	if selection.UseTopLevel || selection.Instance != "gallery" {
		t.Fatalf("explicit instance selection = %+v, want gallery", selection)
	}

	selection, err = resolveDefaultSelection(false, "", instances, false, ErrS3RegionEmpty)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}

	if selection.Instance != "avatars" {
		t.Fatalf("fallback selection = %+v, want the first sorted instance avatars", selection)
	}

	if _, err := resolveDefaultSelection(true, "gallery", instances, false, ErrS3RegionEmpty); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Fatalf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}

	if _, err := resolveDefaultSelection(true, "", nil, false, ErrS3RegionEmpty); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}
}

func TestInitS3NamedInstances(t *testing.T) {
	ShutdownS3()
	t.Cleanup(func() { ShutdownS3() })

	client, err := InitS3(&S3Config{
		Instances: map[string]*S3Config{
			"gallery": {Region: "us-east-1", Bucket: "gallery"},
			"avatars": {Region: "us-east-1", Bucket: "avatars"},
		},
	})
	if err != nil {
		t.Fatalf("InitS3() = %v, want nil", err)
	}

	if client == nil {
		t.Fatal("InitS3() returned nil default client")
	}

	if GetS3() != client {
		t.Fatal("GetS3() must return the same default client InitS3 returned")
	}

	// The top-level config is incomplete, so the first sorted instance is
	// the default. "avatars" sorts before "gallery".
	if GetS3("avatars") != client {
		t.Fatal("the sorted first instance must be the default")
	}

	if GetS3("gallery") == nil {
		t.Fatal("GetS3(gallery) = nil, want a client")
	}

	if err := PingS3(context.Background(), "missing"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("PingS3(missing) = %v, want ErrInstanceNotFound", err)
	}

	ClearS3("gallery")
	if GetS3("gallery") != nil {
		t.Fatal("ClearS3(gallery) left the instance registered")
	}

	if GetS3() == nil {
		t.Fatal("ClearS3(gallery) must not clear the default")
	}

	ClearS3("")
	if GetS3() != nil {
		t.Fatal("ClearS3(\"\") must clear the default reference")
	}

	if GetS3("avatars") == nil {
		t.Fatal("ClearS3(\"\") must not clear named instances")
	}
}

func TestInitS3NoDefault(t *testing.T) {
	ShutdownS3()
	t.Cleanup(func() { ShutdownS3() })

	client, err := InitS3(&S3Config{
		NoDefault: true,
		Instances: map[string]*S3Config{
			"gallery": {Region: "us-east-1", Bucket: "gallery"},
		},
	})
	if err != nil {
		t.Fatalf("InitS3() = %v, want nil", err)
	}

	if client != nil {
		t.Fatalf("InitS3() = %v, want nil with no_default", client)
	}

	if GetS3() != nil {
		t.Fatal("GetS3() must be nil with no_default")
	}

	if GetS3("gallery") == nil {
		t.Fatal("GetS3(gallery) = nil, want the named instance")
	}
}

func TestInitS3DefaultInstanceAlias(t *testing.T) {
	ShutdownS3()
	t.Cleanup(func() { ShutdownS3() })

	if _, err := InitS3(&S3Config{
		DefaultInstance: "gallery",
		Instances: map[string]*S3Config{
			"gallery": {Region: "us-east-1", Bucket: "gallery"},
			"avatars": {Region: "us-east-1", Bucket: "avatars"},
		},
	}); err != nil {
		t.Fatalf("InitS3() = %v, want nil", err)
	}

	if GetS3() == nil || GetS3() != GetS3("gallery") {
		t.Fatal("GetS3() must be the gallery alias")
	}

	if GetS3Instances()["gallery"] != GetS3() {
		t.Fatal("the named snapshot must expose the same alias client")
	}

	ClearS3("gallery")
	if GetS3() != nil {
		t.Fatal("clearing the default alias must clear the shared reference")
	}

	if GetS3("avatars") == nil {
		t.Fatal("clearing gallery must not clear avatars")
	}
}

func TestClearS3All(t *testing.T) {
	ShutdownS3()
	t.Cleanup(func() { ShutdownS3() })

	if _, err := InitS3(&S3Config{
		Region: "us-east-1",
		Instances: map[string]*S3Config{
			"gallery": {Region: "us-east-1", Bucket: "gallery"},
		},
	}); err != nil {
		t.Fatalf("InitS3() = %v, want nil", err)
	}

	ClearS3All()

	if GetS3() != nil || GetS3("gallery") != nil {
		t.Fatal("ClearS3All must clear the default and every named instance")
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
