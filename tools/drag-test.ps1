# Drags the running flower with real mouse input and reports where its window is
# before, during and after the drag. This is the end-to-end check that the thing
# on screen is the thing the physics drives: run `pocket-petal -dry-run -trace`
# first, so a drop reports what it would have delivered and types nothing.
#
#	powershell -ExecutionPolicy Bypass -File tools/drag-test.ps1

Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class D {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int m);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L,T,R,B; }
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
  [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(int x, int y, IntPtr ignored);
  [DllImport("user32.dll", EntryPoint="WindowFromPoint")] public static extern IntPtr WindowAt(POINT p);
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
}
'@

const uint LEFTDOWN = 0x0002
const uint LEFTUP = 0x0004

function Get-FlowerHandle {
  $script:h = [IntPtr]::Zero
  $cb = [D+Proc]{ param($w, $l)
    $sb = New-Object System.Text.StringBuilder 256
    [void][D]::GetClassName($w, $sb, 256)
    if ($sb.ToString() -eq 'PocketPetalFlowerWindow') { $script:h = $w; return $false }
    return $true
  }
  [void][D]::EnumWindows($cb, [IntPtr]::Zero)
  return $script:h
}

function Get-Rect($h) {
  $r = New-Object D+RECT
  [void][D]::GetWindowRect($h, [ref]$r)
  return "$($r.L),$($r.T) $($r.R - $r.L)x$($r.B - $r.T)"
}

$h = Get-FlowerHandle
if ($h -eq [IntPtr]::Zero) { Write-Output 'FAIL: no flower window is on screen'; exit 1 }
$before = Get-Rect $h
Write-Output "flower window before drag: $before"

# the bloom's centre in the window's own coordinates: an opaque pixel, which is
# the only kind a per-pixel-alpha window will give a click to
$r = New-Object D+RECT; [void][D]::GetWindowRect($h, [ref]$r)
$grabX = $r.L + 95
$grabY = $r.T + 80
Write-Output "pressing at $grabX,$grabY"

[void][D]::SetCursorPos($grabX, $grabY)
Start-Sleep -Milliseconds 120
[D]::mouse_event($LEFTDOWN, 0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 120

$moves = @(@(0, -20), @(-60, -60), @(-160, -120), @(-300, -180))
foreach ($m in $moves) {
  [void][D]::SetCursorPos($grabX + $m[0], $grabY + $m[1])
  Start-Sleep -Milliseconds 140
  Write-Output ("  cursor $($grabX + $m[0]),$($grabY + $m[1]) -> window " + (Get-Rect $h))
}

$mid = Get-Rect $h
Write-Output "window mid-drag: $mid"

# a second of stillness so the carry physics settles over the new spot, then let go
Start-Sleep -Milliseconds 400
[D]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 300
$dropped = Get-Rect $h
Write-Output "window just after release: $dropped"

# the flower walks home once the delivery is done
Start-Sleep -Seconds 3
$after = Get-Rect $h
Write-Output "window back at rest: $after"

$r2 = New-Object D+RECT; [void][D]::GetWindowRect($h, [ref]$r2)
if ("$before" -eq "$mid") { Write-Output 'FAIL: the window never moved with the cursor' ; exit 1 }
if ("$before" -ne "$after") { Write-Output "NOTE: it did not return to exactly where it started ($before)" }
Write-Output 'PASS: the window followed the cursor while the button was down'
