package database

import (
	"strings"

	"github.com/iotaledger/hive.go/logger"
)

type BadgerLogger struct {
	log *logger.Logger
}

func NewBadgerLogger() *BadgerLogger {
	return &BadgerLogger{
		log: logger.NewLogger("Badger"),
	}
}

func (b BadgerLogger) Errorf(s string, i ...interface{}) {
	b.log.Errorf(strings.TrimSuffix(s, "\n"), i...)
}

func (b BadgerLogger) Warningf(s string, i ...interface{}) {
	b.log.Warnf(strings.TrimSuffix(s, "\n"), i...)
}

func (b BadgerLogger) Infof(s string, i ...interface{}) {
	b.log.Infof(strings.TrimSuffix(s, "\n"), i...)
}

func (b BadgerLogger) Debugf(s string, i ...interface{}) {
	b.log.Debugf(strings.TrimSuffix(s, "\n"), i...)
}
