package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/gohornet/hornet/packages/syncutils"
)

// every instance of the logger uses the same logger to ensure that
// concurrent prints/writes don't overlap
var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

func NewLogger(prefix string, logLevel ...LogLevel) *Logger {
	l := &Logger{Prefix: prefix}
	if len(logLevel) > 0 {
		l.logLevel = logLevel[0]
	} else {
		l.logLevel = LevelNormal
	}
	return l
}

type LogLevel byte

const (
	LevelInfo     LogLevel = 1
	LevelNotice            = LevelInfo << 1
	LevelWarning           = LevelNotice << 1
	LevelError             = LevelWarning << 1
	LevelCritical          = LevelError << 1
	LevelPanic             = LevelCritical << 1
	LevelFatal             = LevelPanic << 1
	LevelDebug             = LevelFatal << 1

	LevelNormal = LevelInfo | LevelNotice | LevelWarning | LevelError | LevelCritical | LevelPanic | LevelFatal
)

type Logger struct {
	Prefix     string
	changeMu   syncutils.Mutex
	logLevel   LogLevel
	disabledMu syncutils.Mutex
	disabled   bool
}

func (l *Logger) Enabled() bool {
	return !l.disabled
}

func (l *Logger) Enable() {
	l.disabledMu.Lock()
	l.disabled = false
	l.disabledMu.Unlock()
}

func (l *Logger) Disable() {
	l.disabledMu.Lock()
	l.disabled = true
	l.disabledMu.Unlock()
}

func (l *Logger) ChangeLogLevel(logLevel LogLevel) {
	l.changeMu.Lock()
	l.logLevel = logLevel
	l.changeMu.Unlock()
}

// Fatal is equivalent to l.Critical(fmt.Sprint()) followed by a call to os.Exit(1).
func (l *Logger) Fatal(args ...interface{}) {
	if l.logLevel&LevelFatal == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ FATAL ] %s:", l.Prefix), fmt.Sprint(args...))
	os.Exit(1)
}

// Fatalf is equivalent to l.Critical followed by a call to os.Exit(1).
func (l *Logger) Fatalf(format string, args ...interface{}) {
	if l.logLevel&LevelFatal == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ FATAL ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Panic is equivalent to l.Critical(fmt.Sprint()) followed by a call to panic().
func (l *Logger) Panic(args ...interface{}) {
	if l.logLevel&LevelPanic == 0 || l.disabled {
		return
	}
	logger.Panicln(fmt.Sprintf("[ PANIC ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Panicf is equivalent to l.Critical followed by a call to panic().
func (l *Logger) Panicf(format string, args ...interface{}) {
	if l.logLevel&LevelPanic == 0 || l.disabled {
		return
	}
	logger.Panicln(fmt.Sprintf("[ PANIC ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Critical logs a message using CRITICAL as log level.
func (l *Logger) Critical(args ...interface{}) {
	if l.logLevel&LevelCritical == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ CRITICAL ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Criticalf logs a message using CRITICAL as log level.
func (l *Logger) Criticalf(format string, args ...interface{}) {
	if l.logLevel&LevelCritical == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ CRITICAL ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Error logs a message using ERROR as log level.
func (l *Logger) Error(args ...interface{}) {
	if l.logLevel&LevelError == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ ERROR ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Errorf logs a message using ERROR as log level.
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.logLevel&LevelError == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ ERROR ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Warning logs a message using WARNING as log level.
func (l *Logger) Warning(args ...interface{}) {
	if l.logLevel&LevelWarning == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ WARNING ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Warningf logs a message using WARNING as log level.
func (l *Logger) Warningf(format string, args ...interface{}) {
	if l.logLevel&LevelWarning == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ WARNING ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Notice logs a message using NOTICE as log level.
func (l *Logger) Notice(args ...interface{}) {
	if l.logLevel&LevelNotice == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ NOTICE ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Noticef logs a message using NOTICE as log level.
func (l *Logger) Noticef(format string, args ...interface{}) {
	if l.logLevel&LevelNotice == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ NOTICE ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Info logs a message using INFO as log level.
func (l *Logger) Info(args ...interface{}) {
	if l.logLevel&LevelInfo == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ INFO ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Infof logs a message using INFO as log level.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.logLevel&LevelInfo == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ INFO ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}

// Debug logs a message using DEBUG as log level.
func (l *Logger) Debug(args ...interface{}) {
	if l.logLevel&LevelDebug == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ DEBUG ] %s:", l.Prefix), fmt.Sprint(args...))
}

// Debugf logs a message using DEBUG as log level.
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.logLevel&LevelDebug == 0 || l.disabled {
		return
	}
	logger.Println(fmt.Sprintf("[ DEBUG ] %s:", l.Prefix), fmt.Sprintf(format, args...))
}
