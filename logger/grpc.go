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
 * @file grpc.go
 * @package logger
 * @author Dr.NP <np@herewe.tech>
 * @since 11/26/2023
 */

package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// GRPCLogger is a logger component.
type GRPCLogger interface {
	// String returns the name of logger
	String() string
	// Level set log level
	Level(level Level)

	Info(args ...any)
	Infoln(args ...any)
	Infof(msg string, args ...any)
	Warning(args ...any)
	Warningln(args ...any)
	Warningf(msg string, args ...any)
	Error(args ...any)
	Errorln(args ...any)
	Errorf(msg string, args ...any)
	Fatal(args ...any)
	Fatalln(args ...any)
	Fatalf(msg string, args ...any)
	V(level int) bool
}

// DefaultGRPCLogger is a shared logger value.
var DefaultGRPCLogger = NewGRPC(nil)

// SetDefaultGRPC sets defaultgrpc.
func SetDefaultGRPC(logger GRPCLogger) {
	DefaultGRPCLogger = logger
}

type grpcLogger struct {
	ins   *slog.Logger
	level *slog.LevelVar
}

// NewGRPC creates a new GRPC.
func NewGRPC(l ...*slog.Logger) GRPCLogger {
	var ins *slog.Logger
	level := new(slog.LevelVar)
	if len(l) > 0 {
		ins = l[0]
	} else {
		ins = slog.Default()
	}

	if ins == nil {
		// Generate default slog.Logger
		ins = slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{
					AddSource: true,
					Level:     level,
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.LevelKey {
							if level, ok := a.Value.Any().(slog.Level); ok {
								levelLabel, exists := AdditionalLabels[level]
								if exists {
									a.Value = slog.StringValue(levelLabel)
								}
							}
						}

						return a
					},
				},
			),
		)
	}

	gl := &grpcLogger{
		ins:   ins,
		level: level,
	}

	return gl
}

// String returns a human-readable name.
func (gl *grpcLogger) String() string {
	return "grpc_logger"
}

// Level returns the log level.
func (gl *grpcLogger) Level(level Level) {
	gl.level.Set(level2slog(level))
}

// Info logs at info level.
func (gl *grpcLogger) Info(args ...any) {
	gl.ins.Info(fmt.Sprint(args...))
}

// Infoln is part of the public API.
func (gl *grpcLogger) Infoln(args ...any) {
	gl.ins.Info(fmt.Sprintln(args...))
}

// Infof logs at info level.
func (gl *grpcLogger) Infof(format string, args ...any) {
	gl.ins.Info(fmt.Sprintf(format, args...))
}

// Warning is part of the public API.
func (gl *grpcLogger) Warning(args ...any) {
	gl.ins.Warn(fmt.Sprint(args...))
}

// Warningln is part of the public API.
func (gl *grpcLogger) Warningln(args ...any) {
	gl.ins.Warn(fmt.Sprintln(args...))
}

// Warningf is part of the public API.
func (gl *grpcLogger) Warningf(format string, args ...any) {
	gl.ins.Warn(fmt.Sprintf(format, args...))
}

// Error returns the error string.
func (gl *grpcLogger) Error(args ...any) {
	gl.ins.Error(fmt.Sprint(args...))
}

// Errorln is part of the public API.
func (gl *grpcLogger) Errorln(args ...any) {
	gl.ins.Error(fmt.Sprintln(args...))
}

// Errorf logs at error level.
func (gl *grpcLogger) Errorf(format string, args ...any) {
	gl.ins.Error(fmt.Sprintf(format, args...))
}

// Fatal logs at fatal level and exits.
func (gl *grpcLogger) Fatal(args ...any) {
	gl.ins.Log(context.Background(), level2slog(FatalLevel), fmt.Sprint(args...))
	os.Exit(-1)
}

// Fatalln is part of the public API.
func (gl *grpcLogger) Fatalln(args ...any) {
	gl.ins.Log(context.Background(), level2slog(FatalLevel), fmt.Sprintln(args...))
	os.Exit(-1)
}

// Fatalf logs at fatal level and exits.
func (gl *grpcLogger) Fatalf(format string, args ...any) {
	gl.ins.Log(context.Background(), level2slog(FatalLevel), fmt.Sprintf(format, args...))
	os.Exit(-1)
}

// V is part of the public API.
func (gl *grpcLogger) V(l int) bool {
	// Always verbose
	return true
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
