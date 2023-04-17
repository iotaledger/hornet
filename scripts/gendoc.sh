#!/bin/bash
pushd ../tools/gendoc

# determine current HORNET version tag
commit_hash=$(git rev-parse --short HEAD)

BUILD_TAGS=rocksdb
BUILD_LD_FLAGS="-X=github.com/iotaledger/hornet/v2/components/app.Version=${commit_hash}"

go run -tags ${BUILD_TAGS} -ldflags ${BUILD_LD_FLAGS} main.go

popd
