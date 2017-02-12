#!/bin/bash

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../)

pushd $SRC &> /dev/null

gometalinter \
  --disable=aligncheck \
  --deadline=100s \
  --cyclo-over=25 \
  --sort=path \
  --exclude='\(defer (.+?)\)\) \(errcheck\)$' \
  --exclude='func easyjson.*should be' \
  --exclude='/easyjson\.go.*(passes|copies) lock' \
  --exclude='/easyjson\.go.*warning' \
  --exclude='^cdp/.*gocyclo' \
  --exclude='^cdp/.*Potential hardcoded credentials' \
  --exclude='^cmd/chromedp-proxy/main\.go.*\(gas\)$' \
  --exclude='^cmd/chromedp-gen/fixup/fixup\.go.*goconst' \
  --exclude='^cmd/chromedp-gen/internal/enum\.go.*unreachable' \
  --exclude='^cmd/chromedp-gen/main\.go.*\(gas\)$' \
  --exclude='^cmd/chromedp-gen/.*gocyclo' \
  --exclude='^cmd/chromedp-gen/.*interfacer' \
  --exclude='^kb/gen\.go.*\((gas|vet)\)$' \
  --exclude='^runner/.*\(gas\)$' \
  --exclude='^handler\.go.*cmd can be easyjson\.Marshaler' \
  ./...

popd &> /dev/null
