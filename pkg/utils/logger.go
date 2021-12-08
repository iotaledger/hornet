package utils

import (
	"github.com/iotaledger/hive.go/logger"
)

// WrappedLogger is a wrapper to call logging functions in case a logger was passed.
type WrappedLogger struct {
	logger *logger.Logger
}

// NewWrappedLogger creates a new WrappedLogger.
func NewWrappedLogger(logger *logger.Logger) *WrappedLogger {
	return &WrappedLogger{logger: logger}
}

// Logger return the underlying logger.
func (l *WrappedLogger) Logger() *logger.Logger {
	if l.logger != nil {
		return l.logger
	}
	return nil
}

// LoggerNamed adds a sub-scope to the logger's name. See Logger.Named for details.
func (l *WrappedLogger) LoggerNamed(name string) *logger.Logger {
	if l.logger != nil {
		return l.logger.Named(name)
	}
	return nil
}

// LogDebug uses fmt.Sprint to construct and log a message.
func (l *WrappedLogger) LogDebug(args ...interface{}) {
	if l.logger != nil {
		l.logger.Debug(args...)
	}
}

// LogDebugf uses fmt.Sprintf to log a templated message.
func (l *WrappedLogger) LogDebugf(template string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Debugf(template, args...)
	}
}

// LogError uses fmt.Sprint to construct and log a message.
func (l *WrappedLogger) LogError(args ...interface{}) {
	if l.logger != nil {
		l.logger.Error(args...)
	}
}

// LogErrorf uses fmt.Sprintf to log a templated message.
func (l *WrappedLogger) LogErrorf(template string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Errorf(template, args...)
	}
}

// LogFatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func (l *WrappedLogger) LogFatal(args ...interface{}) {
	if l.logger != nil {
		l.logger.Fatal(args...)
	}
}

// LogFatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func (l *WrappedLogger) LogFatalf(template string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Fatalf(template, args...)
	}
}

// LogInfo uses fmt.Sprint to construct and log a message.
func (l *WrappedLogger) LogInfo(args ...interface{}) {
	if l.logger != nil {
		l.logger.Info(args...)
	}
}

// LogInfof uses fmt.Sprintf to log a templated message.
func (l *WrappedLogger) LogInfof(template string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Infof(template, args...)
	}
}

// LogWarn uses fmt.Sprint to construct and log a message.
func (l *WrappedLogger) LogWarn(args ...interface{}) {
	if l.logger != nil {
		l.logger.Warn(args...)
	}
}

// LogWarnf uses fmt.Sprintf to log a templated message.
func (l *WrappedLogger) LogWarnf(template string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Warnf(template, args...)
	}
}

// LogPanic uses fmt.Sprint to construct and log a message, then panics.
func (l *WrappedLogger) LogPanic(args ...interface{}) {
	if l.logger != nil {
		l.logger.Panic(args...)
	}
}

// LogPanicf uses fmt.Sprintf to log a templated message, then panics.
func (l *WrappedLogger) LogPanicf(template string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Panicf(template, args...)
	}
}
