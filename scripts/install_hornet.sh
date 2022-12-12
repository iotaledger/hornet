#!/bin/bash
#
# Installs HORNET with the latest commit hash (short)
# E.g.: ./hornet -v --> HORNET 75316fe

DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

commit_hash=$(git rev-parse --short HEAD)
go install -ldflags="-s -w -X github.com/gohornet/hornet/core/app.Version=$commit_hash"
