#!/bin/bash

# go to root directory
cd ..

# snapshot tests
echo "create snapshot test snapshot files..."
cd tools/intsnap && go run main.go && mv *.bin ../../integration-tests/assets/ && cd ../..

# migration tests
echo "create migration test snapshot files..."
rm integration-tests/assets/migration_full_snapshot.bin && go run main.go tools snap-gen --networkID alphanet1 --mintAddress 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 --treasuryAllocation 10000000000 --outputPath integration-tests/assets/migration_full_snapshot.bin

# other tests
echo "create other snapshot files..."
rm integration-tests/assets/full_snapshot.bin && go run main.go tools snap-gen --networkID alphanet1 --mintAddress 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 --treasuryAllocation 0 --outputPath integration-tests/assets/full_snapshot.bin

# return to this directory
cd integration-tests/

