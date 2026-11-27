#!/bin/sh
set -eu
cd "$(dirname "$0")"
exec ./savesync -config ./config.json > ./sync.log 2>&1
