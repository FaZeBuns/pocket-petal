<#
  A fake AI chat, for testing delivery without bothering a real one.

  It is a WinForms window whose title looks like a chat (so the flower names it
  properly), with a text box and an Enter key that "sends": the sent text is
  written to assets/delivered.txt, which is what the tests assert on.

  It also writes its own geometry to assets/chat-rect.txt so tools/drag-flower.ps1
  knows where to drop the flower.

  Usage:  powershell -NoProfile -ExecutionPolicy Bypass -File tools/fake-chat.ps1
#>

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$assets = Join-Path $root 'assets'
New-Item -ItemType Directory -Force -Path $assets | Out-Null
$delivered = Join-Path $assets 'delivered.txt'
$geometry = Join-Path $assets 'chat-rect.txt'
Remove-Item -Force -ErrorAction SilentlyContinue $delivered

$form = New-Object System.Windows.Forms.Form
$form.Text = 'Antigravity — Test Conversation'
$form.Size = New-Object System.Drawing.Size(560, 420)
$form.StartPosition = 'Manual'
$form.Location = New-Object System.Drawing.Point(320, 140)
$form.BackColor = [System.Drawing.Color]::FromArgb(24, 24, 28)
$form.ForeColor = [System.Drawing.Color]::White

$log = New-Object System.Windows.Forms.Label
$log.Text = 'waiting for a flower...'
$log.AutoSize = $false
$log.Dock = 'Top'
$log.Height = 60
$log.Padding = New-Object System.Windows.Forms.Padding(12, 8, 12, 8)
$form.Controls.Add($log)

$box = New-Object System.Windows.Forms.TextBox
$box.Multiline = $true
$box.Height = 90
$box.Dock = 'Bottom'
$box.BackColor = [System.Drawing.Color]::FromArgb(38, 38, 46)
$box.ForeColor = [System.Drawing.Color]::White
$box.Font = New-Object System.Drawing.Font('Segoe UI', 11)
$form.Controls.Add($box)

function Send-Note {
  param([string]$text)
  if ([string]::IsNullOrWhiteSpace($text)) { return }
  # append, so a second delivery cannot hide the first one
  Add-Content -Path $delivered -Value $text -Encoding utf8
  $log.Text = "sent: $text"
  $box.Clear()
}

$box.Add_KeyDown({
  param($sender, $event)
  if ($event.KeyCode -eq [System.Windows.Forms.Keys]::Enter -and -not $event.Shift) {
    $event.SuppressKeyPress = $true
    Send-Note $box.Text
  }
})

# Enter anywhere in the window sends too, which is what a real chat box does
$form.Add_KeyDown({
  param($sender, $event)
  if ($event.KeyCode -eq [System.Windows.Forms.Keys]::Enter) { Send-Note $box.Text }
})

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 400
$timer.Add_Tick({
  $form.Activate() | Out-Null
  $box.Focus() | Out-Null
  $p = $box.PointToScreen([System.Drawing.Point]::new(0, 0))
  $r = $form.RectangleToScreen($form.ClientRectangle)
  @(
    "form=$($r.Left),$($r.Top),$($r.Width),$($r.Height)"
    "box=$($p.X + [int]($box.Width / 2)),$($p.Y + [int]($box.Height / 2))"
    "text=$($box.Text)"
  ) | Set-Content -Path $geometry -Encoding utf8
})

$timer.Start()
[void]$form.ShowDialog()
