# Spotify playlist CLI

Search Spotify for a recording, add it to a playlist, or move and swap tracks between playlists and Liked Songs.

## Setup

Create a Web API app in the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard) and register the redirect URI `http://127.0.0.1:8888/callback`. Spotify requires an active Premium subscription for the owner of a development-mode app.

From the project directory, run these commands on Linux or macOS with Python 3.9 or later.

```sh
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp -n .env.example .env
```

Set `SPOTIPY_CLIENT_ID` in `.env` to your app’s client ID. Run `./spotify playlists` to sign in through your browser. The CLI uses PKCE and caches tokens under `~/.config/spotify-mover/` (or `$XDG_CONFIG_HOME/spotify-mover/`). Keep this directory and your local `.env` private.

If you signed in before the CLI added listening history, the next Spotify command opens a browser to request the added permission. Run `./spotify recent` to see up to 50 recent plays with timestamps.

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
