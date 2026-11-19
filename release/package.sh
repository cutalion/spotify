#!/bin/sh
# Run on Linux with GNU tar and gzip. Paths are relative to the repository.
set -eu
cd "$(dirname "$0")/.."
os=${1:?usage: package.sh linux|darwin amd64|arm64 [output-directory]}
arch=${2:?missing architecture}
case "$os" in linux|darwin) ;; *) exit 2;; esac
case "$arch" in amd64|arm64) ;; *) exit 2;; esac
out=${3:-dist}
mkdir -p "$out"
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/spotify/scripts"
cp SKILL.md "$stage/spotify/"
install -m 755 release/install.sh "$stage/spotify/install.sh"
id=${SPOTIFY_RELEASE_CLIENT_ID:-}
case "$id" in *[!A-Za-z0-9]*) echo "invalid release client ID" >&2; exit 2;; esac
CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags="-X main.defaultClientID=$id" -o "$stage/spotify/scripts/spotifyctl" ./cmd/spotify
tar --sort=name --format=ustar --mtime=@0 --owner=0 --group=0 --numeric-owner --mode='u=rwX,go=rX' -cf "$stage/archive.tar" -C "$stage" spotify
gzip -n -c "$stage/archive.tar" > "$out/spotify-$os-$arch.tar.gz"
