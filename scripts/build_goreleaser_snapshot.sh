#!/bin/bash
#
# Allows manual building of packages via goreleaser.
# Allows building of dirty git state and will not push results to github.

# make script executable independent of path
SCRIPTDIR=$(dirname "$0")
pushd "${SCRIPTDIR}" >/dev/null
pushd ".." >/dev/null


GORELEASER_IMAGE=iotmod/goreleaser-cgo-cross-compiler:1.13.5
REPO_PATH="/build"

docker pull "${GORELEASER_IMAGE}"
docker run --rm --privileged -v "${PWD}":"${REPO_PATH}" -w "${REPO_PATH}" "${GORELEASER_IMAGE}" goreleaser --rm-dist --snapshot --skip-publish


popd >/dev/null
popd >/dev/null
