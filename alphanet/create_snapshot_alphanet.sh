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
go run ../main.go tool snapgen alphanet1 fb9de5f493239dff165e574ec3f5be0f1f5a4c9e4ff2568d6b137445ebe4ff40 1000000000 snapshots/alphanet1/full_export.bin
cp snapshots/alphanet1/full_export.bin snapshots/alphanet2/
cp snapshots/alphanet1/full_export.bin snapshots/alphanet3/
cp snapshots/alphanet1/full_export.bin snapshots/alphanet4/
