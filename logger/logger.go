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
 * @file logger.go
 * @package logger
 * @author Dr.NP <np@herewe.tech>
 * @since 11/27/2023
 */

package logger

import "log/slog"

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
	}

	return "unknown"
}

// Hack for log/slog
var (
	lTrace  = slog.Level(-8)
	lNotice = slog.Level(2)
	lFatal  = slog.Level(12)

	AdditionalLabels = map[slog.Level]string{
		lTrace:  "TRACE",
		lNotice: "NOTICE",
		lFatal:  "FATAL",
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
	}

	return slog.LevelInfo
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
