#!/bin/bash

BASE=$(realpath $(cd -P $GOPATH/src/github.com/chromedp && pwd))

FILES=$(find $BASE/{chromedp*,goquery,examples} -type f -iname \*.go -not -iname \*.qtpl.go -print0|wc -l --files0-from=-|head -n -1)$'\n'

AUTOG=$(find $BASE/cdproto/ -type f -iname \*.go -not -iname \*easyjson\* -print0|wc -l --files0-from=-|head -n -1)

if [ "$1" != "--total" ]; then
  echo -e "code:\n$FILES\n\ngenerated:\n$AUTOG"
else
  echo "code: $(awk '{s+=$1} END {print s}' <<< "$FILES")"
  echo "generated: $(awk '{s+=$1} END {print s}' <<< "$AUTOG")"
fi
