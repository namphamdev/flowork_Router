# flow_router — Windows tray (NotifyIcon)
# Native Windows.Forms NotifyIcon. No CGO. Run as:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/tray-win.ps1

param(
    [string]$Url = $env:FLOW_ROUTER_URL,
    [string]$Title = "flow_router"
)

if (-not $Url) { $Url = "http://127.0.0.1:2402" }

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$icon = New-Object System.Windows.Forms.NotifyIcon
$icon.Text = $Title
$icon.Icon = [System.Drawing.SystemIcons]::Application
$icon.Visible = $true

$menu = New-Object System.Windows.Forms.ContextMenuStrip

$openItem = $menu.Items.Add("Open dashboard")
$openItem.Add_Click({ Start-Process $Url })

$statusItem = $menu.Items.Add("Check status")
$statusItem.Add_Click({
    try {
        $resp = Invoke-WebRequest -Uri "$Url/api/health" -UseBasicParsing -TimeoutSec 5
        $icon.ShowBalloonTip(3000, $Title, $resp.Content, [System.Windows.Forms.ToolTipIcon]::Info)
    } catch {
        $icon.ShowBalloonTip(3000, $Title, "Router unreachable", [System.Windows.Forms.ToolTipIcon]::Warning)
    }
})

$menu.Items.Add("-") | Out-Null

$exitItem = $menu.Items.Add("Quit tray")
$exitItem.Add_Click({
    $icon.Visible = $false
    $icon.Dispose()
    [System.Windows.Forms.Application]::Exit()
})

$icon.ContextMenuStrip = $menu
$icon.Add_DoubleClick({ Start-Process $Url })

[System.Windows.Forms.Application]::Run()
