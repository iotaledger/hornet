package prometheus

import (
	tanglepkg "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	whiteFlagTailsIncluded prometheus.Counter
)

func init() {
	whiteFlagTailsIncluded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_wf_tails_included",
			Help: "The count of tails included.",
		},
	)

	registry.MustRegister(whiteFlagTailsIncluded)

	tangle.Events.MilestoneConfirmed.Attach(events.NewClosure(func(wf *tanglepkg.WhiteFlagConfirmation) {
		whiteFlagTailsIncluded.Add(float64(len(wf.Mutations.TailsIncluded)))
	}))
}
