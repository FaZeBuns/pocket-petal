# Pokes the running flower with real mouse input: press, drag fast, screenshot
# mid-drag, drop, screenshot the light that follows. Run the app with
# `pocket-petal -dry-run -trace` first, so a drop writes nothing into whatever is
# underneath.
#
#	powershell -ExecutionPolicy Bypass -File tools/poke.ps1

param(
  [string]$Point = "",      # x,y of a pixel of the flower to press on
  [string]$Out = ".poke"
)

Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class P {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int m);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L,T,R,B; }
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
}
'@

Add-Type -AssemblyName System.Drawing

const uint LEFTDOWN = 0x0002
const uint LEFTUP = 0x0004

function Shot($name, $x, $y, $w, $h) {
  $bmp = New-Object System.Drawing.Bitmap $w, $h
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($x, $y, 0, 0, (New-Object System.Drawing.Size($w, $h)))
  $bmp.Save("$PSScriptRoot\..\$name.png", [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose()
  Write-Output "saved $name.png"
}

if ($Point -eq "") {
  # Press on a pixel the app itself says is flower. A layered window only takes
  # a click where it is opaque, so the app's own trace line ('opaque pixel at
  # x,y') is the one place worth aiming at; a hard-coded point stops being
  # flower the moment the window moves.
  $log = Join-Path $PSScriptRoot '..\.app.log'
  if (Test-Path $log) {
    $hits = Select-String -Path $log -Pattern 'opaque pixel at (\d+),(\d+)' -AllMatches | ForEach-Object { $_.Matches } | Select-Object -Last 1
    if ($hits) { $Point = "$($hits.Groups[1].Value),$($hits.Groups[2].Value)" }
  }
  if ($Point -eq "") { $Point = "1779,986" }
}
$parts = $Point.Split(',')
$startX = [int]$parts[0]
$startY = [int]$parts[1]

$script:h = [IntPtr]::Zero
$cb = [P+Proc]{ param($w, $l)
  $sb = New-Object System.Text.StringBuilder 256
  [void][P]::GetClassName($w, $sb, 256)
  if ($sb.ToString() -eq 'PocketPetalFlowerWindow') { $script:h = $w; return $false }
  return $true
}
[void][P]::EnumWindows($cb, [IntPtr]::Zero)
if ($script:h -eq [IntPtr]::Zero) { Write-Output 'FAIL: the flower window is not on screen'; exit 1 }
$r = New-Object P+RECT; [void][P]::GetWindowRect($script:h, [ref]$r)
Write-Output ("flower window at {0},{1} {2}x{3}, pressing at {4},{5}" -f $r.L, $r.T, ($r.R - $r.L), ($r.B - $r.T), $startX, $startY)

[void][P]::SetCursorPos($startX, $startY)
Start-Sleep -Milliseconds 150
[P]::mouse_event($LEFTDOWN, 0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 120

# a fast, jagged carry: the pose that used to slice the petals off
$path = @(@(1200, 420), @(760, 640), @(1420, 300), @(900, 760))
foreach ($p in $path) {
  [void][P]::SetCursorPos($p[0], $p[1])
  Start-Sleep -Milliseconds 90
}
Start-Sleep -Milliseconds 120
Shot "$Out-drag" 600 200 900 700

[P]::mouse_event($LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 260
Shot "$Out-shine-a" 600 200 900 700
Start-Sleep -Milliseconds 320
Shot "$Out-shine-b" 600 200 900 700
Start-Sleep -Milliseconds 400
Shot "$Out-shine-c" 600 200 900 700
Write-Output 'done'
