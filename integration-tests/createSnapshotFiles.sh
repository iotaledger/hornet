#!/bin/bash

# go to root directory
cd ..

# snapshot tests
echo "create snapshot test snapshot files..."
cd tools/intsnap && go run main.go && mv *.bin ../../integration-tests/assets/ && cd ../..

# migration tests
echo "create migration test snapshot files..."
rm integration-tests/assets/migration_full_snapshot.bin && go run main.go tools snap-gen --protocolParametersPath integration-tests/protocol_parameters.json --mintAddress atoi1qp5jpvtk7cf7c7l9ne50c684jl4n8ya0srm5clpak7qes9ratu0eysflmsz --treasuryAllocation 10000000000 --outputPath integration-tests/assets/migration_full_snapshot.bin

# other tests
echo "create other snapshot files..."
rm integration-tests/assets/full_snapshot.bin && go run main.go tools snap-gen --protocolParametersPath integration-tests/protocol_parameters.json --mintAddress atoi1qp5jpvtk7cf7c7l9ne50c684jl4n8ya0srm5clpak7qes9ratu0eysflmsz --treasuryAllocation 0 --outputPath integration-tests/assets/full_snapshot.bin

# return to this directory
cd integration-tests/

