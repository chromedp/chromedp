#!/bin/bash

SRC=$(realpath $(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)/..)

pushd $SRC &> /dev/null

IMAGE=${IMAGE:-docker.io/chromedp/headless-shell:latest}

set -e

(set -x;
  CGO_ENABLED=0 go test -c
)

CMD=docker
if [ ! -z "$(type -p podman)" ]; then
  CMD=podman
fi

(set -x;
  $CMD run \
    --rm \
    --volume=$PWD:/chromedp \
    --entrypoint=/chromedp/chromedp.test \
    --workdir=/chromedp \
    --env=PATH=/headless-shell \
    --env=HEADLESS_SHELL=1 \
    $IMAGE -test.v -test.parallel=1 -test.timeout=3m
)

popd &> /dev/null
