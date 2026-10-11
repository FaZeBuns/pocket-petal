#!/usr/bin/env bash
#
# One command: everything the flower is, in a single exe you can put on the
# releases tab.
#
# The flags are the whole point of having a script rather than a line in a README,
# because each one is a promise the exe has to keep:
#
#   -H=windowsgui   no console window, ever. A double-clicked flower owns nothing
#                   black, and closing the terminal it was launched from cannot
#                   take the flower down with it. This is the difference between a
#                   desktop app and a command that happens to draw.
#   -s -w           the symbol table and the debug tables stripped: most of the
#                   file size, and none of the behaviour.
#   -trimpath       no build-machine paths baked in, so two machines building the
#                   same commit make the same bytes.
#   CGO_ENABLED=0   no C runtime to be missing: it runs on a bare Windows.
#
# Nothing else is needed to run it. There is no installer, no runtime, no asset
# folder: the flower is drawn by its own rasteriser, the tray icon is drawn by the
# same engine at 16 pixels, the chimes (the MP3s in assets/sounds/pack, if this machine
# has them) are embedded in the exe, and the settings file is the only thing it ever
# writes.
#
# It is checked before it is built, because a release that does not pass its own
# tests is not a release.
#
#	tools/build-release.sh              # the version the source carries
#	tools/build-release.sh 1.2.0        # or one named here
#
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-$(sed -n 's/^[[:space:]]*Version = "\(.*\)"/\1/p' internal/buddy/buddy.go)}"
SLUG="$(sed -n 's/^[[:space:]]*Slug[[:space:]]*= "\(.*\)"/\1/p' internal/buddy/buddy.go)"
NAME="$(sed -n 's/^[[:space:]]*Name[[:space:]]*= "\(.*\)"/\1/p' internal/buddy/buddy.go)"
if [ -z "$VERSION" ] || [ -z "$SLUG" ]; then
  echo "build-release: could not read the version or the slug out of internal/buddy" >&2
  exit 1
fi

#
# The exe's own name never carries a version: it is always PocketPetal.exe, so a
# shortcut, a pinned taskbar icon or a startup entry survives an update instead of
# pointing at a file that is suddenly from last month. The version lives in exactly
# one place instead — the GitHub release's tag — which is also why the release
# workflow below needs no change to ship this: it already uploads whatever is in
# dist/*.exe, and there is now only ever one.
OUT="dist/PocketPetal.exe"

echo "== $NAME $VERSION =="
echo "-- gofmt"
UNFORMATTED="$(gofmt -l main.go internal cmd 2>/dev/null || true)"
if [ -n "$UNFORMATTED" ]; then
  echo "build-release: these files are not gofmt'd:" >&2
  echo "$UNFORMATTED" >&2
  exit 1
fi

echo "-- go vet"
go vet ./...

echo "-- go test"
go test ./...

GOARCH="${GOARCH:-amd64}"

# The icon and the version stamp are not drawn at run time: they are resources in the
# binary, and the file they come out of is an object the linker picks up on its own
# (see cmd/mkicon). Which means they are made here, before the build, from the same
# flower and the same version as everything else.
echo "-- icon and version stamp"
go run ./cmd/mkicon -version "$VERSION" -arch "$GOARCH" \
  -ico "assets/${SLUG}.ico" -syso "rsrc_windows_${GOARCH}.syso"

n=$(ls assets/sounds/pack/*.mp3 2>/dev/null | wc -l | tr -d ' ')
echo "-- sounds embedded: $n"

echo "-- build $OUT"
mkdir -p dist
CGO_ENABLED=0 GOOS="${GOOS:-windows}" GOARCH="$GOARCH" go build \
  -trimpath \
  -ldflags "-s -w -H=windowsgui -X main.version=${VERSION}" \
  -o "$OUT" .

# and say what was made, in the terms a release is judged by: how big, how it is
# identified, and — the one that matters — that it is a *windowed* binary, since a
# console subsystem exe would bring the black box back
size=$(wc -c < "$OUT" | tr -d ' ')
sha=$(sha256sum "$OUT" | cut -c1-16)
# the PE header, read out of the file itself: 0x3C holds where the header starts,
# and the subsystem is two bytes 0x5C into it. 2 is WINDOWS_GUI, 3 is the console
pe=$(od -An -tu4 -j 60 -N 4 "$OUT" | tr -d ' ')
subsystem=$(od -An -tu2 -j $((${pe:-0} + 92)) -N 2 "$OUT" | tr -d ' ')
if [ "$subsystem" != "2" ]; then
  echo "build-release: the exe is subsystem $subsystem, and a desktop app must be 2 (GUI):" >&2
  echo "               a console subsystem binary is the black window this script exists to avoid" >&2
  exit 1
fi
echo "== ${OUT} =="
echo "   $(echo "$size" | awk '{printf "%.1f MB", $1/1048576}')  sha256 $(echo "$sha")…  subsystem gui"
echo "   nothing else is needed beside it: copy it anywhere and double-click it"
