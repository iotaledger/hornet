#!/bin/bash
#
# Allows manual building of packages via goreleaser.
# Allows building of dirty git state and will not push results to github.

# make script executable independent of path
trap 'cd $(dirs -l -0)' EXIT INT QUIT TERM
cd ..

GORELEASER_IMAGE=iotmod/goreleaser-cgo-cross-compiler:1.13.5-musl
REPO_PATH="/build"

docker pull "${GORELEASER_IMAGE}"
docker run --rm --privileged -v "${PWD}":"${REPO_PATH}" -w "${REPO_PATH}" "${GORELEASER_IMAGE}" goreleaser --rm-dist --snapshot --skip-publish
