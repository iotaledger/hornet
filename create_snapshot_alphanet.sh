#!/bin/bash
rm snapshots/alphanet/full_export.bin
rm snapshots/alphanet/delta_export.bin
mkdir -p snapshots/alphanet/
go run main.go tool snapgen alphanet1 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 snapshots/alphanet/full_export.bin
