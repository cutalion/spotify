---
name: spotify
description: Use this repository’s Spotify CLI to search for recordings and manage the user’s playlists and Liked Songs.
---

# Spotify playlist management

Run commands from this directory with the project virtual environment active. Use `./spotify --help` for syntax and place `--json` before the subcommand for structured results.

Discover destinations with `./spotify --json playlists`. Names match exactly, ignoring case. Quote names containing spaces. If multiple playlists share a name, use the intended playlist ID. Use `favorites` for Liked Songs.

For a request such as “add The Show Must Go On by Queen to Road Trip,” run:

```sh
./spotify --json search 'track:The Show Must Go On artist:Queen'
```

Match the title, artist, and requested version against the returned recordings. Pass the selected track ID to `./spotify --json add "Road Trip" TRACK_ID`. Repeat for each requested destination. A result of `added: false` means the track was already present. When asked to create a playlist, `create` makes it private and returns its ID.

For transfers, specify both collections explicitly. For example, `./spotify move-all --from "Driving" --to "Road Trip"` moves tracks out of Driving and into Road Trip. Use `move` for one track and `swap` to exchange both collections. A swap preserves shared tracks in both. Resume an interrupted transfer with its original arguments. A second successful swap reverses the first.
