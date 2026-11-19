# Spotify skill and CLI

Install the skill to search Spotify, manage playlists and Liked Songs, inspect recent plays, and control playback from Codex, Claude Code, or another agent that can run local commands.

## Install from the plugin marketplace

**Claude Code** — run these commands inside Claude Code:

```text
/plugin marketplace add cutalion/spotify
/plugin install spotify@cutalion
/reload-plugins
```

**Codex** — run these commands in a terminal, then start a new session:

```sh
codex plugin marketplace add cutalion/spotify
codex plugin add spotify@cutalion
```

Ask the agent to “use the Spotify plugin to show my current track.” On first use, ask it for the installed skill’s absolute `scripts/spotifyctl` path and run that executable with `login` in your terminal. The plugin includes the Linux and macOS binaries for Intel/AMD x86-64 and ARM64. It uses the same Spotify login as the standalone skill.

## Install the skill manually

Choose the release archive for Linux or macOS (`darwin`) and your processor. Use `amd64` for Intel/AMD x86-64 and `arm64` for Apple Silicon or 64-bit ARM. Each archive contains the skill instructions and a standalone executable. [GitHub Releases](https://github.com/cutalion/spotify/releases)

Extract the archive and run its installer. Substitute the archive filename for your platform.

```sh
tar -xzf spotify-linux-amd64.tar.gz
./spotify/install.sh
~/.local/bin/spotifyctl login
```

The installer puts the executable in `~/.local/bin` and registers the Spotify skill for Codex and Claude Code under their user skills directories. For another agent, copy the extracted `spotify` directory into that agent’s skills directory. The bundled `scripts/spotifyctl` can also run directly.

## Upgrade

**Codex marketplace.** Refresh the catalog, reinstall the plugin from the updated catalog, then start a new Codex session.

```sh
codex plugin marketplace upgrade cutalion
codex plugin remove spotify@cutalion
codex plugin add spotify@cutalion
```

**Claude Code marketplace.** Run these commands in a terminal, then restart Claude Code.

```sh
claude plugin marketplace update cutalion
claude plugin update spotify@cutalion
```

**Manual install.** Extract the latest archive for your platform into a fresh directory and run its `./spotify/install.sh` again. If you copied the skill into another agent, replace that agent’s `spotify` skill directory with the newly extracted copy. Restart the agent to load the updated skill. When upgrading from v1 to v2, run `spotifyctl login` once with your existing client ID. Version 2 uses a new configuration directory. [GitHub Releases](https://github.com/cutalion/spotify/releases)

## Uninstall

For a marketplace installation, run the command for your agent in a terminal, then restart the agent.

| Agent | Command |
| --- | --- |
| Codex | `codex plugin remove spotify@cutalion` |
| Claude Code | `claude plugin uninstall spotify@cutalion` |

For a manual installation with `install.sh`, remove the executable and the two skill files it installed.

```sh
rm -f "$HOME/.local/bin/spotifyctl" \
  "${CODEX_HOME:-$HOME/.codex}/skills/spotify/SKILL.md" \
  "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills/spotify/SKILL.md"
```

If you copied the entire skill into another agent, remove that copy from the agent’s skills directory. All installations share the same Spotify login by default. To also erase local credentials and transfer journals after removing your last installation, delete the `spotifyctl` configuration directory described below.

## Connect your account

On first login, create a Web API app, register the exact redirect URI `http://127.0.0.1:8888/callback`, and paste the app’s client ID at the terminal prompt. Complete consent in the browser on the same machine. The CLI uses PKCE and stores configuration and tokens under `$XDG_CONFIG_HOME/spotifyctl` (default `~/.config/spotifyctl`).

Set `SPOTIFYCTL_CONFIG_DIR` to use a different private writable directory. Run login with the same setting you use for subsequent commands. Set it to the same path in both agents to share the login. The setting changes the storage location, and Spotify commands still require network access. [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)

An existing `SPOTIPY_CLIENT_ID` environment variable overrides the saved ID. The source checkout also reads its local `.env`. Existing Spotipy token caches and transfer journals in the configuration directory are supported. Missing scopes trigger browser consent.

For a small shared app, the owner can give users the same public client ID and add their accounts to the app allowlist. A development-mode app requires Premium for its owner and normally supports up to five authorized users. [Spotify quota modes](https://developer.spotify.com/documentation/web-api/concepts/quota-modes)

## Verify or build a release

Verify the downloaded archive before extraction and check that the attestation identifies the expected release tag and workflow:

```sh
gh attestation verify "$archive" --repo cutalion/spotify
```

To independently rebuild, check out the same tag, use the exact Go version in `.github/workflows/release.yml`, and run the following on Linux with GNU tar and gzip. Compare the archive digest with the release’s SHA256SUMS. Pushing a version tag such as `v1.0.0` starts the release workflow. It compares archives from two fresh runners and smoke-tests each executable on its target platform before attesting and publishing the archives. A manual workflow run exercises the same checks and produces downloadable workflow artifacts.

```sh
release/package.sh linux amd64
sha256sum dist/spotify-linux-amd64.tar.gz
```

The two marketplace catalogs pin the same distribution tag, such as `spotify-plugin-v1.1.0`. Keep both catalogs on the last published distribution while releasing a new source tag. After CI publishes the new distribution tag, verify it with `git ls-remote --exit-code --tags origin refs/tags/spotify-plugin-vX.Y.Z`, then update both catalog refs together. After building all four platform archives, run `SPOTIFY_PLUGIN_VERSION=X.Y.Z release/plugin.sh` to reproduce `dist/spotify-plugin.tar.gz`. CI checks and attests this archive alongside the standalone archives, then publishes its contents as a separate Git tag that both clients can install. [Claude Code catalog](.claude-plugin/marketplace.json) · [Codex catalog](.agents/plugins/marketplace.json)

For a custom distribution, set `SPOTIFY_RELEASE_CLIENT_ID` when packaging to embed a public client ID. Disclose that build input to anyone reproducing the archive. The standard release prompts each user for their ID.

## Run from source

With Go 1.25 or later, run `./spotifyctl --help` or `go test ./...` from the checkout. The examples below use the checkout launcher. For a release installation, substitute the installed executable.

## Usage

```sh
./spotifyctl playlists
./spotifyctl search 'track:The Show Must Go On artist:Queen'
./spotifyctl create "Road Trip"
./spotifyctl add "Road Trip" TRACK_ID
./spotifyctl list "Road Trip"
./spotifyctl move TRACK_ID --from favorites --to "Road Trip"
./spotifyctl move-all --from "Driving" --to "Road Trip"
./spotifyctl swap "Road Trip" --from favorites
```

Replace `TRACK_ID` with an ID from search results. Use a playlist name, ID, or link to select a playlist. Names match exactly, ignoring case. Use an ID for duplicate names and `favorites` for Liked Songs. New playlists are private. Use `spotifyctl remove "Road Trip"` to remove that playlist from your Spotify library.

The single-track move above adds the track to Road Trip and removes it from Liked Songs. Both move commands require an explicit source and destination.

A swap exchanges tracks unique to each collection and keeps shared tracks in both. If a transfer is interrupted, rerun the same command to resume.

Use `./spotifyctl --help` for command syntax. For automation, put `--json` before the command, as in `./spotifyctl --json playlists`. See [SKILL.md](SKILL.md) for agent workflows.

## Playback

Spotify Premium is required for playback control. On the next sign-in, approve access to read and control playback. Commands target the active device by default. Use `./spotifyctl devices` to find a device ID and add `--device ID` to any playback control to target it.

```sh
./spotifyctl status
./spotifyctl play --playlist "Ночная передача"
./spotifyctl play --track TRACK_ID
./spotifyctl queue
./spotifyctl queue TRACK_ID
./spotifyctl pause
./spotifyctl play
./spotifyctl next
./spotifyctl previous
./spotifyctl volume 35
./spotifyctl seek 90
./spotifyctl shuffle on
./spotifyctl repeat track
```

Use `play` alone to resume. Seek positions are in seconds. Repeat accepts `off`, `track`, or `context` (the current playlist or album).

Use `queue` alone to list upcoming items. Pass a track ID, track URI or link, or an episode URI or link to append an item to the queue. Prepending, reordering, removing, and clearing queue entries require the Spotify app. [Spotify playback API](https://developer.spotify.com/documentation/web-api/reference)
