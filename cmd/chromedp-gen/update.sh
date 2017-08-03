#!/bin/bash

# updates protocol.json to the latest from the chromium source tree

OUT=protocol.json
HAR_PROTO=har.json
BASE_URL="https://chromium.googlesource.com"

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd ))

BROWSER_VER=${1:-"master"}
JS_VER=${2:-"master"}

set -e

pushd $SRC &> /dev/null

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

BROWSER_PROTO="$BASE_URL/chromium/src/+/$BROWSER_VER/third_party/WebKit/Source/core/inspector/browser_protocol.json?format=TEXT"
JS_PROTO="$BASE_URL/v8/v8/+/$JS_VER/src/inspector/js_protocol.json?format=TEXT"

TMP=$(mktemp -d /tmp/chromedp-gen.XXXXXX)
BROWSER_TMP="$TMP/browser_protocol.json"
JS_TMP="$TMP/js_protocol.json"

echo "BROWSER_PROTO: $BROWSER_PROTO"
echo "JS_PROTO: $JS_PROTO"

# download
curl -s $BROWSER_PROTO | base64 -d > $BROWSER_TMP
curl -s $JS_PROTO | base64 -d > $JS_TMP

# merge browser, js and har definition files
jq \
  -s '[.[] | to_entries] | flatten | reduce .[] as $dot ({}; .[$dot.key] += $dot.value)' \
  $BROWSER_TMP $JS_TMP $HAR_PROTO > $OUT

popd &> /dev/null
