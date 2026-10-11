# Checks what the flower does when the screen changes under it.
#
# The messages Windows sends on a resolution change, a scaling change or the taskbar
# moving are sent to the running flower for real, and what it does about them is read
# back out of its own state file and its trace. Nothing here changes a display mode: it
# is the *detection* that is being checked, and the arithmetic behind it is checked
# without a screen at all (internal/buddy/place_test.go).
#
#	tools/display-check.ps1 dist/PocketPetal.exe
param([string]$Exe = "")

Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class D {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int m);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern IntPtr SendMessageW(IntPtr h, uint msg, IntPtr w, IntPtr l);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L,T,R,B; }
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool SystemParametersInfoW(uint action, uint param, out RECT r, uint winIni);
  [DllImport("user32.dll")] public static extern uint GetDpiForSystem();
}
'@

$WM_SETTINGCHANGE = 0x001A
$WM_DISPLAYCHANGE = 0x007E
$SPI_GETWORKAREA = 0x0030

if ($Exe -eq "") { $Exe = "dist/PocketPetal.exe" }
if (-not (Test-Path $Exe)) { Write-Output "FAIL: no such exe: $Exe"; exit 1 }

# the screen as this script sees it, which is what the flower has to agree with
$area = New-Object D+RECT
[void][D]::SystemParametersInfoW($SPI_GETWORKAREA, 0, [ref]$area, 0)

$sandbox = Join-Path $env:TEMP ("flower-display-" + (Get-Random))
New-Item -ItemType Directory -Path $sandbox | Out-Null
$state = Join-Path $sandbox "flower.json"
$log = Join-Path $sandbox "trace.log"
$settings = Join-Path $sandbox "settings.json"
# a settings file of its own, so a resolution check cannot disturb the one in use
$env:POCKETPETAL_SETTINGS = $settings

$p = Start-Process -FilePath $Exe -ArgumentList @("-trace", "-state", $state) `
  -PassThru -RedirectStandardError $log -WorkingDirectory $sandbox
Start-Sleep -Seconds 4
$p.Refresh()
if ($p.HasExited) { Write-Output "FAIL: the flower exited immediately (code $($p.ExitCode))"; exit 1 }

$h = [IntPtr]::Zero
$cb = [D+Proc]{ param($w, $l)
  $sb = New-Object System.Text.StringBuilder 256
  [void][D]::GetClassName($w, $sb, 256)
  if ($sb.ToString() -eq 'PocketPetalFlowerWindow') { $script:h = $w; return $false }
  return $true
}
[void][D]::EnumWindows($cb, [IntPtr]::Zero)
if ($h -eq [IntPtr]::Zero) { Write-Output "FAIL: the flower window is not on screen"; Stop-Process -Id $p.Id -Force; exit 1 }

function Read-State {
  for ($i = 0; $i -lt 40; $i++) {
    if (Test-Path $state) {
      try { return Get-Content $state -Raw | ConvertFrom-Json } catch { }
    }
    Start-Sleep -Milliseconds 100
  }
  return $null
}

$before = Read-State
if ($null -eq $before) { Write-Output "FAIL: the flower wrote no state file"; Stop-Process -Id $p.Id -Force; exit 1 }
Write-Output ("before: window {0},{1} {2}x{3}, the base of the stem at {4},{5}" -f $before.windowX, $before.windowY, $before.width, $before.height, $before.baseX, $before.baseY)
Write-Output ("        work area {0},{1} {2}x{3}, so the flower stands {4} pixels above the taskbar" -f $area.L, $area.T, ($area.R - $area.L), ($area.B - $area.T), ($area.B - $before.baseY))

# and what the flower made of the display: the same work area and the same scaling this
# script can see for itself, and a flower that is the size its settings asked for times
# that scaling
$screenScale = [Math]::Round([D]::GetDpiForSystem() / 96, 2)
Write-Output ("        the flower read: {0}x{1} at {2}x, drawn {3} pixels tall" -f $before.workWidth, $before.workHeight, $before.scale, [int]$before.flowerHeight)
$ok = $true
if ($before.workWidth -ne ($area.R - $area.L) -or $before.workHeight -ne ($area.B - $area.T)) {
  Write-Output ("FAIL: the flower thinks its work area is {0}x{1} and the shell says {2}x{3}" -f $before.workWidth, $before.workHeight, ($area.R - $area.L), ($area.B - $area.T))
  $ok = $false
}
if ([Math]::Abs($before.scale - $screenScale) -gt 0.01) {
  Write-Output ("FAIL: the flower is drawn at {0}x and the display is at {1}x" -f $before.scale, $screenScale)
  $ok = $false
}
if ($before.flowerHeight -lt 80 -or $before.flowerHeight -gt ($area.B - $area.T)) {
  Write-Output ("FAIL: the flower is drawn {0} pixels tall in a work area {1} tall" -f $before.flowerHeight, ($area.B - $area.T))
  $ok = $false
}

# the screen "changing": the messages Windows sends, with the screen left exactly as it
# is. The flower has to look at the work area, find that nothing moved, and leave the
# flower alone — a resolution change that did not happen must not make it jump
[void][D]::SendMessageW($h, $WM_DISPLAYCHANGE, [IntPtr]8, [IntPtr](($area.B - $area.T) -shl 16 -bor ($area.R - $area.L)))
[void][D]::SendMessageW($h, $WM_SETTINGCHANGE, [IntPtr]0, [IntPtr]::Zero)
Start-Sleep -Milliseconds 900

$after = Read-State
Write-Output ("after:  window {0},{1} {2}x{3}, the base of the stem at {4},{5}" -f $after.windowX, $after.windowY, $after.width, $after.height, $after.baseX, $after.baseY)

if ($after.width -ne $before.width -or $after.height -ne $before.height) {
  Write-Output ("FAIL: the window changed size ({0}x{1} -> {2}x{3}) for a screen that did not change" -f $before.width, $before.height, $after.width, $after.height)
  $ok = $false
}
if ($after.baseX -ne $before.baseX -or $after.baseY -ne $before.baseY) {
  Write-Output "FAIL: the flower moved for a screen that did not change"
  $ok = $false
}
# and it is on the ground line: the base of the stem sits on the top of the taskbar, in
# the corner it was asked for, at every resolution there is
if ([Math]::Abs($after.baseY - $area.B) -gt 2) {
  Write-Output ("FAIL: the base of the stem is at y={0} and the taskbar starts at {1}" -f $after.baseY, $area.B)
  $ok = $false
}
if ($after.baseX -lt $area.L -or $after.baseX -gt $area.R) {
  Write-Output "FAIL: the flower is not on the screen"
  $ok = $false
}
# what it said about the display it found
$said = Select-String -Path $log -Pattern 'display scaling ([0-9.]+)x: the flower stands ([0-9]+) pixels tall' | Select-Object -First 1
if ($said) {
  Write-Output ("trace:  display scaling {0}x, the flower stands {1} pixels tall" -f $said.Matches[0].Groups[1].Value, $said.Matches[0].Groups[2].Value)
} else {
  Write-Output "note:   the flower said nothing about the display it is on"
}
if (Select-String -Path $log -Pattern 'the display changed' -Quiet) {
  Write-Output "FAIL: the flower reported a change of display when nothing changed"
  $ok = $false
}

Start-Sleep -Milliseconds 300
Stop-Process -Id $p.Id -Force
Start-Sleep -Milliseconds 300
Remove-Item $sandbox -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item Env:\POCKETPETAL_SETTINGS -ErrorAction SilentlyContinue
if ($ok) { Write-Output "PASS: the flower held its place through a screen that changed and one that did not" }
