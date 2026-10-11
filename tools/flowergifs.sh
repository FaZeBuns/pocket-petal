#!/usr/bin/env bash
#
# Every picture the README shows, in one command.
#
# The GIFs and the stills are build output, not assets somebody drew: they come out of
# the app's own rig, carry model, burst and rasteriser (see cmd/flowergif), and this is
# the command that makes them, so a change to the flower cannot leave the pictures on the
# front page showing a flower that no longer exists.
#
#	tools/flowergifs.sh
#
# It writes:
#
#	assets/pocket-petal-hero.png   the flower at full bloom, big, for the top of the page
#	assets/gifs/drag.gif           picked up, carried to the right, and let go of
#	assets/gifs/burst.gif          the same drag, framed on the whole flight of the petals
#	assets/gifs/bloom.gif          the bloom unfurling, in its own six seconds, with the sun
#	assets/gifs/colours.gif        the four colourways, held and faded one into the next
#	assets/story-drag.png          the drag as a storyboard: the whole gesture in one picture
#
# Needs nothing but Go. It is checked in because a picture that cannot be made again is a
# picture nobody can verify.
set -euo pipefail
cd "$(dirname "$0")/.."

run() {
  echo "-- $1"
  shift
  go run ./cmd/flowergif "$@"
}

run "the flower, big"    -anim hero    -out assets/pocket-petal-hero.png
run "the drag"           -anim drag    -out assets/gifs/drag.gif
run "the drop"           -anim burst   -out assets/gifs/burst.gif
run "the bloom"          -anim bloom   -out assets/gifs/bloom.gif
run "the colourways"     -anim colours -out assets/gifs/colours.gif
run "the drag, as frames" -anim drag   -strip 6 -out assets/story-drag.png

echo
echo "== the pictures =="
ls -la assets/gifs/*.gif assets/pocket-petal-hero.png assets/story-drag.png
