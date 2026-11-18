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

Ask the agent to “use the Spotify plugin to show my current track.” On first use, ask it for the installed skill’s absolute `scripts/spotify` path and run that executable with `login` in your terminal. The plugin includes the Linux and macOS binaries for Intel/AMD x86-64 and ARM64. It uses the same Spotify login as the standalone skill.

## Install the skill manually

Choose the release archive for Linux or macOS (`darwin`) and your processor. Use `amd64` for Intel/AMD x86-64 and `arm64` for Apple Silicon or 64-bit ARM. Each archive contains the skill instructions and a standalone executable. [GitHub Releases](https://github.com/cutalion/spotify/releases)

### Choose the install directory

Set `skills` in your terminal for the agent you use, then run the extraction and login commands below.

| Agent | Install directory | Example request after login |
|---|---|---|
| Codex | `skills="${CODEX_HOME:-$HOME/.codex}/skills"` | In a new session, ask “Use the Spotify skill to show my current track.” |
| Claude Code | `skills="$HOME/.claude/skills"` | Run `/spotify show my current track` in Claude Code. |
| Other agents with skill discovery | Set `skills` to the agent’s documented skills directory. | Ask the agent to use the Spotify skill to show your current track. |
| Agents with local shell access only | `skills="$HOME/.local/share/agent-skills"` | Give the agent the full path to the installed `spotify/SKILL.md` and ask it to follow those instructions to show your current track. |

To install for a single Claude Code project, run `skills="$PWD/.claude/skills"` from the project root. [Claude Code skill locations](https://code.claude.com/docs/en/skills#where-skills-live)

### Extract and sign in

From the download directory, run the following in the same terminal. Substitute the archive filename for your platform.

```sh
archive=spotify-linux-amd64.tar.gz
mkdir -p "$skills"
tar -xzf "$archive" -C "$skills"
"$skills/spotify/scripts/spotify" login
```

You can check the installation with `"$skills/spotify/scripts/spotify" --json status` before trying it in your agent.

## Connect your account

On first login, create a Web API app, register the exact redirect URI `http://127.0.0.1:8888/callback`, and paste the app’s client ID at the terminal prompt. Complete consent in the browser on the same machine. The CLI uses PKCE and stores configuration and tokens under `$XDG_CONFIG_HOME/spotify-mover` (default `~/.config/spotify-mover`). Keep the token private. [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)

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

With Go 1.25 or later, run `./spotify --help` or `go test ./...` from the checkout. The examples below use the checkout launcher. For a release installation, substitute the installed executable.

## Usage

```sh
./spotify playlists
./spotify search 'track:The Show Must Go On artist:Queen'
./spotify create "Road Trip"
./spotify add "Road Trip" TRACK_ID
./spotify list "Road Trip"
./spotify move TRACK_ID --from favorites --to "Road Trip"
./spotify move-all --from "Driving" --to "Road Trip"
./spotify swap "Road Trip" --from favorites
```

Replace `TRACK_ID` with an ID from search results. Use a playlist name, ID, or link to select a playlist. Names match exactly, ignoring case. Use an ID for duplicate names and `favorites` for Liked Songs. New playlists are private.

The single-track move above adds the track to Road Trip and removes it from Liked Songs. Both move commands require an explicit source and destination.

A swap exchanges tracks unique to each collection and keeps shared tracks in both. If a transfer is interrupted, rerun the same command to resume.

Use `./spotify --help` for command syntax. For automation, put `--json` before the command, as in `./spotify --json playlists`. See [SKILL.md](SKILL.md) for agent workflows.

## Playback

Spotify Premium is required for playback control. On the next sign-in, approve access to read and control playback. Commands target the active device by default. Use `./spotify devices` to find a device ID and add `--device ID` to any playback control to target it.

```sh
./spotify status
./spotify play --playlist "Ночная передача"
./spotify play --track TRACK_ID
./spotify pause
./spotify play
./spotify next
./spotify previous
./spotify volume 35
./spotify seek 90
./spotify shuffle on
./spotify repeat track
```

Use `play` alone to resume. Seek positions are in seconds. Repeat accepts `off`, `track`, or `context` (the current playlist or album).
