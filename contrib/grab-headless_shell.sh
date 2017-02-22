#!/bin/bash

set -e

OUT=${1:-headless_shell}
VER=$2

if [ -z "$VER" ]; then
  VER=$(curl -s https://storage.googleapis.com/docker-chrome-headless/latest.txt|sed -e 's/^headless_shell-//' -e 's/\.tar\.bz2$//')
fi

mkdir -p $OUT

pushd $OUT &> /dev/null

curl -s https://storage.googleapis.com/docker-chrome-headless/headless_shell-$VER.tar.bz2 | tar -jxv

popd &> /dev/null
