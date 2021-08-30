---
keywords:
- IOTA Node 
- Hornet Node
- verifier
- Chrysalis Phase 2
- receipts
description: How to configure a Hornet node as a verifier node.  
image: /img/logo/HornetLogo.png
---


# How to Run Hornet as a Verifier Node

 A verifier node is a node which validates receipts. Receipts are an integral component of the migration mechanism used to migrate funds from the legacy into the new Chrysalis Phase 2 network. You can find a more detailed explanation on how the migration mechanism works in the [Chrysalis documentation](https://chrysalis.docs.iota.org/guides/migration-mechanism).

This guide explains how to configure a Hornet node as a verifier node:

1. Make sure you enabled the `Receipts` plugin under `node.enablePlugins`.
2. Set :
    - `receipts.validator.validate` to `true`. This enables the verification logic in your node.
    - `receipts.validator.ignoreSoftErrors` to `true` or `false`. 
      - Set it to  `true`, if you don't want the verifier node to panic if it can not query a legacy node for data. 
      -  Set it to `false` if you want to make sure that your verifier node panics if it can not query for data from a legacy node. 
      - An invalid receipt will always result in a panic. `ignoreSoftErrors` only controls API call failures to the legacy node.
    - `receipts.validator.api.timeout` to something sensible like `10s` (meaning 10 seconds).
    - `receipts.validator.api.address` to the URI of your legacy node. Note that this legacy node must have the `getWhiteFlagConfirmation` and `getNodeInfo` API commands whitelisted.
    - `receipts.validator.coordinator.address` to the Coordinator address in the legacy network.
    - `receipts.validator.coordinator.merkleTreeDepth` to the correct used Merkle tree depth in the legacy network.
   
3. Run your Hornet verifier node and let it validate receipts.

:::info
We recommend that you use a load balanced endpoint to multiple legacy nodes for `receipts.validator.api.address` in order to obtain high availability.
:::

After this, if your verifier node panics because of an invalid receipt, it is clear that a one of the produced receipts is not valid. In this case, as a verifier node operator, you are invited to inform the community and the IOTA Foundation of your findings. This is, by the way, the same result as when the Coordinator issues a milestone, which diverges from a consistent ledger state.
