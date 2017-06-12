#!/bin/bash

set -ve
go get -u \
  github.com/mailru/easyjson/easyjson \
  github.com/valyala/quicktemplate/qtc

go generate

gofmt -w -s templates/*.go

go build

time ./chromedp-gen $@

go install ../../cdp/...
