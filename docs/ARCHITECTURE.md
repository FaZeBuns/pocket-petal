# Architecture

Pocket Petal is a picture with alpha that behaves like an object. It has no UI
toolkit, no browser and no runtime: the picture is posed by maths, rasterised by
our own scanline renderer, and put on screen by a layered Win32 window.

```
             Motion                  Pose                  Shape[]                bytes
  physics  ──────────▶  PoseOf  ──────────▶  BuildDrawList  ──────▶  Rasterize  ──────▶  layered window
  (a hand)              (skeleton)            (polygons+fills)         (pixels)            (BGRA premultiplied)
```

Each arrow is a pure function of the one before. Nothing reads a clock it was not
handed, nothing touches the screen to work out what it looks like, and every stage
can be rendered headlessly into a PNG or a raw buffer. That is what makes the
drawing checkable and the feel testable.

## 1. The rig — `internal/flower/rig.go`

The flower is a skeleton, not a sprite sheet.

- **The spine** is integrated from a curvature function `StemAngle(s, t, motion)`
  sampled at 34 joints. Curvature is where the character is: the base is stiff
  (`limp` grows with `s`), and the last stretch — the neck — is limp, so the bloom
  swings on it. `s` is arc length, so the joints are always connected: the stem
  cannot tear or stretch.
- **The head hangs.** The bloom is *not* welded to the last joint. It sits
  `NeckStub × headRadius` further along, in the direction the neck points
  (`Head.NeckAngle`) — so the top of the stalk is visibly carrying the flower, and
  when the neck bends, the whole bloom (petals and all) swings out with it.
- **The petals** unfurl on a golden-angle spiral, staggered so the bloom opens
  like a real one, each with its own curl, sway and length. Their tips are stored
  in the head's own frame; the head's `Tilt` (which includes the swing) rotates
  them into place.
- **`t` is time in seconds from planting**, so the same call at the same `t` is
  the same flower: the pose can be scrubbed, tested and rasterised.

## 2. The physics — `internal/flower/physics.go`

The flower behaves like a thing with weight: it lags behind the hand, keeps moving
when you stop, squashes when it lands. It is feed-forward and frame-driven:

```go
carried.Step(dt, flower.Hand{VX: vx, VY: vy, AX: ax, AY: ay, Dragging: true})
motion := carried.Motion()   // straight into PoseOf
```

Three springs, deliberately tuned:

| spring | what it does | frequency |
| --- | --- | --- |
| tether | the bloom lags and falls back behind the hand | stiff |
| whip | the stalk bends against travel and arrives late | medium |
| **neck** | the head's pendulum, barely damped | slow — it keeps swinging |

The neck is the one that makes it read as a flower: `swingStiffness 24`,
`swingDamping 1.85` (a damping ratio of about 0.19, so it swings three or four
times). Drive it with the hand's speed and direction changes, and the bloom trails
a steady drag, then throws *forward* past the stem when the hand stops dead.

## 3. The drawing — `internal/flower/draw.go`

`BuildDrawList` turns a pose into flat polygons with fills, back to front:

```
shadow → stem → neck (cup, knot, crease) → leaves → glow → calyx → petals → centre → glints
```

The whole vocabulary is three fills: solid, linear gradient, radial gradient, each
with stops that may carry their own alpha (`StopA`). That alpha is what lets the
sun halo fade out with no edge — and the same list then feeds the tray icon.

The `glow` is the one shape the app does not draw: it is a halo wider than the
flower's window, so on a desktop all that ever reaches the screen is the middle of
a soft ellipse cut off square by the window's edge — a grey blob. `frameShapes`
drops it and `flower.ApplyShine` visits the flower with real light instead.

Where the light comes from is explicit: `flower.Sun` (upper left). Every petal is
shaded by how far it faces the sun, gets a rim light nudged towards it, a sheen
down its lit side, and the bloom as a whole sits in a warm halo.

## 4. The renderer — `internal/flower/raster.go`

A scanline polygon fill with 2-3x vertical subsampling for antialiasing,
source-over compositing in straight (non-premultiplied) RGBA, and gradient
sampling in design space. Two properties matter:

- **It is bit-identical single- or multi-threaded.** The frame is cut into tiles
  (2 columns, `workers/2` rows) because a flower is a dense blob in the middle of
  a mostly empty window; tiles never overlap, so there is no locking and the
  pixels do not depend on how many cores you have. `TestParallelRenderingIsIdentical`
  asserts exactly that.
- **It is exact, not approximate.** No lookup tables: every pixel samples its
  gradient at full precision, so the same pose is the same pixels on every
  machine and a change to the drawing shows up as a change in the picture.

Measured on the machine this was built on: pose 0.02 ms, draw list 1.0 ms, and the
frame the app actually paints — the flower at 235 px in the 425x382 window it is
framed into — 18 ms over 8 tiles. That is the one number in here worth being
unhappy about: the sample is per pixel, the window is mostly air, and the
rasteriser pays for all of it. Clipping each frame's raster to the region the draw
list covers, and blitting that region into the window's bitmap, is the obvious next
move (`BenchmarkAppFrame` is in the tree so it can be argued about with numbers).

## 5. The window — `internal/win/`

Our own bindings (`api.go` declares the ~40 Win32 calls used; `x/sys/windows` has
no user32/gdi32 in it), and one layered window:

- `WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE`
- a top-down 32bpp DIB section, filled with **premultiplied BGRA**, handed over
  with `UpdateLayeredWindow(ULW_ALPHA)` once per frame — position and pixels in the
  same call, so the flower cannot be seen to lag its own bitmap
- per-pixel alpha hit testing, for free: the mouse falls through everywhere the
  picture is empty, so the window is only "there" where the flower is
- the tray icon is the same drawing at 32 px, turned into an `HICON` through
  `CreateIconIndirect` with an all-zero mask (the alpha channel does the rest)

### A window that exists and shows nothing

The flower went missing for a while with `UpdateLayeredWindow` returning success,
and it is worth writing down what it actually was, because it looked exactly like
a desktop that refuses to composite layered surfaces.

It was two lines in here.

1. **`SourceConstantAlpha` was left at zero.** Windows multiplies the per-pixel
alpha by that constant, so a zero makes the whole window fully transparent — with
no error, no missing pixels, and no way to tell from any of the `Get…` calls. The
window was visible, topmost, layered, correctly sized, in per-pixel mode, and had
`WindowFromPoint` looking straight through it (which is also what a per-pixel
window does where it *is* transparent, so the symptom was doubly misleading). It
must be 255 whenever `AlphaFormat` is `AC_SRC_ALPHA`.
2. **`SetLayeredWindowAttributes` was being called on the same window.**
Attribute-based transparency (a colour key or a constant alpha) and
`UpdateLayeredWindow` are mutually exclusive per window: once the window is in
attribute mode, the per-pixel blit fails with `ERROR_INVALID_PARAMETER`. It was
being called as a belt-and-braces "make sure it is layered" step and did the
opposite.

The lesson is in `cmd/ulwprobe`, which is kept because "the flower is not on
screen" has several entirely different causes and this tells them apart. It
blits a solid rectangle through `UpdateLayeredWindow` with every argument written
out by hand, prints what Windows said about each call, and holds it on screen to
be looked at:

```sh
go run ./cmd/ulwprobe -variant 1          # the flower's own path, plainly
go run ./cmd/ulwprobe -variant 2          # with SetLayeredWindowAttributes first
go run ./cmd/ulwprobe -variant 3          # with a screen DC as the destination
```

`cmd/probe` does the same from the other side, inside this package's own window
code, and `pocket-petal -dump frame.png` writes out the frame the window is
handed — with the alpha — which is how "not on screen" is told apart from "drawn
fully transparent" without any guessing at all.

### A surface bigger than the picture in it

`UpdateLayeredWindow` copies `psize` pixels from `pptSrc` of the source DC, which
means a layered window can be allocated once at the size of everything that will
ever be drawn in it while each frame shows — and pays for — only the rectangle
that has something in it. `BlitRegionAt` is that call; the burst leans on it
entirely.

It was checked against the screen before anything was built on it, because a
sub-rectangle blit that silently reads from the corner of the bitmap would look
like a working flower and a broken explosion: a 40x40 region of pure blue written
at 100,100 of a 512x512 surface and blitted to 300,200 put `0,0,255` at 320,220
and the desktop's own grey at 295,220, with the window reporting itself 40x40 at
300,200. `TestOnlyTheRegionOfABitmapThatIsBlittedIsShown` keeps the geometry of
that in the suite.

```sh
go run ./cmd/probe -mode paint    # GDI painting    -> proves the window is on screen
go run ./cmd/probe -mode alpha    # per-pixel alpha -> the flower's own path
go run ./cmd/probe -mode opaque   # the same bitmap, no alpha
```

## 6. The app — `internal/buddy/`

A frame loop, and the honesty that comes with owning the window:

- **The window is sized around the flower, not the other way round.**
  `flower.FrameFor` simulates the carry through the hardest drags the physics
  allows, unions the outline of every pose it passes through, and hands back a
  window big enough for all of them *at the size the flower is meant to be*. A
  window is a rectangle of air, so nothing has to be shrunk to stop a fast drag
  slicing the top off the bloom. `TestFrameForKeepsTheFlowerTheSizeItWasAsked`
  is the contract; `cmd/boxcheck` prints the numbers.
- **The drag holds one point of the flower, and that point is the hand.** You
  pick the flower up by a *place on it* — `NearestStem` names the piece of stalk
  under the cursor as a fraction `s` of the way up it (`internal/flower/stem.go`),
  or the top of the stem if the bloom is what you grabbed — and every frame the
  window is placed so that place lands exactly on the cursor:

  ```go
  px, py := tr.ToPixel(anchor(pose))     // where the held piece is in the window
  window = hand − (px, py)               // so the held piece is on the hand
  ```

  The whole feel falls out of that. The pose *moves* the held piece — the stem
  bends, the whip sends it sideways, the neck swings the bloom — so the window
  compensates and the rest of the flower sweeps around the point you are
  holding. Standing still the anchor is the base of the stem (the rig's origin),
  which is what a flower stands on: the window is then a constant and the flower
  sways inside it.

  **It is the same share of the desk on every screen.** Everything in here — the
  flower's box, the margin it stands off the sides and the top with, the settings
  panel's plate, its rows and its type — is a measurement on a 1080p screen, and all
  of it is drawn at the scale that screen's own height asks for
  (`deskScaleFor` in `place.go`: 0.67 on a 720p screen, 1.0 on a 1080p one, 1.33 on a
  2K, 2.0 on a 4K, capped at 4.0 for an 8K). It is the screen's height and not
  Windows' DPI setting on purpose: the flower is not text, and a DPI-only rule means
  changing the *resolution* under the flower — the same monitor, the same 100%, a
  different number of pixels — leaves it at the same pixel size and therefore twice
  the flower. On the setups where the two rules agree (4K at 200%, 8K at 400%, 1080p
  at 100%, which is how those screens are almost always used) they give the same
  answer. `TestTheFlowerIsTheSameShareOfTheDeskAtEveryResolution` holds the whole
  table of screens to it, and `TestThePanelIsDrawnAtTheFlowersScale` holds the plate
  to the flower's own scale rather than a fixed 316 pixels — which would be a
  postage stamp beside a flower that had grown with an 8K screen.

  **A screen change is noticed, and everything is worked out again.** The flower's
  window hears `WM_DISPLAYCHANGE`, `WM_SETTINGCHANGE` and `WM_DPICHANGED`
  (`Buddy.onMessage`), and `refitDisplay` decides whether anything a person could see
  changed: the work area and the scale are read again, the flower is re-framed and its
  window rebuilt for the size that comes out, and it is put back on the new ground —
  the same corner, or a spot somebody picked at the same *share* of the desk
  (`sameSpot`). An open settings panel is re-scaled with it, laid out again at the new
  size and put back inside a work area that is itself a different size
  (`menuState.rescale`). A message that changes nothing moves nothing:
  `TestTheScreenChangingUnderTheFlowerPutsItBackInPlace`,
  `TestASpotSurvivesTheScreenChangingSize` and `TestAnOpenPanelRescalesWithTheScreen`
  are the contract.

  **It stands on the taskbar.** The work area stops where the taskbar starts, so
  its bottom edge *is* the top of the bar — and the flower's feet are put exactly
  there (`placeHome`), with no margin, measured from the flower's own outline
  rather than from the base of the stem, so whatever shape it is grown into stands
  on the same line: a taller flower is taller, not higher off the ground. It used to
  stand a 26-pixel margin above it, which read as a flower floating in the corner.
  `TestTheFlowerStandsWhereItIsPut` checks both halves — inside the work area, and
  the lowest point of the flower *is* the ground. (Temporarily putting the margin
  back makes it fail by exactly the 26 px: `1013.7` against `1040`.)

  Two things this deliberately does *not* do. It does not add the carry's lag to
  the window: a window that trails the cursor is a picture sliding around, and the
  lag is already in the pose as lean and whip. And it does not anchor anything the
  pose moves when the flower is *not* being carried — that would make the window
  swim every time the wind blew. `TestTheGrabbedStemStaysUnderTheHand`,
  `TestStandingStillTheWindowDoesNotFollowTheSway` and
  `TestHoldingTheBloomLeavesTheBloomFreeToSwing` in
  `internal/buddy/drag_test.go` are the contract; the flower being under the
  cursor is arithmetic, so it is tested rather than eyeballed.

  **Letting go moves nothing, and that is one line.** The anchor changes the
  moment the hand opens — held piece while carried, base of the stem once
  standing — so a flower released halfway up its stem used to be dragged down to
  the cursor: the hand *became* the base of the stem and the whole flower leapt up
  with its feet under the mouse. `Buddy.letGo` (`internal/buddy/deliver.go`)
  reconciles them by making the hand the base of the stem at the instant of the
  release, which is the same rule from the other side: the anchor still lands on
  the hand, so nothing moves at all. The walk home starts the same way for the same
  reason. `TestLettingGoLeavesTheFlowerWhereItIs` is the property, checked at every
  place a hand can take hold — and `tools/drag-check.ps1` checks it against the
  running exe with real mouse input, reading where the base of the stem is from the
  `-state` file either side of the `mouse_up` (see "There is no console" below).
- **The ring is painted once per size, never per frame.** The warm rectangle
  around the window the flower would land on is a second layered window, and over
  a maximised chat its bitmap is eight megabytes: `ringRGBA` measures 19 ms for a
  1936x1056 ring, and it used to be called on *every frame* of a drag, with the
  premultiplied blit behind it. That is the whole of the stutter that appeared
  exactly when the flower came over something worth dropping it on — over a
  browser or an empty desktop the ring is hidden and the same drag is smooth. So
  the picture is kept, and the last few sizes with it: a frame in which nothing
  has moved does nothing at all (`TestTheRingCostsNothingWhileTheTargetHoldsStill`
  asserts it, `BenchmarkRingRGBA` keeps the cost visible), and the look under the
  cursor is taken at most every 40 ms rather than 60 times a second.
- **Only a chat or a terminal is a drop target** (`target.Deliverable`).
  A browser is the tricky case: a name in a browser title is evidence of almost
  nothing, so a *page* only counts when it is one of the title's own segments,
  whole — "ChatGPT" and "Flower physics - ChatGPT" are the chat, while "Why
  ChatGPT lies - The Verge" and "bash - Google Search" are pages that mention a
  word. Nor can a browser ever be a terminal: a page about `bash`, `wsl` or
  `ubuntu` used to light up as somewhere a note could be typed. Everything else —
  an editor, a document, the shell itself — is a clipboard delivery, which costs
  nothing. The ring is drawn only while the cursor is *inside* that window, and
  the drop takes its own look rather than the last throttled one, so the note goes
  where the flower was let go over.
- **The note is made, not written.** A delivery used to type one fixed sentence,
  which reads as a machine the second time you see it. `target.Note` builds the
  message instead, out of the word banks and sentence shapes in
  `internal/target/notes.go`: a compliment assembled fresh every time, and then the
  clause that makes the whole feature work — *"continue where you last left off"* —
  at the end of **every** note, because that is the instruction that keeps a
  long-running assistant going. The banks are split by where in a sentence they
  read (adjectives describe you, qualities are things you have, works are things
  you do, verbs are read with "you" after them), which is what keeps the sentences
  grammatical rather than merely different. The space is **3.9 million** notes from
  38 shapes, and each shape is drawn evenly so the *shape* of the compliment varies
  too — drawing in proportion to size was tried and makes the two biggest shapes two
  thirds of everything, which is the repetition it was meant to avoid.
  `TestTheNoteSpaceIsInTheMillions`, `TestTheNotesDoNotRepeatThemselves`,
  `TestEveryNoteEndsWithTheClauseThatKeepsTheChatGoing` and
  `TestTheLongestNoteFitsWhatCanBeTyped` are the contract (the last works out the
  worst case from the longest word in each bank rather than by sampling).
- **Release gives the bloom its landing** (`Impact`) and then the delivery: work
  out the target window, interrupt it if the settings allow, paste the note, press
  Enter — all off the frame loop, in order, with small waits.
- **The burst flies in a surface sized to its flight, and a frame draws only the
  part the petals are in.** This is the one animation that cannot be framed by the
  flower's window: a burst is thrown from the middle of the bloom, spreads out,
  sinks and fades, so it is a *cloud that drifts down and out*, and a fixed square
  centred on the bloom is not where it goes. It used to be exactly that — a 620
  design-unit square — and a petal that outran it was sliced off square at the
  bottom and the sides, which is precisely what a person watching the explosion
  sees.

  So the box is measured rather than guessed: `flower.Burst.Flight` flies a copy of
  the burst to its end with the same `Step` the app uses, unions every frame of it,
  and returns the box the whole flight will stay inside — scaling every piece's
  speed down, together, if the burst would reach further than `burstReach` (a
  flower flung across the desk gets a slightly smaller explosion instead of petals
  through the edge of their own window). That box is what the surface is built
  from, in pixels (`burstSurfacePixels`): the clamp is a *ceiling*, a square two
  reaches on a side that knows nothing about which way the petals went, and a
  surface built from it is mostly air. Building from the measured flight is what
  lets the explosion be big. The surface is allocated once and kept; each frame
  rasterises only the box the pieces are in right now and hands it to
  `win.Window.BlitRegionAt`, which writes it into the corner of the surface and
  resizes the window to show exactly that rectangle. Measured for the flower as it
  ships: a plain drop flies in a 1346x1620 surface (2.2 megapixels, 9 MB,
  allocated once — *smaller* than the square it replaced), its busiest frame draws
  1.79 megapixels over 74 frames, and the cloud spans 1312x1233 pixels, where the
  square it used to fly in was 531 pixels each way. `BenchmarkBurstFrame` prices the
  widest frame at **3.0 ms**, against the flower's own frame at ~16.6 ms
  single-threaded.

  `TestTheBurstNeverDrawsOutsideTheBoxItIsGiven` (flower),
  `TestThePetalSurfaceIsSizedFromTheFlight` and `TestEvenAHardFlingFitsItsSurface`
  (buddy) are the contract, and all three are sweeps over the throws the physics
  allows.

  The throw is tuned for **area, not for the size of the pieces**. What a person
  sees when a flower goes off is how far the petals travel, so the speeds are high
  (`burstSpeedMax` 1020 against 700) and the fall is gentle (`burstGravity` 760
  against 1150): a heavy fall drags the whole cloud straight down and eats the reach
  the petals needed to go outwards, which is exactly what made the old explosion
  read as a small puff with big petals in it. `flower.Confetti`'s pieces are
  therefore slightly *smaller* than they were (14-21 design units, against 17-26) —
  the area is the effect.

  The burst is also thrown *with* the hand: every piece gets the hand's own velocity
  added to its push away from the middle of the bloom (`burstMomentum`), so the
  shower carries the momentum of the drag that caused it — flick the flower right
  and the explosion goes right, which is half of why it reads as the flower going
  off rather than a decoration playing over it. `TestTheExplosionCarriesTheDrag`
  states it: right goes right, left goes left, and both go further than a flower
  let go of standing still.
- **The delivery goes off, and the flower stays whole.** `flower.Confetti` cuts a
  hundred and thirty-two small petals out of the flower's own colourway — one
  flower's colour worn by every piece, sized the same, so the burst reads as one
  gesture rather than as party confetti — and `flower.Burst` throws them from the
  middle of the bloom: a spin, a lift, gravity, its own fade. The first three
  tenths of a second are a *flash*: a soft disc of the same colour opening out of
  the bloom to a 340-unit radius and thinning as it goes. Nothing is lifted off the flower; the bloom is not left bare and does
  not grow back, because nothing left it. They fly in a window of their own
  (`win.BurstClass`), because the flower's window is only as big as the poses it
  can reach and an explosion in it would be cut off square. The flower takes the
  recoil (`Impact` and a `Flick` against the throw) so it answers its own
  explosion, and then it walks home.
- **The sun visits.** `flower.ApplyShine` adds a sweeping band of warm light and
  a few glints, weighted by the flower's own alpha, so it can only ever brighten
  flower; every fourteen seconds or so, and again on a pick-up or a landing.
- **The settings panel is drawn, not borrowed.** The shell's menus cannot be
  styled at all — grey and square, at whatever size the system font happens to be
  — so the panel is painted the way everything else here is: rounded rectangles,
  circles and round-capped lines antialiased by coverage, real type blended in
  through a GDI coverage mask (`internal/win/text.go`), all into one straight-RGBA
  buffer, page by page, with switches, ticks, swatches and sliders in it. It is a
  real window that *takes the focus* — the only one in the app that does — which is
  what gives it the keyboard and, more usefully, tells it when to close: anything
  that takes the focus away closes it.

  Its pages are Colour (four colourways, plus a wheel that turns any of them into
  any colour there is), Shape (the flower's own dials: stem length and thickness,
  wind, leaves, petal count, bloom size, how wide it opens, flutter, and how
  floppy the head is), Delivery, Looks and Position. Every page also ends with the one
  row in the panel that is not a setting: a green **Buy me a Coffee! ☕** button
  (`rowCoffee`), centred on its own plate at the foot of every page, set in the biggest
  face the panel owns so that the label fills the button rather than being lost in the
  middle of a green bar, and the only green in the app — which is what makes it read as
  a door out rather than another setting.

  **A row's colour says what the row is** (`row.ink`). The green plate is the only green;
  the ways out — the cross, Back and Quit — are the only red, so a row that leaves cannot
  be mistaken for a row that changes something; and each colourway's name on the Colour
  page is written in the colour it would make the flower, so the row is the colour rather
  than a word about it (`TestTheWaysOutAreRedAndAColourwayIsItsOwnColour`).

  The page the coffee button will one day open has not been decided — so pressing it says
  so in the trace and changes nothing at all, because a button that quietly wrote to the
  settings file would be the worst of both
  (`TestEveryPageEndsWithTheCoffeeButton`, `TestTheCoffeeButtonIsGreenWithACupOnIt`).

  Every choice is written to `%APPDATA%\pocket-petal\settings.json` as it is made
  (`internal/buddy/settings.go`, field by field, so an old file cannot zero a new
  field).

  **The pointer says what the thing under it does.** Every window with something to
  do answers `WM_SETCURSOR` (`win.SetCursor`, `win.CursorHand`, `win.CursorMove`),
  which is the one message whose answer is *inverted*: returning TRUE means "the
  cursor has been set, leave it alone", and answering zero — what every other
  handled message here answers — would let Windows put the class cursor back over
  it. So `windowProc` maps a handled `WM_SETCURSOR` to 1 and everything else to 0.
  Both the panel and the flower show the pointing hand and nothing else. On the
  panel every part of the plate does something — a row is clicked, the cross is
  clicked, and the rest of it is the handle the panel is dragged by — so the finger
  is the honest answer over all of it, and the one cursor for "there is something
  here" says both without having to be read. It costs nothing to keep it honest: the
  hand is decided from the hover the panel already tracks rather than asked of
  Windows. The flower is the same case from the other side: its window is layered,
  so the message only arrives where the picture is drawn, and everything the pointer
  can be over on it is something to pick up.

  **The panel can be moved, and has a way out of its own.** Pressing the plate
  where there is no row — the title, a heading, the background between the rows —
  takes hold of the *whole panel*, and it follows the pointer from there: that is
  how it gets put wherever the work is. The move is `win.Window.MoveTo`, which is
  `SetWindowPos` and nothing else — a blit would re-premultiply a hundred thousand
  pixels to move a rectangle whose picture has not changed, which is the panel's own
  latency all over again. It is clamped to leave a sliver of the panel on the work
  area, so it can never be dragged somewhere it cannot be fetched back from. A
  small cross in the corner (`menuState.closeRect`) puts it away without the
  keyboard, and lives in the one band of the panel that belongs to no row — it is
  redrawn on its own `chrome` flag, and `TestHoveringTheCrossRepaintsOnlyTheTitle-
  Strip` holds it to the same promise the rows make. `TestTheCrossInTheCornerPuts-
  ThePanelAway` and `TestTheBackgroundIsHowThePanelIsMoved` are the behaviour.

  The cross is drawn in the red of the ways out and a hover only makes it brighter, and
  the band the title sits in carries the title and nothing else: an accent bar used to be
  painted under the title in the flower's colour, and because the title's own band starts
  at the top of the plate the bar ran through the first few letters of the word — a line
  across a title reads as a rendering fault, not as decoration.
  `TestSettingsPanelDrawsEveryPage` requires the plate's background to hold no accent
  colour at all, which is that fault's shape.

  **The panel is pixels that are kept, not a picture that is redrawn.** It was
  measured before it was believed: a full repaint — the shadow, the plate, every
  row, through `win.Font.Mask`, which builds a device context and a bitmap for
  every line of text — cost **112 ms**, and letting go of a slider cost **270 ms**
  because the exact envelope was measured in the message handler. Both of those
  are fixed rather than worked around:

  - the shadow, the plate and the title are drawn once into `bg`, and the picture
    on screen is `bg` plus a pass over the rows;
  - anything that changes marks a *row* (`menuState.mark`), so a hover is a band
    copy and one row — **2.0 ms** — and a dragged slider is the row's number,
    redrawn — **4.3 ms** including regrowing the flower;
  - `win.Font` remembers the masks it has made (bounded, and shared: a caller may
    read one and must not draw into it), so laying the page out again is not a
    trip through GDI per label;
  - the exact envelope is measured on its own goroutine and applied by the frame
    loop when it arrives (`Buddy.requestFrame`), so releasing a slider is
    **6.0 ms** instead of a third of a second;
  - the panel takes the focus when it appears, not on every repaint — and a panel
    that has *changed size* really does appear again: a layered window's bitmap is
    fixed when it is made, so a new page is a new window, and a new window that is
    created, blitted into and never shown is one with a picture in it that sits on
    top of everything and is invisible. That was the bug that made opening a
    sub-page look like the click had been swallowed; `shown` is cleared with the
    window, so the replacement is shown and re-focused like the first one was.
    `TestClickingASubPageOpensItAndLeavesAPanelYouCanSee` is the check, against a
    real popup window (`win.Window.Visible`).

  `internal/buddy/bench_test.go` keeps all of it honest — `BenchmarkPanelHover`,
  `BenchmarkSliderDrag`, `BenchmarkReleaseASlider`, `BenchmarkExactReframe` (the
  measurement itself, still a third of a second: that is why it moved) — and
  `TestAHoverRepaintsItsOwnRowAndNothingElse` pins the incremental path down to the
  pixel: everything outside the hovered row's band is byte for byte what it was.
- **The shape dials are multipliers of the flower as it ships.** They reach
  `flower.Params` where the rig is grown (`rigOptions`), so the stem, the petals
  and the leaves are *that shape* rather than a drawing being stretched; the colour
  wheel turns the whole palette in HSL (`flower.HueTurn`), keeping every colour's
  lightness and saturation so all the shading still lands where it was designed to,
  and turning the stem and leaves a fifth as far so a violet stem cannot happen.
- **A slider drags a flower, not a form.** Every mouse move regrows the plant and
  reframes it, and the two costs are kept apart on purpose: `reframeFast` scales
  the envelope measured for the flower as it ships — one pose, about a
  millisecond — where the exact `FrameFor` drives the whole carry model and costs
  about a third of a second. So the preview follows the pointer; the window, the
  tray icon and the settings file catch up on mouse-up; and the exact envelope is
  measured *off the frame loop* while the flower keeps running, then applied when
  it arrives.
- **There is no console.** The app is linked `-H=windowsgui` (`tools/build-release.sh`),
  which is what stops Windows giving a desktop flower a black box of its own: it
  used to open one on every run, it was there for the whole life of the process, and
  closing it killed the flower — three symptoms, one linker flag. The cost is that
  the few flags that *print* something would print into nothing, so a run that is
  going to say anything borrows the console of whatever started it
  (`win.AttachConsole`, called only for `-version`, `-trace`, `-dry-run` and
  `-stats`) — and never otherwise, because a process attached to a console dies with
  it, which would have been a worse bug than the one being fixed. The release script
  re-reads the system field out of the built PE file and fails the build if it is
  not *GUI*: the flag is a promise, and the file is where it is kept.
- **Everything the app shows is made by the app, and it carries what it sounds
  like.** No toolkit and no installer: the tray icon is the flower's own rasteriser
  at 16 pixels (`renderIcon`), the picture on the exe is the same rasteriser at every
  size Explorer asks for (see the next bullet), and the sounds — the handful of magic
  twinkles the flower is heard with when it lands on a chat — are MP3s *embedded in
  the binary* (`assets/sounds`), not a folder beside it. The MP3 files are not
  published with the source: they are git-ignored files in `assets/sounds/pack` that
  are embedded when the release exe is built, and a build without them plays the
  synthesised chime instead.

  That leaves two audio paths, because Windows will only play a WAV out of memory. An
  MP3 is written out once to one folder under the user's temp directory and played
  from there through MCI (`mciSendString`: `open "…" type mpegvideo alias …`, then
  `play`), which is the same machinery the old Media Player control used and is
  present on every Windows; the WAV the app synthesises itself (`internal/chime` —
  three notes of a rising major triad, a soft octave under each, gain-normalised to
  0.55 of full scale and faded out over its tail) goes to `winmm` with
  `PlaySoundW(SND_MEMORY)`, the one Windows call that plays a sound that was never a
  file, and is the fallback for any machine the MP3 path cannot reach. That buffer is
  held in a package variable for as long as it plays, since Windows reads that memory
  until the sound ends and a collected buffer is heard as a chime stopping early. It
  is written byte by byte rather than shipped so that it can be *diffed and tested
  like anything else*: `chime_test.go` checks the RIFF header against the data chunk,
  the length against the tuning, and that the same sound comes out every time;
  `library_test.go` checks that every name in the list of twinkles is a file the
  binary really carries and that the file is an MP3 MCI will open; and `internal/win`
  plays one of them through MCI on whatever machine the tests are run on.

  The sound is also the one thing in the app with a *rule about when it may happen*: it
  is the sound of a note landing *in a chat*, so it is heard for a drop onto a chat the
  app recognised and nowhere else — not a terminal, where a note is also delivered, not
  a browser page, not the desktop — and it is the whole of what the app says out loud.
  Which of the twinkles it is gets drawn at random each time rather than taken in turn,
  because the note it carries is new every time and the sound of it should not be the
  one thing that never changes.
- **The file's own face is written by hand too.** An exe with no icon on it looks like
  a script that got compiled, and Windows will not complain about a malformed resource
  — it draws nothing and says nothing. So the icon is made the same way as the picture:
  `cmd/mkicon` renders the bloom at 16, 24, 32, 48, 64, 128 and 256 pixels, packs the
  small sizes as DIBs and the 256 as a PNG, writes the `.ico` for shortcuts you make,
  and writes an object file the linker picks up on its own
  (`rsrc_windows_amd64.syso`) whose single `.rsrc` section *is* the resource directory
  — the icon, the group that names its sizes, and the version stamp. The Go linker
  copies that section into the exe and fills in the addresses the object lists as
  relocations, and the split between what is relocated and what is not is the fiddly
  part: the tree's internal offsets are counted from the start of the section (as they
  are in any linker-made binary — checked against `notepad.exe`, whose directory
  entries are offsets like 0x38), while the *data entries* hold addresses only the
  linker can know. Two silent mistakes are worth writing down, because both were made
  here first: a resource lives at */type/name/language*, and a tree with the language
  second has no icon that `ExtractIconEx` can read at all, while a tree with no
  language level has an icon and *no version block* — `GetFileVersionInfoSize` returns
  0 with `ERROR_RESOURCE_TYPE_NOT_FOUND`, so the Details tab stays empty for a file
  with a perfectly good version inside it. `internal/icon`'s tests walk the tree back
  the way Windows does (and fail loudly if the levels are swapped again), and
  `tools/verify-release.ps1` asks Windows itself about the built exe: how many icons it
  reads out of it, and how many bytes of version.
- **One cursor, and it means "there is something here".** The flower and every part
  of the settings panel show the system's pointing finger (`IDC_HAND`), the same
  signal the rest of the machine uses for something you can act on — a row is
  clicked, the plate's background is dragged, the flower is picked up — and the
  four-way move cursor is deliberately *not* used: it is the signal for moving a
  *window about*, and a flower is not a window about. Only the parts of a window
  that belong to Windows itself (edges, corners, borders) get Windows' own answer,
  because a hit test that does not answer is a hit test that does not have an
  opinion. `TestThePointerSaysTheThingUnderItIsThereToBeUsed` is the contract, and
  `tools/drag-check.ps1` reads the real cursor over the real flower.
- **Diagnostics**: `-trace` prints where the window is and who owns its pixels,
  `-dump frame.png` writes the frame the window is handed, `-dry-run` works out
  what a drop *would* do and types nothing, `-state flower.json` writes where the
  flower is — the window, the base of the stem, the hand — ten times a second, so a
  script can watch a drag without looking at the screen, `-version` prints the
  settings the app is actually running with and where it read them from, and
  `tools/inspect-window.ps1`, `tools/poke.ps1`, `tools/drag-check.ps1` and
  `tools/verify-release.ps1` interrogate, drag and ship-check the live window from
  outside. `POCKETPETAL_SETTINGS=<file>` points the settings somewhere else, which is
  how those scripts get a settings file of their own instead of the one somebody is
  using.

  `cmd/look` is the one for the drop target itself: it sweeps a grid of points over
  the real screen and prints what the drag would see at each one, so "the flower
  thinks my desktop is a chat" is answered with the machine's own windows rather
  than a guess. Nothing on the list but a chat or a terminal is a bug:

  ```sh
  go run ./cmd/look -step 120      # every distinct window under the grid, marked
                                   # with the ones a drop would type into
  ```

## Working on the drawing

1. **Keep the stages pure.** `PoseOf`, `BuildDrawList` and `Rasterize` take their
   inputs as arguments and read no clock and no screen, so a change to the drawing
   is tested by calling them, not by running the app.
2. **Look at it.** `go run ./cmd/flowerlab -mode window -out look.png` renders the
   flower the way the app shows it, with no window.
3. **Redraw the pictures.** The pictures on the README are made by the app's own
   renderer; `tools/flowergifs.sh` makes all of them again, so a change to the
   flower cannot leave the front page showing a flower that no longer exists.
4. **Test before building.** `tools/build-release.sh` formats, vets and tests first,
   and refuses to build when any of them fails.
