#!/bin/bash
pushd ./..

go mod tidy

cd tools/gendoc

go mod tidy

popd
