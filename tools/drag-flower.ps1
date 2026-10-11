<#
  A scripted hand: press the flower, carry it to a point, let go.

  The real thing is a mouse and a human; this exists so the drag path - the glow
  that shows the target, and the delivery on release - can be tested without one.
  It finds the flower by its window class, picks a point on the bloom, and moves
  the pointer in steps the way a hand would, so the carry physics sees a real
  gesture rather than a teleport.

  Usage:
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/drag-flower.ps1 -To 600,480
    ... -To 600,480 -DryDrag     # press and release without moving
#>

param(
  [Parameter(Mandatory = $true)][string]$To,
  [int]$Steps = 26,
  [int]$StepMs = 16,
  [int]$HoldMs = 220,
  [switch]$DryDrag
)

$ErrorActionPreference = 'Stop'

$signature = @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class CareDrag {
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
  [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT p);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint flags, int dx, int dy, uint data, UIntPtr extra);
  [DllImport("user32.dll")] public static extern IntPtr FindWindow(string cls, string title);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  public static RECT RectOf(IntPtr h) { RECT r; GetWindowRect(h, out r); return r; }
  // from C#, so that "no title" is a real null and not an empty string
  public static IntPtr FindClass(string cls) { return FindWindow(cls, null); }
  public static void Move(int x, int y) { SetCursorPos(x, y); }
  public static void Down() { mouse_event(0x0002, 0, 0, 0, UIntPtr.Zero); }
  public static void Up() { mouse_event(0x0004, 0, 0, 0, UIntPtr.Zero); }
}
'@
Add-Type -TypeDefinition $signature -Language CSharp | Out-Null

$flower = [CareDrag]::FindClass('PocketPetalFlowerWindow')
if ($flower -eq [IntPtr]::Zero) { throw 'the flower is not running (no PocketPetalFlowerWindow)' }
if (-not [CareDrag]::IsWindowVisible($flower)) { throw 'the flower window is not visible' }

$rect = [CareDrag]::RectOf($flower)
# the bloom sits in the upper part of the window, which is where a hand takes hold
$from = @{
  X = [int](($rect.Left + $rect.Right) / 2)
  Y = $rect.Top + [int](($rect.Bottom - $rect.Top) * 0.30)
}
$parts = $To.Split(',')
$target = @{ X = [int]$parts[0]; Y = [int]$parts[1] }

Write-Output "flower=$($rect.Left),$($rect.Top)  grab=$($from.X),$($from.Y)  to=$($target.X),$($target.Y)"
[CareDrag]::Move($from.X, $from.Y)
Start-Sleep -Milliseconds 90
[CareDrag]::Down()
Start-Sleep -Milliseconds $HoldMs

if (-not $DryDrag) {
  for ($i = 1; $i -le $Steps; $i++) {
    $t = $i / $Steps
    # ease in and out, so the carry physics gets acceleration to work with
    $eased = $t * $t * (3 - 2 * $t)
    $x = [int]($from.X + ($target.X - $from.X) * $eased)
    $y = [int]($from.Y + ($target.Y - $from.Y) * $eased)
    [CareDrag]::Move($x, $y)
    Start-Sleep -Milliseconds $StepMs
  }
}

Start-Sleep -Milliseconds 160
[CareDrag]::Up()
Write-Output 'released'
