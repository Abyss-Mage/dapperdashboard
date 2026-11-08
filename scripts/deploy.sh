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

# Checks first: nothing is sent unless the Go services build and pass vet and tests and the web app type-checks.
# Tests that need a database skip unless DAPPER_TEST_DATABASE_URL points at a throwaway one.
echo "Checking the code before sending it…"
(cd dapper-platform/apps/api && go build ./... && go vet ./... && go test ./...) || { echo "Go check failed: nothing was deployed."; exit 1; }
(cd dapper-platform/apps/web && npx tsc --noEmit) || { echo "Web type check failed: nothing was deployed."; exit 1; }
if ! git diff --quiet HEAD -- . 2>/dev/null; then echo "Note: this working tree has changes that are not committed yet."; fi

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
  --exclude '/main website/assets/frames/' \
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
[ -d /opt/dapper/app/.dapper/bin ] && rm -rf /opt/dapper/app/.dapper/bin.prev && cp -a /opt/dapper/app/.dapper/bin /opt/dapper/app/.dapper/bin.prev   # the last working services, for a quick way back
echo "Building the Go services…"
for t in workspace gateway dapper; do go build -o /opt/dapper/app/.dapper/bin/$t ./cmd/$t; done
if [ -f /opt/dapper/app/.env ]; then echo "Migrating the database…"; /opt/dapper/app/.dapper/bin/dapper migrate --home /opt/dapper/app; fi
'
if systemctl list-unit-files dapper.service >/dev/null 2>&1 && [ -f /opt/dapper/app/.env ]; then systemctl restart dapper && echo "Restarted the dapper service."; else echo "Built. The dapper service starts once setup has written .env."; fi
REMOTE
echo "Deploy finished. (To go back to the previous Go services: ssh dapper-prod 'sudo -u dapper cp -a /opt/dapper/app/.dapper/bin.prev/. /opt/dapper/app/.dapper/bin/ && sudo systemctl restart dapper')"
