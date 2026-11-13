# New production server: step-by-step setup (2026-11-13)

**Platform (owner decision, 2026-11-13): Ubuntu 26.04 LTS with Apache 2.4** as the HTTPS front door, using Let's Encrypt certificates through certbot. This replaces Caddy, which the dev server used.

User direction (2026-11-13): a brand-new server for full production, set up with a **new, clean database**. The legacy website's data comes in through the legacy import MCP server. Claude gets no shell access to this server. Its only way in is the import MCP server, once step 10 is done. Every step below is run by you.

**Conventions**
- Run each command block as **root** on the new server (`ssh root@NEW_SERVER_IP`) unless it says *on your PC*.
- Replace `NEW_SERVER_IP` with the new address.
- Anything in `<angle brackets>` is yours to choose. Never paste it into chat.

The old dev server's data is backed up on your PC (`C:\Users\apexp\DapperBackups\pre-production-20261113-094233\`). It is **not** restored here, because this server starts clean.

---

## 0. Before you start (on your PC / domain registrar)
1. At the registrar, add DNS A records → `NEW_SERVER_IP` for **`dashboard`** and **`workspace`** (TTL 300).
2. Leave `dapperdetailing.ca` and `www` on the legacy host for now; they move on go-live day.
3. Make sure you can log in: `ssh root@NEW_SERVER_IP`.

## 1. Update the system
```bash
apt update && apt -y full-upgrade && reboot
```
Log in again after the reboot.

## 2. Packages, firewall, fail2ban
```bash
apt install -y git curl ufw fail2ban postgresql python3-venv rsync apache2 certbot python3-certbot-apache
ufw allow OpenSSH && ufw allow 'Apache Full' && ufw --force enable
printf '[sshd]\nenabled = true\nmaxretry = 6\nfindtime = 10m\nbantime = 1h\n' > /etc/fail2ban/jail.d/dapper-sshd.local
systemctl enable --now fail2ban
```
Recommended once your SSH key works: set `PasswordAuthentication no` and `PermitRootLogin prohibit-password` in `/etc/ssh/sshd_config`, run `systemctl reload ssh`, and **test a new key login in a second window before closing the first**.

## 3. Go 1.26.4 and Node 24.16.0 (the versions the app is built with)
```bash
cd /tmp
curl -fLO https://go.dev/dl/go1.26.4.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.26.4.linux-amd64.tar.gz
curl -fLO https://nodejs.org/dist/v24.16.0/node-v24.16.0-linux-x64.tar.xz && mkdir -p /usr/local/node && tar -xJf node-v24.16.0-linux-x64.tar.xz -C /usr/local/node --strip-components=1
/usr/local/go/bin/go version && /usr/local/node/bin/node -v
```

## 4. Apache modules
```bash
a2enmod proxy proxy_http headers ssl rewrite
apache2 -v && psql --version && python3 --version
```
Note the PostgreSQL version this prints. Ubuntu 26.04 ships a newer PostgreSQL than the 16 on your PC, so for restore checks on the PC install that same major version's tools. `pg_restore` can't read a dump made by a newer `pg_dump`. Ubuntu 26.04 also uses `sudo-rs` as `sudo`. The commands below use only the common options it supports.

## 5. The clean database
```bash
sudo -u postgres createuser --pwprompt dapper      # type a new database password and keep it in your password manager
sudo -u postgres createdb -O dapper dapper
```

## 6. App account and code
The repository is public, so the server clones it without credentials. Only the parts the app needs are fetched; the ERPNext/Frappe reference code is skipped.
```bash
useradd --system --create-home --home-dir /opt/dapper --shell /bin/bash dapper
sudo -u dapper git clone --filter=blob:none --sparse -b master https://github.com/Abyss-Mage/dapperdashboard.git /opt/dapper/app
cd /opt/dapper/app && sudo -u dapper git sparse-checkout set dapper-platform/apps/api dapper-platform/apps/web "main website" scripts tools
install -d -o dapper -g dapper -m 700 /opt/dapper/backups /opt/dapper/imports
bash /opt/dapper/app/scripts/server-update.sh
```
The last command builds everything (a few minutes). It ends with "Built. Run the setup wizard next".

## 7. Setup wizard (owner account, first franchise, email)
On the server:
```bash
sudo -u dapper /opt/dapper/app/.dapper/bin/dapper setup --home /opt/dapper/app --no-browser
```
It prints a **setup code**. Leave it running.

*On your PC*, open a second terminal and run:
```bash
ssh -L 8099:127.0.0.1:8099 root@NEW_SERVER_IP
```
Then open http://127.0.0.1:8099 in your browser, enter the code, and fill in:

| Field | Value |
|---|---|
| Site address | `https://dashboard.dapperdetailing.ca` (changes to `https://dapperdetailing.ca` on go-live day) |
| Listen address | `127.0.0.1:8443` |
| HTTPS | behind a reverse proxy (`proxy`) |
| Timezone | `America/Toronto` |
| Database | host `127.0.0.1`, port `5432`, name `dapper`, user `dapper`, the password from step 5 (don't tick "create database") |
| Email (SMTP) | your **working** mail provider (the Outlook account was failing on 2026-11-13). You can skip it and add it later |
| Owner | your email and a long password. This account becomes **Super Admin** |
| First franchise | name, code, address and phone of the first franchise (e.g. Peterborough) |

When the wizard says it's finished, it stops by itself. Back on the server:
```bash
echo 'DAPPER_EXTRA_ORIGINS=https://workspace.dapperdetailing.ca' >> /opt/dapper/app/.env
sudo -u dapper /opt/dapper/app/.dapper/bin/dapper migrate --home /opt/dapper/app
cp /opt/dapper/app/scripts/server/dapper.service /etc/systemd/system/dapper.service
systemctl daemon-reload && systemctl enable --now dapper && sleep 15 && curl -s http://127.0.0.1:8443/api/v1/health -H 'Host: 127.0.0.1:8443'
```

## 8. HTTPS addresses (Apache + Let's Encrypt)
```bash
cp /opt/dapper/app/scripts/server/apache-dapper.conf /etc/apache2/sites-available/dapper.conf
a2ensite dapper && a2dissite 000-default
apachectl configtest && systemctl reload apache2
certbot --apache -d dashboard.dapperdetailing.ca -d workspace.dapperdetailing.ca
```
certbot asks for your e-mail (certificate notices) and whether to redirect HTTP to HTTPS. Choose **redirect**. It renews the certificates by itself (`systemctl list-timers | grep certbot`). If it fails, check that DNS points at this server (`dig +short dashboard.dapperdetailing.ca`).

Open **https://dashboard.dapperdetailing.ca** and sign in as the owner. https://workspace.dapperdetailing.ca opens the staff workspace.

What the Apache site does:
- keeps the original address (`ProxyPreserveHost On`), because the app refuses addresses it doesn't know
- allows long requests (`ProxyTimeout 7200`) for photo uploads and owner backups
- streams `/legacy-import/` unbuffered and uncompressed for the import MCP server
- sends everything else to the app on `127.0.0.1:8443`

## 9. Backups
```bash
cp /opt/dapper/app/scripts/server/dapper-backup.cron /etc/cron.d/dapper-backup && chmod 644 /etc/cron.d/dapper-backup
sudo -u dapper bash -c 'cd /opt/dapper/backups && pg_dump -Fc -d dapper -f first-check.dump && ls -l'
```
The nightly job (03:15) saves the database, `.env`, uploads and SOP files for 14 days, on this disk only. For an off-server copy every day:
- *on your PC*, set up the SSH alias `dapper-prod` for the new server (`~/.ssh/config`: `Host dapper-prod`, `HostName NEW_SERVER_IP`, `User root`, your key);
- schedule `pwsh -NoProfile -File scripts/pull-backup.ps1` (the instructions are at the top of that script).

## 10. Legacy import MCP server (Claude's way in)
**a) WordPress Application Password.** In the live site's admin: **Users → your admin user → Application Passwords**. Name it `Dapper import`, then **Add New**. Copy the password it shows (spaces don't matter).

**b) Install and configure on the server:**
```bash
python3 -m venv /opt/dapper/mcp-venv && /opt/dapper/mcp-venv/bin/pip install --quiet "mcp==1.30.0" && chown -R dapper:dapper /opt/dapper/mcp-venv
install -d -m 700 /etc/dapper
TOKEN=$(openssl rand -hex 32)
printf 'DAPPER_IMPORT_MCP_TOKEN=%s\nDAPPER_LEGACY_WP_USER=%s\nDAPPER_LEGACY_WP_APP_PASSWORD=%s\n' "$TOKEN" '<WordPress admin username>' '<application password>' > /etc/dapper/import-mcp.env
chmod 600 /etc/dapper/import-mcp.env
cp /opt/dapper/app/scripts/server/dapper-import-mcp.service /etc/systemd/system/
systemctl daemon-reload && systemctl enable --now dapper-import-mcp
curl -s -o /dev/null -w '%{http_code}\n' https://dashboard.dapperdetailing.ca/legacy-import/mcp     # 401 = running and locked
echo "$TOKEN"     # copy it for the next step, then: clear
```

**c) On your PC** (PowerShell), store the same token, then restart Claude Code:
```powershell
[Environment]::SetEnvironmentVariable('DAPPER_IMPORT_MCP_TOKEN', '<the token>', 'User')
```
The project's `.mcp.json` already has `dapper-legacy-import-prod`, pointing at `https://dashboard.dapperdetailing.ca/legacy-import/mcp` with that token. **Tell Claude when this is done.** Claude then runs `legacy_import_status` and checks the sign-in to the live site.

## 11. The BookingPress export (customers, bookings, payments)
In the live site's admin, download the full BookingPress export (**BookingPress → Settings → Import/Export → Export**). *On your PC*:
```bash
scp <path to the export file> root@NEW_SERVER_IP:/opt/dapper/imports/bookingpress-export.json
```
On the server: `chown dapper:dapper /opt/dapper/imports/*`. Then tell Claude.

## 12. Importing into the clean database (with Claude)
Order:
1. Catalogue names
2. Catalogue prices
3. Customers and bookings
4. Billing

For each one, Claude shows a dry-run report (counts, totals, anything odd). It is applied only after you say yes, in one transaction. A customers and bookings batch can be undone. Website content and images are fetched for review and placed by hand.

## 13. Then the production checklist
Continue with `PRODUCTION_CHECKLIST_2026-11-13.md`:
- live Square keys per franchise
- email domain records (SPF, DKIM, DMARC)
- staff invitations, roles and SOP documents
- SEO settings
- monitoring
- go-live day: move `dapperdetailing.ca`, change the site address in `.env`, turn on indexing

**Updating the app later** (on the server):
```bash
bash /opt/dapper/app/scripts/server-update.sh
```
To return to a tagged version: `bash /opt/dapper/app/scripts/server-update.sh pre-production-2026-11-13`.
