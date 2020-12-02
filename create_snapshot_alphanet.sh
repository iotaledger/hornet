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
go run main.go tool snapgen alphanet1 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 snapshots/alphanet1/full_export.bin
cp snapshots/alphanet1/full_export.bin snapshots/alphanet2/
cp snapshots/alphanet1/full_export.bin snapshots/alphanet3/
cp snapshots/alphanet1/full_export.bin snapshots/alphanet4/
