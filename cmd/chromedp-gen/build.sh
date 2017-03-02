#!/bin/bash

set -ve
go generate

gofmt -w -s templates/*.go

go build

time ./chromedp-gen $@

go install ../../cdp/...
