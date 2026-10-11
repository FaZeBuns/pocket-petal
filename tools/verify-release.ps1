# Verifies the standalone exe the way a person meets it: copied somewhere on its
# own, double-clicked, with nothing beside it. Reports the windows it owns, any
# console it has, and where it keeps its settings.
#
#	tools/verify-release.ps1 dist/PocketPetal.exe
param([string]$Exe)

Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class V {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int m);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("shell32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern int ExtractIconExW(string file, int index, IntPtr[] large, IntPtr[] small, int count);
  [DllImport("version.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern uint GetFileVersionInfoSizeW(string name, out uint handle);
  [DllImport("version.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool GetFileVersionInfoW(string name, uint handle, uint len, byte[] data);
  [DllImport("version.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool VerQueryValueW(byte[] block, string sub, out IntPtr outBuf, out uint outLen);
}
'@

if (-not (Test-Path $Exe)) { Write-Output "FAIL: no such exe: $Exe"; exit 1 }

# The file's own face, read back by *Windows* rather than by the app: the icon Explorer
# and the taskbar draw, and the version block the Details tab shows. Both are written by
# hand (cmd/mkicon) and both fail quietly — an exe with no picture on it, and a version
# that reads as blank — so they are checked here, on the real file, with the same calls
# the shell itself makes.
$large = New-Object IntPtr[] 8
$small = New-Object IntPtr[] 8
$icons = [V]::ExtractIconExW($Exe, 0, $large, $small, 8)
if ($icons -lt 1) {
  Write-Output "FAIL: Windows reads no icon out of the exe: the file has no face on it"
} else {
  Write-Output ("  icon: Windows reads {0} icon(s) out of the file, the large one {1}" -f $icons, $large[0])
}
$vh = 0
$vsize = [V]::GetFileVersionInfoSizeW($Exe, [ref]$vh)
if ($vsize -eq 0) {
  Write-Output "FAIL: Windows reads no version block out of the exe: the Details tab would be empty"
} else {
  $vbuf = New-Object byte[] $vsize
  $p = [IntPtr]::Zero; $len = 0
  $said = ""
  if ([V]::GetFileVersionInfoW($Exe, 0, $vsize, $vbuf) -and
      [V]::VerQueryValueW($vbuf, "\StringFileInfo\040904B0\ProductName", [ref]$p, [ref]$len)) {
    # read to the terminator rather than trusting the length: VerQueryValue reports a
    # string's length in characters on some builds and in bytes on others
    $sb = New-Object System.Text.StringBuilder 128
    for ($i = 0; $i -lt 120; $i++) {
      $c = [Runtime.InteropServices.Marshal]::ReadInt16($p, $i * 2)
      if ($c -eq 0) { break }
      [void]$sb.Append([char]$c)
    }
    $said = $sb.ToString()
  }
  Write-Output ("  version: Windows reads {0} bytes of it{1}" -f $vsize, $(if ($said -ne "") { ", called `"$said`"" } else { "" }))
}

# a directory of its own, with nothing in it: if the exe needed a file beside it,
# this is where that would show up
$sandbox = Join-Path $env:TEMP ("flower-release-" + (Get-Random))
New-Item -ItemType Directory -Path $sandbox | Out-Null
$there = Join-Path $sandbox (Split-Path $Exe -Leaf)
Copy-Item $Exe $there

$before = (Get-Process -ErrorAction SilentlyContinue).Id
$p = Start-Process -FilePath $there -PassThru
Start-Sleep -Seconds 4
$p.Refresh()
if ($p.HasExited) { Write-Output "FAIL: the exe exited immediately (code $($p.ExitCode))"; exit 1 }
Write-Output ("running: pid {0} from {1}" -f $p.Id, $there)

# every window the process owns, and any console window at all
$classes = New-Object System.Collections.ArrayList
$cb = [V+Proc]{ param($h, $l)
  $pid2 = 0; [void][V]::GetWindowThreadProcessId($h, [ref]$pid2)
  if ($pid2 -eq $script:want) {
    $sb = New-Object System.Text.StringBuilder 256
    [void][V]::GetClassName($h, $sb, 256)
    [void]$script:classes.Add(("{0} visible={1}" -f $sb.ToString(), [V]::IsWindowVisible($h)))
  }
  return $true
}
$script:want = $p.Id
$script:classes = $classes
[void][V]::EnumWindows($cb, [IntPtr]::Zero)
foreach ($c in $classes) { Write-Output ("  window: {0}" -f $c) }
if (($classes | Where-Object { $_ -like 'ConsoleWindowClass*' }).Count -gt 0) {
  Write-Output "FAIL: the process owns a console window: the black box is back"
} else {
  Write-Output "  no console window at all"
}

# a console subsystem exe gets a conhost child even when nobody asked for one
$conhost = Get-CimInstance Win32_Process -Filter "Name='conhost.exe'" -ErrorAction SilentlyContinue |
  Where-Object { $_.ParentProcessId -eq $p.Id }
if ($conhost) { Write-Output "FAIL: a conhost was started for it" } else { Write-Output "  no conhost child" }

# what it wrote while it ran: the settings file is the only thing it may create
Write-Output "  wrote:"
Get-ChildItem $sandbox -Force | ForEach-Object { Write-Output ("    {0}" -f $_.Name) }
$settings = Join-Path $env:APPDATA 'pocket-petal\settings.json'
Write-Output ("  settings: {0} exists={1}" -f $settings, (Test-Path $settings))

Start-Sleep -Seconds 1
Stop-Process -Id $p.Id -Force
Start-Sleep -Milliseconds 500
Remove-Item $sandbox -Recurse -Force -ErrorAction SilentlyContinue
Write-Output "done: the flower was stopped again"
