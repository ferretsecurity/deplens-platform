# Run the development services

Install Node.js, pnpm, Go, uv, and Docker Compose, then run `pnpm install` in the repository root.
Create `.env` using the root `.env.example` if it does not already exist. Keep your existing credentials and encryption key.

Run from the repository root:

```bash
bash deploy/dev/install.sh
```

Enable user services at boot with `sudo loginctl enable-linger "$USER"` if lingering is not already enabled.
The installer creates and enables `deplens-platform-api.service` and `deplens-platform-web.service` under `~/.config/systemd/user/`.
The API service starts Postgres using the root Compose file and waits for database readiness.
Keep Docker enabled at startup. On WSL with Docker Desktop, enable Docker Desktop startup at Windows sign-in.

Open `http://localhost:9005`. Port 9005 is reserved for Deplens on the configured workstation.
The web server listens on all IPv4 interfaces. The API listens on loopback port 8080, with browser requests forwarded by Next.js.
The runner loads secrets with `load_dotenv` and leaves `.env` unchanged.
It uses the local Compose database on port 5432 and `var/blobs` for storage, overriding container-only blob paths.
Set `DATABASE_URL` in the environment if you use another database.

Next.js reloads web source changes. The API watcher checks `cmd`, `internal`, `db`, `go.mod`, `go.sum`, and `.env` every second.
It rebuilds the API and restarts it after a successful build. A failed build leaves the previous API running.
After changing the runner or web process environment, restart the relevant service.

Inspect and control the services:

```bash
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
systemctl --user status deplens-platform-{api,web}
journalctl --user -u deplens-platform-api -u deplens-platform-web -f
systemctl --user restart deplens-platform-api deplens-platform-web
```

To disable automatic startup:

```bash
systemctl --user disable --now deplens-platform-api deplens-platform-web
```

## Enable Windows network access

For WSL NAT networking, copy `windows-install.ps1` and `windows-forward.ps1` to a Windows-local directory.
Run `windows-install.ps1` from an administrator PowerShell window.
The scripts target the `Ubuntu` distribution and port 9005.
They create the `Deplens Platform` scheduled task, which starts WSL at Windows sign-in and keeps it running.
Linux services start when WSL starts. This setup does not start WSL before Windows sign-in.

The task forwards Windows port 9005 to WSL and updates the destination when WSL's address changes.
The firewall rule `DeplensPlatform9005` allows home subnet and Tailscale IPv4 clients.
Tailscale access also depends on your tailnet access rules.
Forwarding and installation status files are written beside the Windows scripts.

On the configured workstation, the scripts are installed in `C:\Users\proce\AppData\Local\DeplensPlatform`.
Open any of these addresses:

- Localhost: `http://localhost:9005`
- Home network: `http://192.168.1.147:9005`
- Tailscale: `http://jekospc-wsl.tail12e672.ts.net:9005`

To remove Windows startup and forwarding, run in administrator PowerShell:

```powershell
Stop-ScheduledTask -TaskName 'Deplens Platform'
Unregister-ScheduledTask -TaskName 'Deplens Platform' -Confirm:$false
netsh interface portproxy delete v4tov4 listenaddress=0.0.0.0 listenport=9005
Remove-NetFirewallRule -Name 'DeplensPlatform9005'
```
