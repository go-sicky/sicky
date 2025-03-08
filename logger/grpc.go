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

type GRPCLogger interface {
	// String returns the name of logger
	String() string
	// Level set log level
	Level(Level)

	Info(...any)
	Infoln(...any)
	Infof(string, ...any)
	Warning(...any)
	Warningln(...any)
	Warningf(string, ...any)
	Error(...any)
	Errorln(...any)
	Errorf(string, ...any)
	Fatal(...any)
	Fatalln(...any)
	Fatalf(string, ...any)
	V(int) bool
}

var DefaultGRPCLogger = NewGRPC(nil)

func SetDefaultGRPC(logger GRPCLogger) {
	DefaultGRPCLogger = logger
}

type grpcLogger struct {
	ins   *slog.Logger
	level *slog.LevelVar
}

func NewGRPC(l ...*slog.Logger) GRPCLogger {
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
	}

	gl := &grpcLogger{
		ins:   ins,
		level: level,
	}

	return gl
}

func (gl *grpcLogger) String() string {
	return "grpc_logger"
}

func (gl *grpcLogger) Level(level Level) {
	gl.level.Set(level2slog(level))
}

func (gl *grpcLogger) Info(args ...any) {
	gl.ins.Info(fmt.Sprint(args...))
}

func (gl *grpcLogger) Infoln(args ...any) {
	gl.ins.Info(fmt.Sprintln(args...))
}

func (gl *grpcLogger) Infof(format string, args ...any) {
	gl.ins.Info(fmt.Sprintf(format, args...))
}

func (gl *grpcLogger) Warning(args ...any) {
	gl.ins.Warn(fmt.Sprint(args...))
}

func (gl *grpcLogger) Warningln(args ...any) {
	gl.ins.Warn(fmt.Sprintln(args...))
}

func (gl *grpcLogger) Warningf(format string, args ...any) {
	gl.ins.Warn(fmt.Sprintf(format, args...))
}

func (gl *grpcLogger) Error(args ...any) {
	gl.ins.Error(fmt.Sprint(args...))
}

func (gl *grpcLogger) Errorln(args ...any) {
	gl.ins.Error(fmt.Sprintln(args...))
}

func (gl *grpcLogger) Errorf(format string, args ...any) {
	gl.ins.Error(fmt.Sprintf(format, args...))
}

func (gl *grpcLogger) Fatal(args ...any) {
	gl.ins.Log(context.TODO(), level2slog(FatalLevel), fmt.Sprint(args...))
	os.Exit(-1)
}

func (gl *grpcLogger) Fatalln(args ...any) {
	gl.ins.Log(context.TODO(), level2slog(FatalLevel), fmt.Sprintln(args...))
	os.Exit(-1)
}

func (gl *grpcLogger) Fatalf(format string, args ...any) {
	gl.ins.Log(context.TODO(), level2slog(FatalLevel), fmt.Sprintf(format, args...))
	os.Exit(-1)
}

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
