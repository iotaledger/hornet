## How to Run Hornet as a Verifier Node

> A verifier node is a node which validates receipts. Receipts are an integral component of the
> migration mechanism used to migrate funds from the legacy into the new Chrysalis Phase 2 network.
> See [here](https://chrysalis.docs.iota.org/guides/migration-in-depth.html) for a more detailed explanation on how the migration mechanism works.

This guide explains how to configure a Hornet node as a verifier node:

1. Make sure the `Receipts` plugin is enabled under `node.enablePlugins`.
1. Set:
    - `receipts.validator.validate` to `true` (this is what enables the verification logic in your node).
    - `receipts.validator.ignoreSoftErrors` to `true` or `false`. If `true`, the verifier node will not panic if it can
      not query a legacy node for data. Set it to `false` if you want to make sure that your verifier node panics if it
      can not query for data from a legacy node. An invalid receipt will always result in a panic; `ignoreSoftErrors`
      only controls API call failures to the legacy node.
    - `receipts.validator.api.timeout` to something sensible like `10s` (meaning 10 seconds).
    - `receipts.validator.api.address` to the URI of your legacy node. Note that this legacy node **must support/have
      the `getWhiteFlagConfirmation` and `getNodeInfo` API commands whitelisted**.
    - `receipts.validator.coordinator.address` to the Coordinator address in the legacy network.
    - `receipts.validator.coordinator.merkleTreeDepth` to the correct used Merkle tree depth in the legacy network.
1. Run your Hornet verifier node and let it validate receipts.

> Note, it is suggested that you use a loadbalanced endpoint to multiple legacy nodes for `receipts.validator.api.address`
> in order to obtain high availability.

If now your verifier node panics because of an invalid receipt, it is clear that a receipt was produced which is not
valid, in which case as a verifier node operator, you are invited to inform the community and the IOTA Foundation of
your findings. This is by the way the same result as when a milestone is issued by the Coordinator, which diverges from
a consistent ledger state.