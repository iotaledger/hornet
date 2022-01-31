To generate the snapshot for the assets:

* Snapshot tests:
  - `cd tools/intsnap && go run main.go`

* Migration tests:
  - `./hornet tools snap-gen alphanet1 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 10000000000 migration_full_snapshot.bin`

* Other tests:
  - `./hornet tools snap-gen alphanet1 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 0 full_snapshot.bin`