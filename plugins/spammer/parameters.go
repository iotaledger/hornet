package spammer

import flag "github.com/spf13/pflag"

func init() {
	flag.String("spammer.address", "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999", "Tx Address")
	flag.String("spammer.message", "Spamming with HORNET tipselect", "Message of the Tx")
	flag.String("spammer.tag", "HORNET99INTEGRATED99SPAMMER", "Tag of the Tx")
	flag.Uint("spammer.depth", 3, "Depth of the random walker")
	flag.Float64("spammer.tpsRateLimit", 0.10, "Rate limit for the spam (0 = no limit)")
	flag.Uint("spammer.workers", 1, "How many spammers should run in parallel")
}
