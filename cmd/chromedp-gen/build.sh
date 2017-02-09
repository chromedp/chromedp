#!/bin/bash

set -ve
go generate

go build

time ./chromedp-gen $@

go install ../../cdp/...
