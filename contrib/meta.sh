#!/bin/bash

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../)

pushd $SRC &> /dev/null

gometalinter \
  --disable=aligncheck \
  --enable=misspell \
  --enable=gofmt \
  --deadline=100s \
  --cyclo-over=25 \
  --sort=path \
  --exclude='\(defer (.+?)\)\) \(errcheck\)$' \
  --exclude='/easyjson\.go.*(passes|copies) lock' \
  --exclude='/easyjson\.go.*ineffectual assignment' \
  --exclude='/easyjson\.go.*unnecessary conversion' \
  --exclude='/easyjson\.go.*this value of key is never used' \
  --exclude='/easyjson\.go.*\((gocyclo|golint|goconst|staticcheck)\)$' \
  --exclude='^cdp/.*Potential hardcoded credentials' \
  --exclude='^cdp/cdp\.go.*UnmarshalEasyJSON.*\(gocyclo\)$' \
  --exclude='^cdp/cdputil/cdputil\.go.*UnmarshalMessage.*\(gocyclo\)$' \
  --exclude='^cmd/chromedp-gen/.*\((gocyclo|interfacer)\)$' \
  --exclude='^cmd/chromedp-proxy/main\.go.*\(gas\)$' \
  --exclude='^cmd/chromedp-gen/fixup/fixup\.go.*\(goconst\)$' \
  --exclude='^cmd/chromedp-gen/internal/enum\.go.*unreachable' \
  --exclude='^cmd/chromedp-gen/(main|domain-gen)\.go.*\(gas\)$' \
  --exclude='^examples/[a-z]+/main\.go.*\(errcheck\)$' \
  --exclude='^kb/gen\.go.*\((gas|vet)\)$' \
  --exclude='^runner/.*\(gas\)$' \
  --exclude='^handler\.go.*cmd can be easyjson\.Marshaler' \
  ./...

popd &> /dev/null
