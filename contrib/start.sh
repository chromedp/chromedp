#!/bin/bash

TMP=$(mktemp -d /tmp/google-chrome.XXXXX)

google-chrome \
  --user-data-dir=$TMP \
  --remote-debugging-port=9222 \
  --no-first-run \
  --no-default-browser-check \
  about:blank
