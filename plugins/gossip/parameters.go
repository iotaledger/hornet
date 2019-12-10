package gossip

import flag "github.com/spf13/pflag"

func init() {
	flag.Bool("network.autoTetheringEnabled", false, "Enable new connections from unknown neighbors")
	flag.String("network.address", "0.0.0.0", "Bind the TCP server socket for the gossip protocol to an address")
	flag.Int("network.port", 15600, "Bind the TCP server socket for the gossip protocol to a port")
	flag.StringSlice("network.neighbors", nil, "Set the URLs and IP addresses of neighbors")
	flag.Int("network.reconnectAttemptIntervalSeconds", 60, "Set the number of seconds to wait before trying to reconnect to a disconnected neighbor")
	flag.Int("network.maxNeighbors", 5, "Set the maximum number of neighbors")

	flag.String("milestones.coordinator", "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9", "The address of the coordinator")
	flag.Int("milestones.coordinatorSecurityLevel", 2, "The security level used in coordinator signatures")
	flag.Int("milestones.numberOfKeysInAMilestone", 23, "The depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message.")

	flag.Int("protocol.mwm", 14, "The minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash. Increasing this number by 1 will result in proof of work that is 3 times as hard.")
}
