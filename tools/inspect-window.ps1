Add-Type @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public class E {
  public delegate bool Proc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(Proc p, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int m);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h, StringBuilder s, int m);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr h);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L,T,R,B; }
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X,Y; }
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll", EntryPoint="GetWindowLongPtrW")] public static extern IntPtr GetWindowLongPtr(IntPtr h, int i);
  [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(POINT p);
  [DllImport("user32.dll")] public static extern IntPtr GetWindow(IntPtr h, uint cmd);
  [DllImport("user32.dll")] public static extern bool GetLayeredWindowAttributes(IntPtr h, out uint key, out byte alpha, out uint flags);
  [DllImport("user32.dll")] public static extern bool ScreenToClient(IntPtr h, ref POINT p);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint flags);
}
'@

$targets = @()
$cb = [E+Proc]{ param($h, $l)
  $sb = New-Object System.Text.StringBuilder 256
  [void][E]::GetClassName($h, $sb, 256)
  if ($sb.ToString() -like 'PocketPetal*') {
    $pid2 = 0; [void][E]::GetWindowThreadProcessId($h, [ref]$pid2)
    $script:targets += [pscustomobject]@{ hwnd = $h; cls = $sb.ToString(); pid = $pid2 }
  }
  return $true
}
[void][E]::EnumWindows($cb, [IntPtr]::Zero)

foreach ($t in $targets) {
  $r = New-Object E+RECT; [void][E]::GetWindowRect($t.hwnd, [ref]$r)
  $ex = [E]::GetWindowLongPtr($t.hwnd, -20).ToInt64()
  $key = 0; $alpha = 0; $flags = 0
  $ok = [E]::GetLayeredWindowAttributes($t.hwnd, [ref]$key, [ref]$alpha, [ref]$flags)
  Write-Output ("{0} pid={1} rect={2},{3} {4}x{5} visible={6} iconic={7}" -f $t.cls, $t.pid, $r.L, $r.T, ($r.R - $r.L), ($r.B - $r.T), [E]::IsWindowVisible($t.hwnd), [E]::IsIconic($t.hwnd))
  Write-Output ("   exstyle=0x{0:x} topmost={1} layered={2} transparent={3} toolwindow={4} noactivate={5} attrs={6} key=0x{7:x} alpha={8} flags={9}" -f $ex, (($ex -band 0x8) -ne 0), (($ex -band 0x80000) -ne 0), (($ex -band 0x20) -ne 0), (($ex -band 0x80) -ne 0), (($ex -band 0x8000000) -ne 0), $ok, $key, $alpha, $flags)
  if ($t.cls -eq 'PocketPetalFlowerWindow') {
    foreach ($off in @(@(95,125), @(30,40), @(150,60), @(95,230))) {
      $p = New-Object E+POINT; $p.X = $r.L + $off[0]; $p.Y = $r.T + $off[1]
      $top = [E]::WindowFromPoint($p)
      $tsb = New-Object System.Text.StringBuilder 256; [void][E]::GetClassName($top, $tsb, 256)
      Write-Output ("   at {0},{1}: top={2} class={3} ours={4}" -f $p.X, $p.Y, $top, $tsb.ToString(), ($top -eq $t.hwnd))
    }
    # walk the z-order upwards from the flower: who is above us
    $prev = $t.hwnd
    for ($i = 0; $i -lt 6; $i++) {
      $above = [E]::GetWindow($prev, 3) # GW_HWNDPREV
      if ($above -eq [IntPtr]::Zero) { Write-Output "   nothing above (index $i)"; break }
      $asb = New-Object System.Text.StringBuilder 256; [void][E]::GetClassName($above, $asb, 256)
      $apid = 0; [void][E]::GetWindowThreadProcessId($above, [ref]$apid)
      Write-Output ("   above[{0}]: class={1} pid={2} visible={3}" -f $i, $asb.ToString(), $apid, [E]::IsWindowVisible($above))
      $prev = $above
    }
  }
}

# capture a crop around the flower so the pixels themselves can be judged
Add-Type -AssemblyName System.Drawing
$h = ($targets | Where-Object { $_.cls -eq 'PocketPetalFlowerWindow' } | Select-Object -First 1).hwnd
$r = New-Object E+RECT; [void][E]::GetWindowRect($h, [ref]$r)
$w = $r.R - $r.L; $ht = $r.B - $r.T
$bmp = New-Object System.Drawing.Bitmap($w + 80, $ht + 80)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($r.L - 40, $r.T - 40, 0, 0, (New-Object System.Drawing.Size(($w + 80), ($ht + 80))))
$bmp.Save("$PSScriptRoot\.window-crop.png", [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose(); $bmp.Dispose()
Write-Output ("saved crop of {0},{1} {2}x{3}" -f $r.L, $r.T, $w, $ht)
