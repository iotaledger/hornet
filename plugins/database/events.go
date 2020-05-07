package database

import (
	"encoding/json"
	"time"

	"github.com/iotaledger/hive.go/events"
)

type DatabaseCleanup struct {
	Start time.Time
	End   time.Time
}

func (c *DatabaseCleanup) MarshalJSON() ([]byte, error) {

	cleanup := struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	}{
		Start: 0,
		End:   0,
	}

	if !c.Start.IsZero() {
		cleanup.Start = c.Start.Unix()
	}

	if !c.End.IsZero() {
		cleanup.End = c.End.Unix()
	}

	return json.Marshal(cleanup)
}

var Events = pluginEvents{
	DatabaseCleanup: events.NewEvent(DatabaseCleanupCaller),
}

type pluginEvents struct {
	DatabaseCleanup *events.Event
}

func DatabaseCleanupCaller(handler interface{}, params ...interface{}) {
	handler.(func(*DatabaseCleanup))(params[0].(*DatabaseCleanup))
}
