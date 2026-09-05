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

// Logger is a shared logger value.
var Logger GeneralLogger

// Level is a logger component.
type Level int8

const (
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	// TraceLevel is a logger constant.
	TraceLevel Level = iota - 2
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	// DebugLevel is a logger constant.
	DebugLevel
	// InfoLevel is the default logging priority.
	// General operational entries about what's going on inside the application.
	// InfoLevel is a logger constant.
	InfoLevel
	// NoticeLevel level. Notice messages
	// NoticeLevel is a logger constant.
	NoticeLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	// WarnLevel is a logger constant.
	WarnLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// ErrorLevel is a logger constant.
	ErrorLevel
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. highest level of severity.
	// FatalLevel is a logger constant.
	FatalLevel
	// SilenceLevel level. Logs nothing.
	// SilenceLevel is a logger constant.
	SilenceLevel
)

// String returns a human-readable name.
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

// LogLevel is part of the public API.
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
	case "silence":
		return SilenceLevel
	default:
		return InfoLevel
	}
}

// Hack for log/slog.
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
// Helpers.
func Trace(msg string, args ...any) {
	Logger.Log(TraceLevel, msg, args...)
}

// Tracef logs at trace level.
func Tracef(format string, args ...any) {
	Logger.Logf(TraceLevel, format, args...)
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	Logger.Log(DebugLevel, msg, args...)
}

// Debugf logs at debug level.
func Debugf(format string, args ...any) {
	Logger.Logf(DebugLevel, format, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	Logger.Log(InfoLevel, msg, args...)
}

// Infof logs at info level.
func Infof(format string, args ...any) {
	Logger.Logf(InfoLevel, format, args...)
}

// Notice logs at notice level.
func Notice(msg string, args ...any) {
	Logger.Log(NoticeLevel, msg, args...)
}

// Noticef logs at notice level.
func Noticef(format string, args ...any) {
	Logger.Logf(NoticeLevel, format, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	Logger.Log(WarnLevel, msg, args...)
}

// Warnf logs at warn level.
func Warnf(format string, args ...any) {
	Logger.Logf(WarnLevel, format, args...)
}

// Error returns the error string.
func Error(msg string, args ...any) {
	Logger.Log(ErrorLevel, msg, args...)
}

// Errorf logs at error level.
func Errorf(format string, args ...any) {
	Logger.Logf(ErrorLevel, format, args...)
}

// Fatal logs at fatal level and exits.
func Fatal(msg string, args ...any) {
	Logger.Log(FatalLevel, msg, args...)
	os.Exit(-1)
}

// Fatalf logs at fatal level and exits.
func Fatalf(format string, args ...any) {
	Logger.Logf(FatalLevel, format, args...)
	os.Exit(-1)
}

// Helpers with context.
func TraceContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, TraceLevel, msg, args...)
}

// TracefContext logs at trace level.
func TracefContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, TraceLevel, format, args...)
}

// DebugContext logs at debug level.
func DebugContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, DebugLevel, msg, args...)
}

// DebugfContext logs at debug level.
func DebugfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, DebugLevel, format, args...)
}

// InfoContext logs at info level.
func InfoContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, InfoLevel, msg, args...)
}

// InfofContext logs at info level.
func InfofContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, InfoLevel, format, args...)
}

// NoticeContext logs at notice level.
func NoticeContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, NoticeLevel, msg, args...)
}

// NoticefContext logs at notice level.
func NoticefContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, NoticeLevel, format, args...)
}

// WarnContext logs at warn level.
func WarnContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, WarnLevel, msg, args...)
}

// WarnfContext logs at warn level.
func WarnfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, WarnLevel, format, args...)
}

// ErrorContext logs at error level.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, ErrorLevel, msg, args...)
}

// ErrorfContext logs at error level.
func ErrorfContext(ctx context.Context, format string, args ...any) {
	Logger.LogfContext(ctx, ErrorLevel, format, args...)
}

// FatalContext logs at fatal level and exits.
func FatalContext(ctx context.Context, msg string, args ...any) {
	Logger.LogContext(ctx, FatalLevel, msg, args...)
	os.Exit(-1)
}

// FatalfContext logs at fatal level and exits.
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
