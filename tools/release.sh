#!/usr/bin/env bash
#
# Build the release exe on this machine and put it on a GitHub release with its
# checksum.
#
# It is built here, not by a workflow, because the chimes are not in the repository:
# they are git-ignored files in assets/sounds/pack that the build embeds in the exe, and
# a build on a clean checkout has none. So the exe on the releases page has to come from
# a machine that has them.
#
#	tools/release.sh 0.1.0        # builds, tags v0.1.0 and uploads
#
# It refuses to run unless the source being built is exactly what is on GitHub, so a
# release is never built from changes nobody can see.
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:?usage: tools/release.sh <version, e.g. 0.1.0>}"
tag="v${version#v}"
version="${tag#v}"

if [ -n "$(git status --porcelain)" ]; then
  echo "release: the working tree has uncommitted changes" >&2
  exit 1
fi
git fetch -q origin main
if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
  echo "release: HEAD is not origin/main: push (or pull) first" >&2
  exit 1
fi

sounds=$(ls assets/sounds/pack/*.mp3 2>/dev/null | wc -l | tr -d ' ')
if [ "$sounds" -eq 0 ]; then
  echo "release: assets/sounds/pack has no MP3s, so this exe would have no chimes" >&2
  exit 1
fi
echo "-- $sounds chimes will be embedded"

bash tools/build-release.sh "$version"

( cd dist && sha256sum PocketPetal.exe > SHA256SUMS.txt && cat SHA256SUMS.txt )

if gh release view "$tag" >/dev/null 2>&1; then
  gh release upload "$tag" dist/PocketPetal.exe dist/SHA256SUMS.txt --clobber
else
  gh release create "$tag" dist/PocketPetal.exe dist/SHA256SUMS.txt \
    --target main --title "$tag" --generate-notes
fi
