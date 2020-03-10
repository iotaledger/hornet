#!/bin/bash
#
# Builds HORNET with the latest git tag and commit hash (short)
# E.g.: ./hornet -v --> HORNET 0.3.0-75316fe

latest_tag=$(git describe --tags $(git rev-list --tags --max-count=1))
commit_hash=$(git rev-parse --short HEAD)

go build -ldflags="-s -w -X github.com/gohornet/hornet/plugins/cli.AppVersion=${latest_tag:1}-$commit_hash"
