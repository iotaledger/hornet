## Bootstrapping the Chrysalis Phase 2 Hornet Node From the Genesis Snapshot

1. Rename the `genesis_snapshot.bin` to `full_snapshot.bin`.
1. Make sure your C2 (Chrysalis Phase 2) Hornet node has no database and no prior snapshot files.
1. Place the `full_snapshot.bin` in the directory as defined in the `snapshots.fullPath` config option (this option
   contains the full path including the file name).
1. Adjust `protocol.networkID` to the same value as used in
   the `-genesis-snapshot-file-network-id="<network-id-for-chrysalis-phase-2>"` flag. (This should not really be
   necessary as the C2 Hornet version will ship with the appropriate default values).
1. Control that the corresponding `protocol.publicKeyRanges` are correct.
1. Start your C2 Hornet node and add peers via the dashboard.
