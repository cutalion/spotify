---
name: spotify
description: Use the bundled Spotify CLI to search for recordings, manage playlists and Liked Songs, inspect recent plays and devices, and control playback.
---

# Spotify playlists and playback

Resolve `scripts/spotify` relative to the directory containing this SKILL.md. Invoke that executable by its absolute path from any working directory. In the examples below, `spotify` denotes that executable. On first use, guide the user through `spotify login` in their terminal to configure a client ID and authorize their account. Use `spotify --help` for syntax and place `--json` before the subcommand for structured results.

Discover destinations with `spotify --json playlists`. Names match exactly, ignoring case. Quote names containing spaces. If multiple playlists share a name, use the intended playlist ID. Use `favorites` for Liked Songs.

For a request such as “add The Show Must Go On by Queen to Road Trip,” run:

```sh
spotify --json search 'track:The Show Must Go On artist:Queen'
```

Match the title, artist, and requested version against the returned recordings. Pass the selected track ID to `spotify --json add "Road Trip" TRACK_ID`. Repeat for each requested destination. A result of `added: false` means the track was already present. When asked to create a playlist, `create` makes it private and returns its ID.

For transfers, specify both collections explicitly. For example, `spotify move-all --from "Driving" --to "Road Trip"` moves tracks out of Driving and into Road Trip. Use `move` for one track and `swap` to exchange both collections. A swap preserves shared tracks in both. Resume an interrupted transfer with its original arguments. A second successful swap reverses the first.

## Playback and listening history

Use `spotify --json status` for current playback and `spotify --json recent` for up to 50 recent plays with timestamps. A song absent from these results may have been played earlier.

Use `spotify play` to resume, `spotify play --playlist "Road Trip"` to start a playlist, or `spotify play --track TRACK_ID` to start a recording. Use `pause`, `next`, or `previous` for transport controls.

Playback controls target the active device by default. For a named device, match its name in `spotify --json devices` and pass its ID with `--device ID`. Playback control requires Spotify Premium.

Use `volume 35` for 35 percent, `seek 90` for 90 seconds from the start, and `shuffle on` or `shuffle off`. Repeat accepts `track`, `context` (the current playlist or album), or `off`.
