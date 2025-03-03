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
 * @file logger.go
 * @package logger
 * @author Dr.NP <np@herewe.tech>
 * @since 11/27/2023
 */

package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

var Logger GeneralLogger

type Level int8

const (
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel Level = iota - 2
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
	// InfoLevel is the default logging priority.
	// General operational entries about what's going on inside the application.
	InfoLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	NoticeLevel
	// NoticeLevel level. Notice messages
	WarnLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	ErrorLevel
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. highest level of severity.
	FatalLevel
	// SilenceLevel level. Logs nothing.
	SilenceLevel
)

func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case NoticeLevel:
		return "notice"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case SilenceLevel:
		return "silence"
	}

	return "unknown"
}

func LogLevel(l string) Level {
	switch strings.ToLower(l) {
	case "trace":
		return TraceLevel
	case "debug":
		return DebugLevel
	case "notice":
		return NoticeLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "fatal":
		return FatalLevel
	default:
		return InfoLevel
	}
}

// Hack for log/slog
var (
	lTrace   = slog.Level(-8)
	lNotice  = slog.Level(2)
	lFatal   = slog.Level(12)
	lSilence = slog.Level(24)

	AdditionalLabels = map[slog.Level]string{
		lTrace:   "TRACE",
		lNotice:  "NOTICE",
		lFatal:   "FATAL",
		lSilence: "SILENCE",
	}
)

func level2slog(level Level) slog.Level {
	switch level {
	case TraceLevel:
		return lTrace
	case DebugLevel:
		return slog.LevelDebug
	case InfoLevel:
		return slog.LevelInfo
	case NoticeLevel:
		return lNotice
	case WarnLevel:
		return slog.LevelWarn
	case ErrorLevel:
		return slog.LevelError
	case FatalLevel:
		return lFatal
	case SilenceLevel:
		return lSilence
	}

	return slog.LevelInfo
}

/* {{{ [Default logger operation ] */
// Helpers
func Trace(msg string, args ...any) {
	Logger.Log(TraceLevel, msg, args...)
}

func Tracef(format string, args ...any) {
	Logger.Logf(TraceLevel, format, args...)
}

func Debug(msg string, args ...any) {
	Logger.Log(DebugLevel, msg, args...)
}

func Debugf(format string, args ...any) {
	Logger.Logf(DebugLevel, format, args...)
}

func Info(msg string, args ...any) {
	Logger.Log(InfoLevel, msg, args...)
}

func Infof(format string, args ...any) {
	Logger.Logf(InfoLevel, format, args...)
}

func Notice(msg string, args ...any) {
	Logger.Log(NoticeLevel, msg, args...)
}

func Noticef(format string, args ...any) {
	Logger.Logf(NoticeLevel, format, args...)
}

func Warn(msg string, args ...any) {
	Logger.Log(WarnLevel, msg, args...)
}

func Warnf(format string, args ...any) {
	Logger.Logf(WarnLevel, format, args...)
}

func Error(msg string, args ...any) {
	Logger.Log(ErrorLevel, msg, args...)
}

func Errorf(format string, args ...any) {
	Logger.Logf(ErrorLevel, format, args...)
}

func Fatal(msg string, args ...any) {
	Logger.Log(FatalLevel, msg, args...)
	os.Exit(-1)
}

func Fatalf(format string, args ...any) {
	Logger.Logf(FatalLevel, format, args...)
	os.Exit(-1)
}

// Helpers with context
func TraceContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, TraceLevel, msg, args...)
}

func TracefContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, TraceLevel, format, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, DebugLevel, msg, args...)
}

func DebugfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, DebugLevel, format, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, InfoLevel, msg, args...)
}

func InfofContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, InfoLevel, format, args...)
}

func NoticeContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, NoticeLevel, msg, args...)
}

func NoticefContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, NoticeLevel, format, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, WarnLevel, msg, args...)
}

func WarnfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, WarnLevel, format, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, ErrorLevel, msg, args...)
}

func ErrorfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, ErrorLevel, format, args...)
}

func FatalContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, FatalLevel, msg, args...)
	os.Exit(-1)
}

func FatalfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, FatalLevel, format, args...)
	os.Exit(-1)
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
