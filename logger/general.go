/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
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

type GeneralLogger interface {
	// String returns the name of logger
	String() string
	// Level set log level
	Level(Level)
	// Log writes log entry
	Log(Level, string, ...any)
	// Logf writes formatted log entry
	Logf(Level, string, ...any)
	// LogContext writes log entry with context
	LogContext(context.Context, Level, string, ...any)
	// LogfContext writes formatted log entry with context
	LogfContext(context.Context, Level, string, ...any)

	// Helpers
	Trace(string, ...any)
	Tracef(string, ...any)
	Debug(string, ...any)
	Debugf(string, ...any)
	Info(string, ...any)
	Infof(string, ...any)
	Notice(string, ...any)
	Noticef(string, ...any)
	Warn(string, ...any)
	Warnf(string, ...any)
	Error(string, ...any)
	Errorf(string, ...any)
	Fatal(string, ...any)
	Fatalf(string, ...any)

	// Helpers with context
	TraceContext(context.Context, string, ...any)
	TracefContext(context.Context, string, ...any)
	DebugContext(context.Context, string, ...any)
	DebugfContext(context.Context, string, ...any)
	InfoContext(context.Context, string, ...any)
	InfofContext(context.Context, string, ...any)
	NoticeContext(context.Context, string, ...any)
	NoticefContext(context.Context, string, ...any)
	WarnContext(context.Context, string, ...any)
	WarnfContext(context.Context, string, ...any)
	ErrorContext(context.Context, string, ...any)
	ErrorfContext(context.Context, string, ...any)
	FatalContext(context.Context, string, ...any)
	FatalfContext(context.Context, string, ...any)
}

var DefaultGeneralLogger GeneralLogger

func init() {
	SetDefaultGeneral(NewGeneral(nil))
}

func SetDefaultGeneral(logger GeneralLogger) {
	Logger = logger
	DefaultGeneralLogger = logger
}

type generalLogger struct {
	ins   *slog.Logger
	level *slog.LevelVar
}

func NewGeneral(l ...*slog.Logger) GeneralLogger {
	var ins *slog.Logger
	var level = new(slog.LevelVar)
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
							level := a.Value.Any().(slog.Level)
							levelLabel, exists := AdditionalLabels[level]
							if exists {
								a.Value = slog.StringValue(levelLabel)
							}
						}

						return a
					},
				},
			),
		)

		slog.SetDefault(ins)
	}

	gl := &generalLogger{
		ins:   ins,
		level: level,
	}

	return gl
}

func (gl *generalLogger) String() string {
	return "general_logger"
}

func (gl *generalLogger) Level(level Level) {
	gl.level.Set(level2slog(level))
}

func (gl *generalLogger) Log(level Level, msg string, args ...any) {
	gl.ins.Log(context.TODO(), level2slog(level), msg, args...)
}

func (gl *generalLogger) Logf(level Level, format string, args ...any) {
	gl.ins.Log(context.TODO(), level2slog(level), fmt.Sprintf(format, args...))
}

func (gl *generalLogger) LogContext(ctx context.Context, level Level, msg string, args ...any) {
	gl.ins.Log(ctx, level2slog(level), msg, args...)
}

func (gl *generalLogger) LogfContext(ctx context.Context, level Level, format string, args ...any) {
	gl.ins.Log(ctx, level2slog(level), fmt.Sprintf(format, args...))
}

// Helpers
func (gl *generalLogger) Trace(msg string, args ...any) {
	gl.Log(TraceLevel, msg, args...)
}

func (gl *generalLogger) Tracef(format string, args ...any) {
	gl.Logf(TraceLevel, format, args...)
}

func (gl *generalLogger) Debug(msg string, args ...any) {
	gl.Log(DebugLevel, msg, args...)
}

func (gl *generalLogger) Debugf(format string, args ...any) {
	gl.Logf(DebugLevel, format, args...)
}

func (gl *generalLogger) Info(msg string, args ...any) {
	gl.Log(InfoLevel, msg, args...)
}

func (gl *generalLogger) Infof(format string, args ...any) {
	gl.Logf(InfoLevel, format, args...)
}

func (gl *generalLogger) Notice(msg string, args ...any) {
	gl.Log(NoticeLevel, msg, args...)
}

func (gl *generalLogger) Noticef(format string, args ...any) {
	gl.Logf(NoticeLevel, format, args...)
}

func (gl *generalLogger) Warn(msg string, args ...any) {
	gl.Log(WarnLevel, msg, args...)
}

func (gl *generalLogger) Warnf(format string, args ...any) {
	gl.Logf(WarnLevel, format, args...)
}

func (gl *generalLogger) Error(msg string, args ...any) {
	gl.Log(ErrorLevel, msg, args...)
}

func (gl *generalLogger) Errorf(format string, args ...any) {
	gl.Logf(ErrorLevel, format, args...)
}

func (gl *generalLogger) Fatal(msg string, args ...any) {
	gl.Log(FatalLevel, msg, args...)
	os.Exit(-1)
}

func (gl *generalLogger) Fatalf(format string, args ...any) {
	gl.Logf(FatalLevel, format, args...)
	os.Exit(-1)
}

// Helpers with context
func (gl *generalLogger) TraceContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, TraceLevel, msg, args...)
}

func (gl *generalLogger) TracefContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, TraceLevel, format, args...)
}

func (gl *generalLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, DebugLevel, msg, args...)
}

func (gl *generalLogger) DebugfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, DebugLevel, format, args...)
}

func (gl *generalLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, InfoLevel, msg, args...)
}

func (gl *generalLogger) InfofContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, InfoLevel, format, args...)
}

func (gl *generalLogger) NoticeContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, NoticeLevel, msg, args...)
}

func (gl *generalLogger) NoticefContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, NoticeLevel, format, args...)
}

func (gl *generalLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, WarnLevel, msg, args...)
}

func (gl *generalLogger) WarnfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, WarnLevel, format, args...)
}

func (gl *generalLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, ErrorLevel, msg, args...)
}

func (gl *generalLogger) ErrorfContext(ctx context.Context, format string, args ...any) {
	gl.LogfContext(ctx, ErrorLevel, format, args...)
}

func (gl *generalLogger) FatalContext(ctx context.Context, msg string, args ...any) {
	gl.LogContext(ctx, FatalLevel, msg, args...)
	os.Exit(-1)
}

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
