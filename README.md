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

## Usage

```sh
./spotify playlists
./spotify search 'track:The Show Must Go On artist:Queen'
./spotify create "Road Trip"
./spotify add "Road Trip" TRACK_ID
./spotify list "Road Trip"
./spotify move "Road Trip" TRACK_ID --from favorites
./spotify move-all "Road Trip" --from "Driving"
./spotify swap "Road Trip" --from favorites
```

Replace `TRACK_ID` with an ID from search results. Use a playlist name, ID, or link to select a playlist. Names match exactly, ignoring case. Use an ID for duplicate names and `favorites` for Liked Songs. New playlists are private.

A swap exchanges tracks unique to each collection and keeps shared tracks in both. If a transfer is interrupted, rerun the same command to resume.

Use `./spotify --help` for command syntax. For automation, put `--json` before the command, as in `./spotify --json playlists`. See [SKILL.md](SKILL.md) for agent workflows.
