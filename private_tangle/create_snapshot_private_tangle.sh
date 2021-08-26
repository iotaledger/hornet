#!/bin/bash
rm snapshots/private_tangle1/full_snapshot.bin
rm snapshots/private_tangle1/delta_snapshot.bin
rm snapshots/private_tangle2/full_snapshot.bin
rm snapshots/private_tangle2/delta_snapshot.bin
rm snapshots/private_tangle3/full_snapshot.bin
rm snapshots/private_tangle3/delta_snapshot.bin
rm snapshots/private_tangle4/full_snapshot.bin
rm snapshots/private_tangle4/delta_snapshot.bin
mkdir -p snapshots/private_tangle1/
mkdir -p snapshots/private_tangle2/
mkdir -p snapshots/private_tangle3/
mkdir -p snapshots/private_tangle4/
go run ../main.go tool snap-gen private_tangle1 60200bad8137a704216e84f8f9acfe65b972d9f4155becb4815282b03cef99fe 1000000000 snapshots/private_tangle1/full_snapshot.bin
cp snapshots/private_tangle1/full_snapshot.bin snapshots/private_tangle2/
cp snapshots/private_tangle1/full_snapshot.bin snapshots/private_tangle3/
cp snapshots/private_tangle1/full_snapshot.bin snapshots/private_tangle4/
