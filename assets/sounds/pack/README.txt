The chimes are not part of this repository.

The official PocketPetal.exe on the releases page has its sounds inside it, but the
sound files themselves are not published here. Any .mp3 placed in this folder is
embedded into the exe the next time it is built (tools/build-release.sh).

With nothing here, the exe is built without any, and plays the small chime it
synthesises in memory instead. Everything else works the same.

Name the files you add in internal/chime/library.go: that list is what a drop draws from.
