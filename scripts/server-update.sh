#!/usr/bin/env bash
# Updates the production server from GitHub and rebuilds it. Run ON THE SERVER as root:
#   bash /opt/dapper/app/scripts/server-update.sh            (the master branch)
#   bash /opt/dapper/app/scripts/server-update.sh <tag>      (a tagged version, e.g. to go back)
# The server keeps its own .env, .dapper/ (uploads, logs, binaries) and .cache/. The database is migrated only once setup has written .env.
set -euo pipefail
REF="${1:-master}"
APP=/opt/dapper/app
sudo -u dapper -H bash -lc "
set -euo pipefail
export PATH=/usr/local/go/bin:/usr/local/node/bin:\$PATH GOPATH=/opt/dapper/go GOCACHE=/opt/dapper/.cache/go
cd $APP
git fetch --quiet origin
git checkout --quiet '$REF'
[ '$REF' = master ] && git reset --quiet --hard origin/master
echo \"Code: \$(git log --oneline -1)\"
cd dapper-platform/apps/web
echo 'Installing web dependencies…'; npm ci --no-audit --no-fund --loglevel=error
echo 'Building the web app…'; npm run build >/opt/dapper/web-build.log 2>&1 || { tail -40 /opt/dapper/web-build.log; exit 1; }
cd $APP/dapper-platform/apps/api
mkdir -p $APP/.dapper/bin
rm -rf $APP/.dapper/bin.prev && cp -a $APP/.dapper/bin $APP/.dapper/bin.prev
echo 'Building the Go services…'
for t in workspace gateway dapper; do go build -o $APP/.dapper/bin/\$t ./cmd/\$t; done
if [ -f $APP/.env ]; then echo 'Migrating the database…'; $APP/.dapper/bin/dapper migrate --home $APP; fi
"
if [ -f $APP/.env ]; then systemctl restart dapper && echo "Restarted dapper."; else echo "Built. Run the setup wizard next (see the setup guide)."; fi
if systemctl is-enabled --quiet dapper-import-mcp 2>/dev/null; then systemctl restart dapper-import-mcp && echo "Restarted the import MCP server."; fi
echo "Done. To go back to the previous Go services: sudo -u dapper cp -a $APP/.dapper/bin.prev/. $APP/.dapper/bin/ && systemctl restart dapper"
