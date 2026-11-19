#!/bin/sh
set -eu

# Run from an extracted platform archive.
archive_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
bin="$HOME/.local/bin"
codex_skill="${CODEX_HOME:-$HOME/.codex}/skills/spotify"
claude_skill="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/spotify"

mkdir -p "$bin" "$codex_skill" "$claude_skill"
install -m 755 "$archive_dir/scripts/spotifyctl" "$bin/spotifyctl"
install -m 644 "$archive_dir/SKILL.md" "$codex_skill/SKILL.md"
install -m 644 "$archive_dir/SKILL.md" "$claude_skill/SKILL.md"
printf 'Installed %s\n' "$bin/spotifyctl"
printf 'Configure your Spotify login with %s login\n' "$bin/spotifyctl"
