#!/bin/bash

# updates protocol.json to the latest from the chromium source tree

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd ))

BASE_URL="https://chromium.googlesource.com"
BROWSER_PROTO="$BASE_URL/chromium/src/+/master/third_party/WebKit/Source/core/inspector/browser_protocol.json?format=TEXT"
JS_PROTO="$BASE_URL/v8/v8/+/master/src/inspector/js_protocol.json?format=TEXT"

OUT=$SRC/protocol.json

TMP=$(mktemp -d /tmp/chromedp-gen.XXXXXX)
BROWSER_TMP="$TMP/browser_protocol.json"
JS_TMP="$TMP/js_protocol.json"

set -ve
# download
curl -s $BROWSER_PROTO | base64 -d > $BROWSER_TMP
curl -s $JS_PROTO | base64 -d > $JS_TMP

# merge browser_protocol.json and js_protocol.json
jq -s '[.[] | to_entries] | flatten | reduce .[] as $dot ({}; .[$dot.key] += $dot.value)' $BROWSER_TMP $JS_TMP > $OUT

# convert boolean values listed as strings to real booleans
# (this is not used in favor of using the custom Bool type that correctly JSON unmarshals the value)
# left here for completeness
#perl -pi -e 's/"\s*:\s*"(true|false)"/": \1/g' $OUT
