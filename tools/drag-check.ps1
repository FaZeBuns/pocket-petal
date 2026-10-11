# The two things a hand has to be able to trust, checked against the running app
# with real mouse input:
#
#   * pick the flower up and *let go*, and the flower must not move. A flower
#     released halfway up its stem used to leap up with its feet under the mouse,
#     because the point the window is placed by changes the moment the hand lets go:
#     the piece being held while it is carried, the base of the stem once it is
#     standing. The flower writes where it is to a file while this runs (-state), so
#     the base of its stem can be read either side of the mouse-up - and it is held
#     still first, so what is measured is the letting go, not the whip that follows
#     a fast carry.
#   * the pointer over the flower must be the pointing finger, never the four-way
#     move cursor.
#
# The pixels to press on are found rather than assumed: a layered window only answers
# a hit test where it is opaque, so WindowFromPoint is what says "this pixel is
# flower". Nothing over the flower can be pressed through, so the check says so
# rather than passing on a drag that never happened.
#
#	tools/drag-check.ps1 dist/PocketPetal.exe
param([string]$Exe = "dist/PocketPetal.exe")

Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class D {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int m);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(POINT p);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT p);
  [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
  [DllImport("user32.dll")] public static extern IntPtr LoadCursor(IntPtr inst, IntPtr id);
  [DllImport("user32.dll")] public static extern bool GetCursorInfo(ref CURSORINFO ci);
  [DllImport("user32.dll")] public static extern int GetSystemMetrics(int i);
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
  [StructLayout(LayoutKind.Sequential)] public struct CURSORINFO { public int cbSize; public int flags; public IntPtr hCursor; public POINT pt; }
}
'@

$LEFTDOWN = 0x0002
$LEFTUP = 0x0004
$IDC_ARROW = 32512      # nothing here to do
$IDC_SIZEALL = 32646    # the four-way move cursor: the drag cursor, which must NOT be the hover
$IDC_HAND = 32649       # the pointing finger: what hovering the flower must show

function Flower {
  $script:found = [IntPtr]::Zero
  $cb = [D+Proc]{ param($h, $l)
    $pid2 = 0; [void][D]::GetWindowThreadProcessId($h, [ref]$pid2)
    if ($pid2 -eq $script:owner) {
      $sb = New-Object System.Text.StringBuilder 256
      [void][D]::GetClassName($h, $sb, 256)
      if ($sb.ToString() -eq 'PocketPetalFlowerWindow') { $script:found = $h; return $false }
    }
    return $true
  }
  [void][D]::EnumWindows($cb, [IntPtr]::Zero)
  return $script:found
}

# The flower's own account of itself: where the base of its stem is on screen, and
# whether a hand is holding it.
function State {
  for ($try = 0; $try -lt 20; $try++) {
    try { return ((Get-Content $script:state -Raw -ErrorAction Stop) | ConvertFrom-Json) }
    catch { Start-Sleep -Milliseconds 100 }
  }
  return $null
}

function Mine($h, $x, $y) {
  $p = New-Object D+POINT; $p.X = $x; $p.Y = $y
  return ([D]::WindowFromPoint($p) -eq $h)
}

# A pixel of the flower above its own base, as high up as one can be found: the top
# of the flower is the worst thing to be let go of, so that is what gets held.
# Returns nothing when nothing of the flower is exposed — something else is on top.
function FlowerPixelAbove($h) {
  $s = State
  $left = $s.windowX; $top = $s.windowY
  $right = $left + $s.width; $bottom = $top + $s.height
  foreach ($fy in @(0.35, 0.45, 0.55, 0.25, 0.6, 0.2, 0.65, 0.15, 0.7, 0.1, 0.75)) {
    $y = [int]($top + ($bottom - $top) * $fy)
    if ($y -gt $s.baseY - 80) { continue }   # it has to be well above the base to test anything
    foreach ($fx in @(0.5, 0.42, 0.58, 0.35, 0.65, 0.28, 0.72, 0.2, 0.8)) {
      $x = [int]($left + ($right - $left) * $fx)
      if (Mine $h $x $y) { return @($x, $y) }
    }
  }
  return @()
}

function CursorNow {
  $ci = New-Object D+CURSORINFO
  $ci.cbSize = [Runtime.InteropServices.Marshal]::SizeOf([type][D+CURSORINFO])
  [void][D]::GetCursorInfo([ref]$ci)
  return $ci.hCursor
}

if (-not (Test-Path $Exe)) { Write-Output "FAIL: no such exe: $Exe"; exit 1 }
$script:state = Join-Path $env:TEMP ("flower-state-" + (Get-Random) + ".json")
$p = Start-Process -FilePath $Exe -ArgumentList @('-dry-run', '-state', $script:state) -PassThru
$script:owner = $p.Id
Start-Sleep -Seconds 3
$flower = Flower
if ($flower -eq [IntPtr]::Zero) { Write-Output "FAIL: the flower is not on screen"; Stop-Process -Id $p.Id -Force; exit 1 }
$idle = State
if ($null -eq $idle) { Write-Output "FAIL: the flower is not writing its state to $script:state"; Stop-Process -Id $p.Id -Force; exit 1 }
Write-Output ("flower standing with the base of its stem at {0},{1}, window {2},{3} {4}x{5}" -f $idle.baseX, $idle.baseY, $idle.windowX, $idle.windowY, $idle.width, $idle.height)

# and the pointer over it: the pointing finger, never the drag cursor
$spot = FlowerPixelAbove $flower
if ($spot.Count -lt 2) {
  Write-Output "note: no pixel of the flower is exposed to the mouse, so the cursor could not be read"
} else {
  [void][D]::SetCursorPos([int]$spot[0], [int]$spot[1])
  Start-Sleep -Milliseconds 400
  $now = CursorNow
  $finger = [D]::LoadCursor([IntPtr]::Zero, [IntPtr]$IDC_HAND)
  $move = [D]::LoadCursor([IntPtr]::Zero, [IntPtr]$IDC_SIZEALL)
  $arrow = [D]::LoadCursor([IntPtr]::Zero, [IntPtr]$IDC_ARROW)
  if ($now -eq $finger) {
    Write-Output "PASS: the pointer over the flower is the pointing finger"
  } elseif ($now -eq $move) {
    Write-Output "FAIL: the pointer over the flower is the four-way move cursor"
  } elseif ($now -eq $arrow) {
    Write-Output "FAIL: the pointer over the flower is the plain arrow"
  } else {
    Write-Output ("FAIL: the pointer over the flower is something else (0x{0:x})" -f $now.ToInt64())
  }
}

# take hold of it, trying again if the press did not land: the flower sways, and
# something else may be over the pixel the hit test answered for a moment ago
$grabX = 0; $grabY = 0; $held = $null
foreach ($attempt in 1..6) {
  [void][D]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
  $spot = FlowerPixelAbove $flower
  if ($spot.Count -lt 2) {
    Write-Output "FAIL: nothing of the flower is exposed to the mouse, so it cannot be picked up here"
    Stop-Process -Id $p.Id -Force; exit 1
  }
  $grabX = [int]$spot[0]; $grabY = [int]$spot[1]
  [void][D]::SetCursorPos($grabX, $grabY)
  Start-Sleep -Milliseconds 200
  if (-not (Mine $flower $grabX $grabY)) { continue }
  [D]::mouse_event($LEFTDOWN, 0, 0, 0, [UIntPtr]::Zero)
  Start-Sleep -Milliseconds 250
  $held = State
  if ($held.dragging) { break }
  $held = $null
}
if ($null -eq $held -or -not $held.dragging) {
  Write-Output "FAIL: pressing on the flower did not pick it up"
  [void][D]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
  Stop-Process -Id $p.Id -Force; exit 1
}
Write-Output ("holding it at {0},{1}, which is {2}px above the base of its stem" -f $grabX, $grabY, ($idle.baseY - $grabY))

# carry it towards the middle of the screen in small steps, the way a hand moves
$toX = [int]([D]::GetSystemMetrics(0) / 2)
$toY = [int]([D]::GetSystemMetrics(1) / 2)
foreach ($step in 1..12) {
  [void][D]::SetCursorPos($grabX + [int](($toX - $grabX) * $step / 12), $grabY + [int](($toY - $grabY) * $step / 12))
  Start-Sleep -Milliseconds 45
}
# and hold it still until the flower stops swinging in the hand: while it is being
# carried the base of the stem swings about the part being held, and a flower that
# is still moving when the hand opens would be measured settling rather than being
# let go of
$held = State
$settled = $false
foreach ($wait in 1..14) {
  $was = $held
  Start-Sleep -Milliseconds 500
  $held = State
  if ([Math]::Abs($held.baseX - $was.baseX) -le 2 -and [Math]::Abs($held.baseY - $was.baseY) -le 2) { $settled = $true; break }
}
Write-Output ("it came to rest in the hand after {0} x half a second" -f $wait)

$hand = New-Object D+POINT; [void][D]::GetCursorPos([ref]$hand)
$carried = [Math]::Abs($held.baseX - $idle.baseX) + [Math]::Abs($held.baseY - $idle.baseY)
Write-Output ("held still: hand at {0},{1}, base of stem at {2},{3} - {4}px below the hand, carried {5}px" -f `
  $hand.X, $hand.Y, $held.baseX, $held.baseY, ($held.baseY - $hand.Y), $carried)
if ($carried -lt 150) {
  Write-Output "FAIL: the flower did not come with the hand, so there was nothing to let go of"
  [void][D]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
  Stop-Process -Id $p.Id -Force; exit 1
}
if (-not $settled) {
  Write-Output "FAIL: the flower never stopped swinging in the hand, so the letting go could not be measured"
  [void][D]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
  Stop-Process -Id $p.Id -Force; exit 1
}

[D]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 300
$just = State
Start-Sleep -Milliseconds 700
$later = State

$jumpX = $just.baseX - $held.baseX
$jumpY = $just.baseY - $held.baseY
Write-Output ("let go at {0},{1}: base of stem at {2},{3} just after, {4},{5} a second later" -f `
  $hand.X, $hand.Y, $just.baseX, $just.baseY, $later.baseX, $later.baseY)
if ([Math]::Abs($jumpX) -gt 3 -or [Math]::Abs($jumpY) -gt 3) {
  Write-Output ("FAIL: letting go moved the base of the stem {0}px sideways and {1}px vertically" -f $jumpX, $jumpY)
} elseif ([Math]::Abs($just.baseY - $hand.Y) -le 3 -and [Math]::Abs($just.baseX - $hand.X) -le 3) {
  Write-Output "FAIL: the base of the stem landed on the cursor, which is the leap this exists to catch"
} else {
  Write-Output "PASS: the flower stayed where it was standing when the hand let go of it"
}

Remove-Item $script:state -Force -ErrorAction SilentlyContinue
Stop-Process -Id $p.Id -Force
Write-Output "done: the flower was stopped again"
