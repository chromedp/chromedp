#!/bin/bash

UPDATE=0
LASTUPDATE=0
if [ -f .last ]; then
  LASTUPDATE=$(cat .last)
fi

NOW=$(date +%s)
if (( "$NOW" >= $(($LASTUPDATE + 86400*5)) )); then
  UPDATE=1
fi

if [[ "$UPDATE" == 1 ]]; then
  go get -u \
    golang.org/x/tools/cmd/goimports \
    github.com/mailru/easyjson/easyjson \
    github.com/valyala/quicktemplate/qtc

  date +%s > .last
fi

set -ve
go generate

gofmt -w -s templates/*.go

go build

time ./chromedp-gen $@

go install ../../cdp/...
