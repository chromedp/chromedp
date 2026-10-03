#!/bin/bash

SRC=$(realpath $(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)/..)

pushd $SRC &> /dev/null

IMAGE=${IMAGE:-docker.io/chromedp/headless-shell:latest}

# The repository has three modules. Each one builds its own test binary, and
# runs it from its own directory, where its testdata is.
MODULES=(. remote test)

set -e

for MODULE in "${MODULES[@]}"; do
  (set -x;
    cd $MODULE
    CGO_ENABLED=0 go test -c -o chromedp.test
  )
done

CMD=docker
if [ ! -z "$(type -p podman)" ]; then
  CMD=podman
fi

for MODULE in "${MODULES[@]}"; do
  (set -x;
    $CMD run \
      --rm \
      --volume=$PWD:/chromedp \
      --entrypoint=/chromedp/$MODULE/chromedp.test \
      --workdir=/chromedp/$MODULE \
      --env=PATH=/headless-shell \
      --env=HEADLESS_SHELL=1 \
      $IMAGE -test.v -test.parallel=1 -test.timeout=10m
  )
done

popd &> /dev/null
