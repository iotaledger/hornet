#!/bin/bash
rm snapshots/alphanet/export.bin
mkdir -p snapshots/alphanet/
go run main.go tool snapgen 6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92 ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c snapshots/alphanet/export.bin
