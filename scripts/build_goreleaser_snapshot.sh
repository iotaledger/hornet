#!/bin/bash
#
# Allows manual building of packages via goreleaser.
# Allows building of dirty git state and will not push results to github.

# make script executable independent of path
cd $(dirname "$0")/../

GORELEASER_IMAGE=iotmod/goreleaser-cgo-cross-compiler:1.15.5
REPO_PATH="/build"

docker pull "${GORELEASER_IMAGE}"
docker run --rm --privileged -v "${PWD}":"${REPO_PATH}" -w "${REPO_PATH}" "${GORELEASER_IMAGE}" goreleaser --rm-dist --snapshot --skip-publish
