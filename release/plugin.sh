#!/bin/sh
# Build a universal marketplace plugin from the four standalone skill releases.
set -eu
cd "$(dirname "$0")/.."
out=${1:-dist}
ref=$(jq -er '.plugins[0].source.ref' .agents/plugins/marketplace.json)
test "$ref" = "$(jq -er '.plugins[0].source.ref' .claude-plugin/marketplace.json)"
version=${SPOTIFY_PLUGIN_VERSION:-${ref#spotify-plugin-v}}
echo "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
plugin="$stage/spotify-plugin"
skill="$plugin/skills/spotify"
mkdir -p "$plugin/.claude-plugin" "$plugin/.codex-plugin" "$skill/scripts/bin"
jq -n --arg version "$version" '{name:"spotify",version:$version,description:"Manage Spotify playlists and control playback."}' > "$plugin/.claude-plugin/plugin.json"
jq '. + {skills:"./skills/"}' "$plugin/.claude-plugin/plugin.json" > "$plugin/.codex-plugin/plugin.json"
install -m 755 release/plugin-launcher "$skill/scripts/spotify"
cp SKILL.md "$skill/"
for platform in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
  archive="$out/spotify-$platform.tar.gz"
  tar -xOf "$archive" spotify/SKILL.md | cmp - SKILL.md
  binary="$skill/scripts/bin/spotify-$platform"
  tar -xOf "$archive" spotify/scripts/spotify > "$binary"
  chmod 755 "$binary"
done
tar --sort=name --format=ustar --mtime=@0 --owner=0 --group=0 --numeric-owner --mode='u=rwX,go=rX' -cf "$stage/plugin.tar" -C "$stage" spotify-plugin
gzip -n -c "$stage/plugin.tar" > "$out/spotify-plugin.tar.gz"
