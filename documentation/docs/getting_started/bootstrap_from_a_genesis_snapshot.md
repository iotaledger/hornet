---
keywords:
- IOTA Node 
- Hornet Node
- genesis snapshot
- Chrysalis Phase 2
- bootstrap network
description: Please follow these instructions to bootstrap the Chrysalis Phase 2 Hornet node from the Genesis Snapshot.
image: /img/logo/HornetLogo.png
---
# Bootstrapping the Chrysalis Phase 2 Hornet Node From the Genesis Snapshot

Please follow these instructions to bootstrap the Chrysalis Phase 2 Hornet node from the Genesis Snapshot:

1. Rename the `genesis_snapshot.bin` file to `full_snapshot.bin`.
2. Make sure your C2 (Chrysalis Phase 2) Hornet node has no database and no prior snapshot files.
3. Place the `full_snapshot.bin` file in the directory as defined in the `snapshots.fullPath` config option (this option contains the full path including the file name).
4. Adjust `protocol.networkID` to the same value as used in the `-genesis-snapshot-file-network-id="<network-id-for-chrysalis-phase-2>"` flag. This step may not be necessary as the C2 Hornet version will ship with the appropriate default values.
5. Control that the corresponding `protocol.publicKeyRanges` are correct.
6. Start your C2 Hornet node and add peers using the dashboard.
