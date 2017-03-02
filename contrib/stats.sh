#!/bin/bash

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../)

FILES=$(find $SRC/cmd/chromedp-gen -type f -iname \*.go -not -iname \*.qtpl.go -print0|wc -l --files0-from=-|head -n -1)$'\n'
FILES+=$(find $SRC/{client,runner,kb,contrib,examples} -type f -iname \*.go -print0|wc -l --files0-from=-|head -n -1)$'\n'
FILES+=$(find $SRC/ -maxdepth 1 -type f -iname \*.go -print0|wc -l --files0-from=-|head -n -1)

AUTOG=$(find $SRC/cdp/ -type f -iname \*.go -not -iname \*easyjson\* -print0|wc -l --files0-from=-|head -n -1)

if [ "$1" != "--total" ]; then
  echo -e "code:\n$FILES\n\ngenerated:\n$AUTOG"
else
  echo "code: $(awk '{s+=$1} END {print s}' <<< "$FILES")"
  echo "generated: $(awk '{s+=$1} END {print s}' <<< "$AUTOG")"
fi
