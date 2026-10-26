# Dapper: full installation and operations guide

This is the complete guide to installing, running, updating and troubleshooting Dapper on your own server or computer. It works like a WordPress install: you download the code, start it once, and a setup page in your browser asks for your database, your site address, your email server and the first owner account. **Until a completed `.env` file exists, starting Dapper opens that setup page first and nothing else runs.**

Contents: 1 What you are installing · 2 What you need · 3 Prepare PostgreSQL · 4 Get the code · 5 First start (setup) · 6 After setup · 7 HTTPS and your domain · 8 Keep it running · 9 Updating and backups · 10 Configuration reference · 11 Security · 12 Troubleshooting · 13 Reset or uninstall

---

## 1. What you are installing

| Part | What it is | Where |
|---|---|---|
| **Dashboard** (`/dashboard`) | The main dashboard for owners and administrators: calendar, appointments, customers, invoices, quotes, dealerships, staff hours and map, services, opening hours and holidays, franchises, team, card payments | `dapper-platform/apps/web` |
| **Workspace** (`/workspace`) | The staff app for employees: their bookings, clock in/out, photos, availability | `dapper-platform/apps/web` |
| **API** | The server that holds all the rules and talks to PostgreSQL | `dapper-platform/apps/api` (Go) |
| **Gateway** | The HTTPS front door that sends `/api/...` to the API and everything else to the web app | `dapper-platform/apps/api/cmd/gateway` |
| **Public website** | The marketing site (home page with the animated story, services, portfolio, about, contact) | `main website/` (built into `dapper-platform/apps/web/public/site`) |

The public website is **not** wired to the dashboard by setup. It is a separate part of this repository.

One command starts all of the running parts (web app, API, gateway) from the settings in `.env`.

## 2. What you need

| Requirement | Version | Get it |
|---|---|---|
| Git | any recent | https://git-scm.com/downloads |
| Go | 1.26 or newer | https://go.dev/dl/ |
| Node.js (with npm) | 20 LTS or newer | https://nodejs.org |
| PostgreSQL | 15 or newer (tested on 17) | https://www.postgresql.org/download/ |
| A computer or server | 2 GB RAM, 5 GB disk (the first web build needs about 2 GB RAM) | Windows 10/11, macOS, or Linux |

Check the tools are installed (each should print a version):

```
git --version
go version
node --version
npm --version
psql --version
```

You do **not** need Docker, WSL, or any cloud service. Square (card payments), Mailchimp and Invoice-style integrations are optional and are configured later inside the dashboard.

## 3. Prepare PostgreSQL

Setup needs an existing PostgreSQL server and a user it can log in as. Setup creates the tables itself and only installs into an **empty** database. Pick one of the two routes.

### Route A: you can create databases (simplest)

Create a user that is allowed to create databases. In `psql` as an administrator (`postgres`):

```sql
CREATE USER dapper WITH PASSWORD 'choose-a-long-random-password' CREATEDB;
```

In setup, enter that user and password and tick nothing else: setup will create the database (default name `dapper`) for you.

### Route B: someone gives you an empty database

```sql
CREATE USER dapper WITH PASSWORD 'choose-a-long-random-password';
CREATE DATABASE dapper OWNER dapper;
```

In setup, enter the same names. Setup connects to that database and installs into it (it refuses a database that already holds data).

### Install PostgreSQL if you do not have it

- **Windows:** download the installer from https://www.postgresql.org/download/windows/ (keep the default port 5432, remember the `postgres` password).
- **macOS:** `brew install postgresql@17 && brew services start postgresql@17`
- **Debian/Ubuntu:** `sudo apt install postgresql` (it starts automatically), then `sudo -u postgres psql` to run the SQL above.

Allow the connection: if PostgreSQL is on another machine, make sure its `pg_hba.conf` allows this server and the port (5432) is reachable. Use "Encrypted" connection security in setup for any database that is not on the same machine or a private network.

## 4. Get the code

```
git clone https://github.com/Abyss-Mage/dapperdashboard.git
cd dapperdashboard
```

## 5. First start (setup)

Run the start script for your system from the repository folder:

**Windows (PowerShell)**

```
powershell -File scripts/start.ps1
```

**macOS / Linux**

```
sh scripts/start.sh
```

What happens, in order:

1. Dapper looks for a completed `.env` file. There isn't one yet, so it prints a **setup code** and opens the setup page in your browser (`http://localhost:8099`). If the browser does not open, copy the address from the terminal.
2. You enter the setup code (it is printed in the terminal window, and it stops anyone else from running setup on your machine) and answer the pages below.
3. Setup writes `.env`, closes the setup page, and **continues on its own**: it builds the web app (first time only, a few minutes), builds the services, and starts everything.
4. When it prints "Dapper is running at …", your browser opens the dashboard.

If you only want the setup page without starting Dapper, run `powershell -File scripts/setup.ps1` or `sh scripts/setup.sh`.

### The setup pages

**1. Welcome.** Enter the setup code. Setup checks that it can write to the folder and that `go`/`node` exist.

**2. Database.** Host (for example `localhost`), port (`5432`), database name, user, password, and connection security (None for the same machine, Encrypted for anything remote). Press **Test connection**. It tells you whether the server is reachable, whether the database exists, whether this user may create it, and whether it is empty. You can only continue when the result says it is ready.

**3. Your site.**
- *Business name*: shown in emails and on the owner account.
- *Site address*: where people will open Dapper, for example `https://dapper.example.com` or `https://localhost:8443` while testing. No path.
- *Default timezone*: used as the default for new locations (each location keeps its own).
- *Listen address*: where Dapper accepts connections. `127.0.0.1:8443` is only this computer. Use `0.0.0.0:443` to accept connections from the internet or your network (see section 7).
- *HTTPS*: choose one of three (explained in section 7): create a self-signed certificate, use your own certificate files, or "I run a reverse proxy".

**3. Email (optional).** SMTP server, port (usually 587), username, password, From address and From name. Press **Send test** to be sure it works. You can skip it; invitations then stay queued until you add email later.

Examples:
- Google Workspace / Gmail: server `smtp.gmail.com`, port `587`, username your address, password an **App password** (https://myaccount.google.com/apppasswords).
- Microsoft 365: server `smtp.office365.com`, port `587`.
- Providers such as Brevo, Mailgun, Postmark or SendGrid give you the server, port, username and password on their SMTP page.

**4. Owner account.** The email and password of the first person who can sign in. The password must be at least 15 characters. There is no default account; this is the only one setup creates.

**5. Install.** Shows a summary. **Install Dapper** creates the database (if needed), the tables, the owner, two generated secret keys, the certificate (if you chose self-signed) and the `.env` file. It cannot be run twice.

## 6. After setup

Open the address shown (`<your site address>/dashboard`) and sign in with the owner email and password. Staff use `<your site address>/workspace`. The old `/owner` and `/owners` addresses redirect to `/dashboard`.

A sensible order for the first hour:

1. **Franchises:** add your first franchise. The full address is required, and you can add contact people.
2. **Services:** add the central catalogue (name, price before tax, time to complete in minutes). Every franchise receives it automatically.
3. **Opening hours:** pick the time zone and **slots** (how many bookings can start at the same time), set the weekly hours and mark holidays on the calendar.
4. **Taxes** and **Invoice template:** set the tax rates and how printed invoices look.
5. **Team:** invite staff and give them roles. They sign in at `/workspace`.
6. **Card payments:** under "Online payment keys" add your Square testing keys first, connect the Square account, take a test payment, then add the live keys.
7. **Dealerships** (if you serve them): add each with its per-car rate and contact people.

Every appointment you create also creates a draft invoice automatically. Check the prices on it before issuing.

## 7. HTTPS and your domain

The gateway is the only part that faces the network. Choose how HTTPS is provided in setup (you can change it by deleting `.env` and running setup again, or by editing `.env` carefully).

**Self-signed certificate (testing, a private network).** Setup creates a certificate for your site host. Browsers show a warning the first time; you can continue past it. Not suitable for the public internet.

**Your own certificate (public site, no proxy).** Point setup at the certificate and key files. With Let's Encrypt on Linux:

```
sudo certbot certonly --standalone -d dapper.example.com
```

Then in setup choose "Use my own certificate" and enter `/etc/letsencrypt/live/dapper.example.com/fullchain.pem` and `.../privkey.pem`, listen address `0.0.0.0:443`, site address `https://dapper.example.com`. Make sure the account running Dapper can read those files. Renew with certbot and restart Dapper after each renewal.

**Reverse proxy (recommended for production).** Choose "I run a reverse proxy". Dapper then listens on plain HTTP (for example `127.0.0.1:8443`) and your proxy serves the public HTTPS address and forwards to it, keeping the original `Host` header. The site address you enter in setup must be the public `https://` address.

Caddy (`/etc/caddy/Caddyfile`), which obtains certificates itself:

```
dapper.example.com {
    reverse_proxy 127.0.0.1:8443
}
```

nginx:

```
server {
    listen 443 ssl;
    server_name dapper.example.com;
    ssl_certificate     /etc/letsencrypt/live/dapper.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/dapper.example.com/privkey.pem;
    client_max_body_size 40m;
    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

Open the firewall for ports 80/443 only (never 3006, 8121 or 5432). Point your DNS A record at the server.

## 8. Keep it running

`scripts/start.*` runs in the foreground; stop it with Ctrl+C. To keep it running after you close the terminal:

**Linux (systemd).** Create `/etc/systemd/system/dapper.service`:

```
[Unit]
Description=Dapper
After=network.target postgresql.service

[Service]
User=dapper
WorkingDirectory=/opt/dapperdashboard
ExecStart=/bin/sh scripts/start.sh
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

then `sudo systemctl enable --now dapper`. Run the first start (setup) by hand before enabling the service, because setup needs a browser.

**Windows.** Create a Task Scheduler task that runs `powershell -File scripts\start.ps1` at startup, in the repository folder, after you have completed setup once.

Logs are in `.dapper/logs/` (`api.log`, `web.log`, `gateway.log`).

## 9. Updating and backups

**Back up first.** Two things matter: the database and the `.env` file (it holds the secret keys that decrypt stored secrets; a database restored without them cannot read them).

```
pg_dump -Fc -h localhost -U dapper dapper > dapper-backup.dump
copy .env somewhere safe       (cp .env on macOS/Linux)
```

**Update:**

```
git pull
cd dapper-platform/apps/web && npm ci && npm run build && cd ../../..     (only when the web code changed)
sh scripts/migrate.sh        (Windows: powershell -File scripts/migrate.ps1)   applies the database changes
sh scripts/start.sh
```

Delete `.dapper/bin` after an update that changed the Go code so the services are rebuilt.

If you forget `migrate`, `start` stops with a clear message telling you to run it.

**Restore:** create an empty database, `pg_restore -d dapper dapper-backup.dump`, put the saved `.env` back, and start Dapper.

## 10. Configuration reference

Everything is in `.env` at the repository root. Setup writes it; you normally never edit it. Values are in double quotes.

| Key | Meaning |
|---|---|
| `DAPPER_INSTALLED` | `1` marks a completed setup. Without it (or without the database/site/keys) setup opens first |
| `DAPPER_SITE_NAME`, `DAPPER_SITE_URL`, `DAPPER_TIMEZONE` | Business name, public address, default time zone |
| `DAPPER_LISTEN` | Gateway address, for example `127.0.0.1:8443` or `0.0.0.0:443` |
| `DAPPER_TLS_MODE` | `self-signed`, `files` or `proxy` |
| `DAPPER_TLS_CERT`, `DAPPER_TLS_KEY` | Certificate and key paths |
| `DAPPER_DATABASE_URL` | PostgreSQL address, user and password |
| `DAPPER_SMTP_HOST`, `_PORT`, `_USERNAME`, `_PASSWORD`, `_FROM_ADDRESS`, `_FROM_NAME` | Outgoing email (blank = off) |
| `DAPPER_MFA_KEY`, `DAPPER_INVITATION_ENCRYPTION_KEY` | Two generated 256-bit keys. Never change or lose them |
| `DAPPER_SESSION_LIFETIME` | How long a sign-in lasts (default `8h`) |

Other files and folders created next to it (all git-ignored): `.dapper/tls/` (self-signed certificate), `.dapper/uploads/` (staff photos), `.dapper/logs/`, `.dapper/bin/` (built services).

## 11. Security notes

- `.env` holds your database password, email password and the secret keys. It is created readable only by your user, is in `.gitignore`, and must never be committed or shared.
- The setup page only answers requests that carry the one-time setup code, only listens on this computer, refuses unexpected host names, locks out repeated wrong codes, and shuts itself down when it finishes.
- The owner account requires a 15+ character password. Turn on two-factor authentication for administrators when the dashboard asks.
- Square application secrets entered in the dashboard are encrypted with `DAPPER_MFA_KEY` and are never shown again.
- Keep PostgreSQL private (firewall port 5432), and keep the server patched.

## 12. Troubleshooting

| Problem | What to do |
|---|---|
| "go/node/npm was not found" | Install it (section 2) and open a **new** terminal so the PATH updates |
| Setup page did not open | Copy the address printed in the terminal (`http://localhost:8099`) |
| "That setup code is not right" | Use the code from the terminal window that is running setup. Stop with Ctrl+C and start again for a new code |
| "Could not connect" to the database | Check host/port/user/password; is PostgreSQL running? Does `pg_hba.conf` allow you? Try `psql -h HOST -U USER -d postgres` |
| "database already holds data" | Setup installs only into an empty database. Choose a new name, or drop the old one if it is safe |
| Port already in use | Another program uses 3006, 8121 or your listen port. Stop it, or choose a different listen address in setup |
| Browser warns about the certificate | Expected for self-signed. Use a real certificate or a reverse proxy for public use |
| "The database needs updating for this version" | Back up, then run `scripts/migrate` (section 9) |
| Web build fails | Make sure Node is 20+ and you have 2 GB free RAM; delete `dapper-platform/apps/web/.next` and `node_modules`, then start again |
| Emails are not sent | Re-check the SMTP server, port and (app) password; invitations stay queued until email works |
| Lists look out of date | Refresh once; the server sends no-cache headers, so a stuck page usually means an old browser tab after an update |
| Something else | Read `.dapper/logs/*.log` |

## 13. Reset or uninstall

- **Run setup again:** stop Dapper, delete `.env`, start again. Setup refuses a database that already holds data, so also drop the old database or choose a new name.
- **Uninstall:** stop Dapper, delete the repository folder, and drop the database (`DROP DATABASE dapper;`).

---

Developer notes: the product requirements, decisions and delivery status live in the `Dapper Detailing Knowledge` folder (open it as an Obsidian vault and start at `00 Home.md`). The home page can play a frame sequence instead of the 3D scene; see `main website/FRAME_SEQUENCE_SPEC.md`.

## Note for this public copy

The 3D car models (`*.glb`), the internal planning notes and the legacy BookingPress migration data are not part of this repository. The dashboards, API and installer do not need them. The public home page's 3D scene needs a car model at `main website/assets/three/` (rebuild with `build-pages.py`) or the photographic frame sequence (see `main website/home-frames.js`); without either, that scene shows no car.
