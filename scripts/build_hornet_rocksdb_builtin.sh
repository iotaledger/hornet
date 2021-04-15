#!/bin/bash
#
# Builds HORNET with the latest commit hash (short)
# E.g.: ./hornet -v --> HORNET 75316fe

commit_hash=$(git rev-parse --short HEAD)

go build -ldflags="-s -w -X github.com/gohornet/hornet/core/app.Version=$commit_hash" -tags rocksdb,builtin_static
