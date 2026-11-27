#!/bin/sh
set -eu
cd "$(dirname "$0")/SaveSync"
exec ./savesync -config ./config.json -ui -side knulli >> ./sync.log 2>&1
