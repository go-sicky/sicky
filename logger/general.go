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
 * @file general.go
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

// GeneralLogger is a logger component.
type GeneralLogger interface {
	// String returns the name of logger
	String() string
	// Level set log level
	Level(level Level)
	// Log writes log entry
	Log(level Level, msg string, args ...any)
	// Logf writes formatted log entry
	Logf(level Level, msg string, args ...any)
	// LogContext writes log entry with context
	LogContext(ctx context.Context, level Level, msg string, args ...any)
	// LogfContext writes formatted log entry with context
	LogfContext(ctx context.Context, level Level, msg string, args ...any)

	// Helpers
	Trace(msg string, args ...any)
	Tracef(msg string, args ...any)
	Debug(msg string, args ...any)
	Debugf(msg string, args ...any)
	Info(msg string, args ...any)
	Infof(msg string, args ...any)
	Notice(msg string, args ...any)
	Noticef(msg string, args ...any)
	Warn(msg string, args ...any)
	Warnf(msg string, args ...any)
	Error(msg string, args ...any)
	Errorf(msg string, args ...any)
	Fatal(msg string, args ...any)
	Fatalf(msg string, args ...any)

	// Helpers with context
	TraceContext(ctx context.Context, msg string, args ...any)
	TracefContext(ctx context.Context, msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
	DebugfContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	InfofContext(ctx context.Context, msg string, args ...any)
	NoticeContext(ctx context.Context, msg string, args ...any)
	NoticefContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	WarnfContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	ErrorfContext(ctx context.Context, msg string, args ...any)
	FatalContext(ctx context.Context, msg string, args ...any)
	FatalfContext(ctx context.Context, msg string, args ...any)
}

// DefaultGeneralLogger is a shared logger value.
var DefaultGeneralLogger = NewGeneral(nil)

// SetDefaultGeneral sets defaultgeneral.
func SetDefaultGeneral(logger GeneralLogger) {
	Logger = logger
	DefaultGeneralLogger = logger
}

type generalLogger struct {
	ins   *slog.Logger
	level *slog.LevelVar
}

// NewGeneral creates a new General.
func NewGeneral(l ...*slog.Logger) GeneralLogger {
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
					AddSource: false,
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

	slog.SetDefault(ins)
	gl := &generalLogger{
		ins:   ins,
		level: level,
	}

	if Logger == nil {
		Logger = gl
	}

	return gl
}

// String returns a human-readable name.
func (gl *generalLogger) String() string {
	return "general_logger"
}

// Level returns the log level.
func (gl *generalLogger) Level(level Level) {
	gl.level.Set(level2slog(level))
}

// Log is part of the public API.
func (gl *generalLogger) Log(level Level, msg string, args ...any) {
	gl.ins.Log(context.Background(), level2slog(level), msg, args...)
}

// Logf is part of the public API.
func (gl *generalLogger) Logf(level Level, format string, args ...any) {
	gl.ins.Log(context.Background(), level2slog(level), fmt.Sprintf(format, args...))
}

// LogContext is part of the public API.
func (gl *generalLogger) LogContext(ctx context.Context, level Level, msg string, args ...any) {
	gl.ins.Log(ctx, level2slog(level), msg, args...)
}

// LogfContext is part of the public API.
func (gl *generalLogger) LogfContext(ctx context.Context, level Level, format string, args ...any) {
	gl.ins.Log(ctx, level2slog(level), fmt.Sprintf(format, args...))
}

// Helpers.
func (gl *generalLogger) Trace(msg string, args ...any) {
	gl.Log(TraceLevel, msg, args...)
}

// Tracef logs at trace level.
func (gl *generalLogger) Tracef(format string, args ...any) {
	gl.Logf(TraceLevel, format, args...)
}

// Debug logs at debug level.
func (gl *generalLogger) Debug(msg string, args ...any) {
	gl.Log(DebugLevel, msg, args...)
}

// Debugf logs at debug level.
func (gl *generalLogger) Debugf(format string, args ...any) {
	gl.Logf(DebugLevel, format, args...)
}

// Info logs at info level.
func (gl *generalLogger) Info(msg string, args ...any) {
	gl.Log(InfoLevel, msg, args...)
}

// Infof logs at info level.
func (gl *generalLogger) Infof(format string, args ...any) {
	gl.Logf(InfoLevel, format, args...)
}

// Notice logs at notice level.
func (gl *generalLogger) Notice(msg string, args ...any) {
	gl.Log(NoticeLevel, msg, args...)
}

// Noticef logs at notice level.
func (gl *generalLogger) Noticef(format string, args ...any) {
	gl.Logf(NoticeLevel, format, args...)
}

// Warn logs at warn level.
func (gl *generalLogger) Warn(msg string, args ...any) {
	gl.Log(WarnLevel, msg, args...)
}

// Warnf logs at warn level.
func (gl *generalLogger) Warnf(format string, args ...any) {
	gl.Logf(WarnLevel, format, args...)
}

// Error returns the error string.
func (gl *generalLogger) Error(msg string, args ...any) {
	gl.Log(ErrorLevel, msg, args...)
}

// Errorf logs at error level.
func (gl *generalLogger) Errorf(format string, args ...any) {
	gl.Logf(ErrorLevel, format, args...)
}

// Fatal logs at fatal level and exits.
func (gl *generalLogger) Fatal(msg string, args ...any) {
	gl.Log(FatalLevel, msg, args...)
	os.Exit(-1)
}

// Fatalf logs at fatal level and exits.
func (gl *generalLogger) Fatalf(format string, args ...any) {
	gl.Logf(FatalLevel, format, args...)
	os.Exit(-1)
}

// Helpers with context.
func (gl *generalLogger) TraceContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, TraceLevel, msg, args...)
}

// TracefContext logs at trace level.
func (gl *generalLogger) TracefContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, TraceLevel, format, args...)
}

// DebugContext logs at debug level.
func (gl *generalLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, DebugLevel, msg, args...)
}

// DebugfContext logs at debug level.
func (gl *generalLogger) DebugfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, DebugLevel, format, args...)
}

// InfoContext logs at info level.
func (gl *generalLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, InfoLevel, msg, args...)
}

// InfofContext logs at info level.
func (gl *generalLogger) InfofContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, InfoLevel, format, args...)
}

// NoticeContext logs at notice level.
func (gl *generalLogger) NoticeContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, NoticeLevel, msg, args...)
}

// NoticefContext logs at notice level.
func (gl *generalLogger) NoticefContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, NoticeLevel, format, args...)
}

// WarnContext logs at warn level.
func (gl *generalLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, WarnLevel, msg, args...)
}

// WarnfContext logs at warn level.
func (gl *generalLogger) WarnfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, WarnLevel, format, args...)
}

// ErrorContext logs at error level.
func (gl *generalLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, ErrorLevel, msg, args...)
}

// ErrorfContext logs at error level.
func (gl *generalLogger) ErrorfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, ErrorLevel, format, args...)
}

// FatalContext logs at fatal level and exits.
func (gl *generalLogger) FatalContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, FatalLevel, msg, args...)
	os.Exit(-1)
}

// FatalfContext logs at fatal level and exits.
func (gl *generalLogger) FatalfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, FatalLevel, format, args...)
	os.Exit(-1)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
