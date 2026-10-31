#!/usr/bin/env bash
# Deploys this working tree to the production server and rebuilds it there.
#   sh scripts/deploy.sh            (uses the SSH alias "dapper-prod"; set DAPPER_HOST to override)
# What is sent: the files git tracks or would track (respecting .gitignore), minus the planning vault, reference
# material, and anything secret. The server keeps its own .env and .dapper/ (certificates, uploads, logs, binaries).
# On the server: files land in /opt/dapper/app, the web app and Go services are rebuilt as the "dapper" user, the
# database is migrated (only once setup has written .env), and the dapper service is restarted if it is installed.
set -euo pipefail
HOST="${DAPPER_HOST:-dapper-prod}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

LIST="$(mktemp)"
git ls-files -co --exclude-standard \
  | grep -vE '^(Dapper Detailing Knowledge/|output/|3D/|\.impeccable/|\.codex/|reference/|main website/frames-brief/)' \
  | grep -vE '^dapper-platform/apps/(frappe|erpnext|hrms)/' \
  | grep -vE '(^|/)\.env$|\.clixml$|mercedes-gls\.glb$' \
  | while IFS= read -r f; do [ -e "$f" ] && printf '%s\n' "$f"; done > "$LIST"   # skip files deleted but not yet committed
echo "Sending $(wc -l < "$LIST" | tr -d ' ') files to $HOST ($(git rev-parse --short HEAD)$(git diff --quiet HEAD -- . 2>/dev/null || echo ' + uncommitted changes'))"

tar -cf - -T "$LIST" | ssh -o BatchMode=yes "$HOST" 'rm -rf /opt/dapper/incoming && mkdir -p /opt/dapper/incoming && tar -xf - -C /opt/dapper/incoming'
rm -f "$LIST"

ssh -o BatchMode=yes "$HOST" 'bash -s' <<'REMOTE'
set -euo pipefail
command -v rsync >/dev/null || apt-get install -y -qq rsync >/dev/null
# Replace the code, keeping the server's own configuration, data and build output.
rsync -a --delete \
  --exclude '/.env' --exclude '/.dapper/' \
  --exclude 'node_modules/' --exclude '/dapper-platform/apps/web/.next/' \
  --exclude '/main website/assets/frames/' --exclude '/dapper-platform/apps/web/public/site/assets/frames/' \
  /opt/dapper/incoming/ /opt/dapper/app/
rm -rf /opt/dapper/incoming
chown -R dapper:dapper /opt/dapper/app

sudo -u dapper -H bash -lc '
set -euo pipefail
export PATH=/usr/local/go/bin:/usr/local/node/bin:$PATH GOPATH=/opt/dapper/go GOCACHE=/opt/dapper/.cache/go
cd /opt/dapper/app/dapper-platform/apps/web
echo "Installing web dependencies…"; npm ci --no-audit --no-fund --loglevel=error
echo "Building the web app…"; npm run build >/opt/dapper/web-build.log 2>&1 || { tail -40 /opt/dapper/web-build.log; exit 1; }
cd /opt/dapper/app/dapper-platform/apps/api
mkdir -p /opt/dapper/app/.dapper/bin
echo "Building the Go services…"
for t in workspace gateway dapper; do go build -o /opt/dapper/app/.dapper/bin/$t ./cmd/$t; done
if [ -f /opt/dapper/app/.env ]; then echo "Migrating the database…"; /opt/dapper/app/.dapper/bin/dapper migrate --home /opt/dapper/app; fi
'
if systemctl list-unit-files dapper.service >/dev/null 2>&1 && [ -f /opt/dapper/app/.env ]; then systemctl restart dapper && echo "Restarted the dapper service."; else echo "Built. The dapper service starts once setup has written .env."; fi
REMOTE
echo "Deploy finished."
