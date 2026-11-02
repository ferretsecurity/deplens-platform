#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
root="$PWD"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
export PATH="$HOME/.local/share/pnpm:/usr/local/go/bin:$HOME/.local/bin:$PATH"
uv venv --allow-existing var/service-venv
uv pip install --python var/service-venv/bin/python python-dotenv==1.2.1
mkdir -p "$HOME/.config/systemd/user"
for component in api web; do
    unit="$HOME/.config/systemd/user/deplens-platform-$component.service"
    cat > "$unit" <<EOF
[Unit]
Description=Deplens platform $component with source reload
After=network-online.target
StartLimitIntervalSec=0

[Service]
WorkingDirectory=$root
Environment="PATH=$PATH"
Environment=PORT=9005
ExecStart=$root/var/service-venv/bin/python -u $root/deploy/dev/run.py $component
Restart=always
RestartSec=5
TimeoutStartSec=120
TimeoutStopSec=25
KillMode=control-group

[Install]
WantedBy=default.target
EOF
    if [[ "$component" == api ]]; then
        sed -i '/^ExecStart=/i ExecStartPre=/usr/bin/docker compose up -d --wait postgres' "$unit"
    else
        sed -i '/^ExecStart=/i Environment=DEV_LOGIN_PREFILL=true' "$unit"
    fi
done
systemctl --user daemon-reload
systemctl --user enable --now deplens-platform-api.service deplens-platform-web.service
