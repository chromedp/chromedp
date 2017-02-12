#!/bin/bash

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../)

set -ve

pushd $SRC &> /dev/null

echo 'mode: atomic' > coverage.out && go list ./... | xargs -n1 -I{} sh -c 'go test -covermode=atomic -coverprofile=coverage.tmp {} && tail -n +2 coverage.tmp >> coverage.out' && rm coverage.tmp

go tool cover -html=coverage.out

popd &> /dev/null
