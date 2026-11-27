#!/bin/sh
set -eu
cd "$(dirname "$0")"
exec ./savesync -config ./config.json -ui -side muos >> ./sync.log 2>&1
