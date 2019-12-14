package spammer

import flag "github.com/spf13/pflag"

func init() {
	flag.String("spammer.address", "TANGLEKIT99SPAMMER99INTEGRATED99IN99HORNET999999999999999999999999999999999999999", "Tx Address")
	flag.String("spammer.message", "Spamming with HORNET tipselect", "Message of the Tx")
	flag.String("spammer.tag", "TANGLEKIT9SPAMMER99HORNET99", "Tag of the Tx")
	flag.Uint("spammer.depth", 3, "Depth of the random walker")
	flag.Uint("spammer.tpsRateLimit", 1, "Rate limit for the spam (0 = no limit)")
}
