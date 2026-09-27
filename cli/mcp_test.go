package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// TestConsumeSubcommandArgs: sicky.Init re-parses os.Args with the
// global flag set (ExitOnError), so `sicky serve --transport http` used
// to die with "unknown flag" and exit 2 before serving anything.
func TestConsumeSubcommandArgs(t *testing.T) {
	saved := os.Args
	defer func() {
		os.Args = saved
	}()

	os.Args = []string{"sicky", "serve", "--transport", "http", "--listen", "127.0.0.1:3000", "leftover"}

	fs := pflag.NewFlagSet("serve", pflag.ContinueOnError)
	transport := fs.String("transport", "stdio", "")
	listen := fs.String("listen", "127.0.0.1:3000", "")
	if err := fs.Parse(os.Args[2:]); err != nil {
		t.Fatalf("parse: %v", err)
	}

	consumeSubcommandArgs(fs)

	if *transport != "http" || *listen != "127.0.0.1:3000" {
		t.Fatalf("flags = %q/%q", *transport, *listen)
	}

	want := "sicky leftover"
	got := os.Args[0]
	var gotSb36 strings.Builder
	for _, a := range os.Args[1:] {
		gotSb36.WriteString(" " + a)
	}
	got += gotSb36.String()

	if got != want {
		t.Fatalf("os.Args = %q, want %q (subcommand flags must be gone)", got, want)
	}
}

// TestServeMCPListenDefaultsLoopback: without a token the transport only
// accepts loopback peers, so the default must not advertise a wildcard.
func TestServeMCPListenDefaultsLoopback(t *testing.T) {
	fs := pflag.NewFlagSet("serve", pflag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:3000", "")
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if *listen != "127.0.0.1:3000" {
		t.Fatalf("default listen = %q, want 127.0.0.1:3000", *listen)
	}
}

// TestNewHTTPTransportWiresFlags: without the flag the transport only
// accepts loopback Host headers, so a deployment behind a reverse proxy
// needs to be able to list its name.
func TestNewHTTPTransportWiresFlags(t *testing.T) {
	trans := newHTTPTransport("127.0.0.1:3000", "s3cret", " api.example.com , 10.0.0.5:3000 ,")

	if trans.AuthToken != "s3cret" {
		t.Fatalf("auth token = %q", trans.AuthToken)
	}

	if len(trans.AllowedHosts) != 2 {
		t.Fatalf("allowed hosts = %v, want two entries (blanks dropped)", trans.AllowedHosts)
	}

	if trans.AllowedHosts[0] != "api.example.com" || trans.AllowedHosts[1] != "10.0.0.5:3000" {
		t.Fatalf("allowed hosts = %v", trans.AllowedHosts)
	}
}
