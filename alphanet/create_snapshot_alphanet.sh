#!/bin/bash
rm snapshots/alphanet1/full_export.bin
rm snapshots/alphanet1/delta_export.bin
rm snapshots/alphanet2/full_export.bin
rm snapshots/alphanet2/delta_export.bin
rm snapshots/alphanet3/full_export.bin
rm snapshots/alphanet3/delta_export.bin
rm snapshots/alphanet4/full_export.bin
rm snapshots/alphanet4/delta_export.bin
mkdir -p snapshots/alphanet1/
mkdir -p snapshots/alphanet2/
mkdir -p snapshots/alphanet3/
mkdir -p snapshots/alphanet4/
go run ../main.go tool snapgen alphanet1 60200bad8137a704216e84f8f9acfe65b972d9f4155becb4815282b03cef99fe 1000000000 snapshots/alphanet1/full_export.bin
cp snapshots/alphanet1/full_export.bin snapshots/alphanet2/
cp snapshots/alphanet1/full_export.bin snapshots/alphanet3/
cp snapshots/alphanet1/full_export.bin snapshots/alphanet4/
