# Run from an elevated PowerShell. Keep both scripts in a Windows-local directory.
$ErrorActionPreference = 'Stop'
try {
    Get-ScheduledTask -TaskName 'Deplens Platform' -ErrorAction SilentlyContinue | Stop-ScheduledTask
    & netsh interface portproxy delete v4tov4 listenaddress=0.0.0.0 listenport=9006 | Out-Null
    Get-NetFirewallRule -Name 'DeplensPlatform9006' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
    $who = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    Set-Service iphlpsvc -StartupType Automatic
    Start-Service iphlpsvc
    $rule = 'DeplensPlatform9005'
    Get-NetFirewallRule -Name $rule -ErrorAction SilentlyContinue | Remove-NetFirewallRule
    New-NetFirewallRule -Name $rule -DisplayName 'Deplens platform home and Tailscale' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 9005 -RemoteAddress LocalSubnet,100.64.0.0/10 -Profile Any | Out-Null
    $action = New-ScheduledTaskAction -Execute "$env:WINDIR\System32\WindowsPowerShell\v1.0\powershell.exe" -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$PSScriptRoot\windows-forward.ps1`""
    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $who
    $principal = New-ScheduledTaskPrincipal -UserId $who -LogonType Interactive -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew -StartWhenAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    Register-ScheduledTask -TaskName 'Deplens Platform' -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description 'Start Deplens in WSL at sign-in and maintain LAN/Tailscale forwarding on port 9005.' -Force | Out-Null
    Start-ScheduledTask -TaskName 'Deplens Platform'
    Set-Content "$PSScriptRoot\install-status.txt" 'Installed'
} catch {
    Set-Content "$PSScriptRoot\install-status.txt" $_.Exception.ToString()
    exit 1
}
