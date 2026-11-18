#!/bin/sh
# Publish the verified archive tree at a distribution-only tag.
set -eu
version=${1:?source tag required}
archive=${2:?plugin archive required}
ref="spotify-plugin-$version"
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
tar -xzf "$archive" -C "$stage"
cd "$stage/spotify-plugin"
test "v$(jq -er .version .codex-plugin/plugin.json)" = "$version"
test "v$(jq -er .version .claude-plugin/plugin.json)" = "$version"
git init -q
git add -- .
tree=$(git write-tree)
git remote add origin "https://github.com/${GITHUB_REPOSITORY:?}.git"
gh auth setup-git
if git ls-remote --exit-code origin "refs/tags/$ref" >/dev/null; then
  git fetch --depth=1 origin "refs/tags/$ref"
  test "$tree" = "$(git rev-parse 'FETCH_HEAD^{tree}')"
else
  status=$?
  test "$status" -eq 2
  git -c user.name=github-actions -c user.email=github-actions@users.noreply.github.com commit -qm "Package Spotify plugin from $version ($GITHUB_SHA)"
  git push origin "HEAD:refs/tags/$ref"
fi
