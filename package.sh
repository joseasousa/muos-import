#!/bin/sh
set -eu
cd "$(dirname "$0")"
out="${1:-dist}"
muos="$out/muos/MUOS/application/SaveSync"
knulli="$out/knulli/roms/ports"
mkdir -p "$muos" "$knulli/SaveSync"
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -tags sdl -trimpath -o "$muos/savesync" .
cp "$muos/savesync" "$knulli/SaveSync/savesync"
cp packaging/muos/launch.sh "$muos/launch.sh"
cp packaging/knulli/SaveSync.sh "$knulli/SaveSync.sh"
chmod +x "$muos/launch.sh" "$knulli/SaveSync.sh"
for side in muos knulli; do
 case "$side" in
 muos) target="$muos";;
 knulli) target="$knulli/SaveSync";;
 esac
 cat > "$target/config.example.json" <<CONFIG
{
  "Side": "$side",
  "Card": "/path/to/sd2",
  "ROMs": "/path/to/roms",
  "Cores": {
    "Gambatte": {"Saves": "/path/to/gambatte"},
    "mGBA": {"Saves": "/path/to/mgba"}
  }
}
CONFIG
done
