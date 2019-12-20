package profile

import flag "github.com/spf13/pflag"

func init() {
	flag.StringP("useProfile", "p", "default", "Sets the profile with which the node runs")
}
