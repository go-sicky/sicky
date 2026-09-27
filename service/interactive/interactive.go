/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file interactive.go
 * @package interactive
 * @author Dr.NP <np@herewe.tech>
 * @since 08/13/2024
 */

package interactive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

// interactResult tells the stdin loop what to do after one turn.
type interactResult int

const (
	// interactContinue keeps reading (a command was handled).
	interactContinue interactResult = iota
	// interactQuit runs on the stop command: shut the process down.
	interactQuit
	// interactDone ends the loop without shutting down (stdin closed or
	// unreadable). Retry on error would spin the loop at 100% CPU.
	interactDone
)

// Interactive is a interactive component.
type Interactive struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	// mu guards every slice below: the getters return copies so a
	// registration from another goroutine never races a Start() walk.
	mu sync.Mutex

	servers    []server.Server
	brokers    []broker.Broker
	jobs       []job.Job
	registries []registry.Registry
	startOnce  sync.Once
	stopOnce   sync.Once
	done       chan struct{}
	stdin      io.Reader
	reader     *bufio.Reader
	tracers    []tracer.Tracer
	handlers   []Handler
}

// New creates a new instance (nil on invalid config).
func New(opts *service.Options, cfg *Config) *Interactive {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &Interactive{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
		done:    make(chan struct{}),
		stdin:   os.Stdin,
	}
	// One reader for the lifetime of the service: rebuilding it per turn
	// would discard bytes already pulled into the buffer, so piped input
	// ("printf 'a\nb\n'") would lose every line after the first.
	svc.reader = bufio.NewReader(svc.stdin)

	svc.options.Logger.InfoContext(
		svc.ctx,
		"service created",
		"service", svc.String(),
		"id", svc.options.ID,
		"name", svc.options.Name,
		"version", svc.options.Version,
		"branch", svc.options.Branch,
	)

	service.Set(svc)

	return svc
}

// Context returns the component context.
func (s *Interactive) Context() context.Context {
	return s.ctx
}

// Options returns the runtime options.
func (s *Interactive) Options() *service.Options {
	return s.options
}

// String returns a human-readable name.
func (s *Interactive) String() string {
	return "interactive"
}

// Start starts the component.
func (s *Interactive) Start() []error {
	// err  error
	var errs []error

	if s.config.StartupInfo != "" {
		fmt.Println(s.config.StartupInfo)
	}

	// One stdin-reading goroutine per service: repeated Start calls must
	// not pile up concurrent os.Stdin readers.
	s.startOnce.Do(func() {
		go s.loop()
	})

	return errs
}

// Stop stops the component and releases resources.
//
// It stops the stdin loop from starting another read, but it cannot
// cancel a read already blocked in os.Stdin - Go offers no way to
// interrupt it short of closing the file descriptor, which would also
// break every other user of stdin in the process. Such a goroutine ends
// when stdin itself produces input or reaches EOF.
func (s *Interactive) Stop() []error {
	// err  error
	var errs []error

	s.stopOnce.Do(func() {
		if s.done != nil {
			close(s.done)
		}
	})

	return errs
}

// loop drives interact until the stop command, a closed stdin or Stop.
// A reader already blocked inside os.Stdin cannot be canceled, but the
// loop never starts another read once done is closed.
func (s *Interactive) loop() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		switch s.interact() {
		case interactContinue:
			// A command was handled: keep reading.
		case interactQuit:
			fmt.Println()
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGQUIT)

			return
		case interactDone:
			// EOF or a stdin read failure: end quietly instead of
			// re-prompting forever (the process may still be serving).
			s.options.Logger.InfoContext(
				s.ctx,
				"stdin closed, interaction stopped",
				"service", s.String(),
				"id", s.options.ID,
				"name", s.options.Name,
			)

			return
		}
	}
}

// Servers returns the managed servers.
func (s *Interactive) Servers(srvs ...server.Server) []server.Server {
	s.mu.Lock()
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	out := slices.Clone(s.servers)
	s.mu.Unlock()

	return out
}

// Brokers returns a copy of the broker registry.
func (s *Interactive) Brokers(brks ...broker.Broker) []broker.Broker {
	s.mu.Lock()
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	out := slices.Clone(s.brokers)
	s.mu.Unlock()

	return out
}

// Jobs returns a copy of the job registry.
func (s *Interactive) Jobs(jobs ...job.Job) []job.Job {
	s.mu.Lock()
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	out := slices.Clone(s.jobs)
	s.mu.Unlock()

	return out
}

// Registries returns the managed registries.
func (s *Interactive) Registries(rgs ...registry.Registry) []registry.Registry {
	s.mu.Lock()
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	out := slices.Clone(s.registries)
	s.mu.Unlock()

	return out
}

// Tracers returns the managed tracers.
func (s *Interactive) Tracers(trs ...tracer.Tracer) []tracer.Tracer {
	s.mu.Lock()
	if !s.config.DisableTracing && len(trs) > 0 {
		s.tracers = append(s.tracers, trs...)
	}

	out := slices.Clone(s.tracers)
	s.mu.Unlock()

	return out
}

// snapshotHandlers returns a copy for iteration: Handle appends from
// other goroutines while the stdin loop reads.
func (s *Interactive) snapshotHandlers() []Handler {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.handlers)
}

// Handle registers handlers.
func (s *Interactive) Handle(hdls ...Handler) {
	s.mu.Lock()
	s.handlers = append(s.handlers, hdls...)
	s.mu.Unlock()

	for _, hdl := range hdls {
		s.options.Logger.InfoContext(
			s.ctx,
			"interaction handler registered",
			"service", s.String(),
			"id", s.options.ID,
			"name", s.options.Name,
			"handler", hdl.Name(),
		)
	}
}

func (s *Interactive) interact() interactResult {
	var p *color.Color

	switch strings.ToLower(s.config.PromptColor) {
	case "green":
		p = color.New(color.Bold, color.FgGreen)
	case "yellow":
		p = color.New(color.Bold, color.FgYellow)
	case "cyan":
		p = color.New(color.Bold, color.FgCyan)
	case "red":
		p = color.New(color.Bold, color.FgRed)
	case "blue":
		p = color.New(color.Bold, color.FgBlue)
	default:
		p = color.New(color.Bold, color.FgWhite)
	}

	// Stop() (or a shutdown elsewhere) must not print another prompt.
	select {
	case <-s.done:
		return interactDone
	default:
	}

	if s.reader == nil {
		// Nothing to read from: report it once instead of spinning.
		return interactDone
	}

	p.PrintFunc()(s.config.Prompt)
	line, err := s.reader.ReadString('\n')
	if line != "" {
		// A final line without a trailing newline still counts: dropping
		// it would lose the last command piped into the process.
		if s.dispatch(line) {
			return interactQuit
		}
	}

	if err != nil {
		// io.EOF means stdin is closed (`sicky serve < /dev/null`, a
		// finished pipe, Ctrl+D). Returning "keep going" here re-enters
		// immediately and burns a core on prompt output.
		return interactDone
	}

	fmt.Println()

	return interactContinue
}

// dispatch runs one command line against the handlers and reports
// whether the stop command was entered.
func (s *Interactive) dispatch(line string) bool {
	cmd := strings.TrimSpace(line)
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) > 0 {
		if parts[0] == s.config.StopCommand {
			// Quit
			for _, hdl := range s.snapshotHandlers() {
				err := hdl.OnStop()
				if err != nil {
					fmt.Println("Error : ", err.Error())
				}
			}

			return true
		}

		// Normal
		for _, hdl := range s.snapshotHandlers() {
			err := hdl.OnInteract(parts[0], cmd)
			if err != nil {
				fmt.Println("Error : ", err.Error())
			}
		}
	} // Or do nothing

	return false
}

/* {{{[Command handler]. */
type Handler interface {
	Name() string
	OnInteract(cmd, full string) error
	OnStop() error
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
