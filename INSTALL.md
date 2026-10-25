# Installing Dapper (like a WordPress install)

You need: **Git**, **Go 1.26+**, **Node 20+ with npm**, and an existing **PostgreSQL 15+** server you can reach (a user that can create a database, or an empty database someone created for you).

```
git clone <this repository>
cd <repository>
powershell -File scripts/setup.ps1      # Windows
sh scripts/setup.sh                     # macOS / Linux
```

`setup` builds the web app the first time, then prints a one-time **setup code** and an address such as `http://127.0.0.1:8099`. Open it, enter the code, and answer five short steps:

1. **Database**: host, port, database name, user, password. Test the connection; setup creates the database if it is missing and the user is allowed to, and only ever installs into an **empty** database.
2. **Site**: business name, the public site address, default timezone, the address Dapper listens on, and how HTTPS is provided (a self-signed certificate it creates, your own certificate files, or a reverse proxy in front).
3. **Email**: SMTP server, port, login and From address, with a **Send test** button. Skippable; invitations stay queued until email is configured.
4. **Owner**: the first account (email and a password of at least 15 characters). There is no default account.
5. **Install**: creates the tables, the owner, two generated 256-bit secret keys, and the private configuration. It cannot be run twice.

Then start it:

```
powershell -File scripts/start.ps1      # Windows
sh scripts/start.sh                     # macOS / Linux
```

Open `<your site address>/dashboard` and sign in as the owner. Staff use `<your site address>/workspace`. The old `/owner` and `/owners` addresses redirect to `/dashboard`.

## Where things are stored
Everything the installer writes goes in the `.dapper/` folder at the repository root, which git ignores and which is restricted to your user: `config.json` (database URL with its password, SMTP password, the keys), the certificate, uploads, logs and `installed.lock`. Back this folder up and never commit or share it. To run setup again, stop Dapper and delete `.dapper/` (this does not touch the database).

## Notes
- Setup is protected by the one-time code, listens on `127.0.0.1` only unless you pass `--listen`, refuses unexpected `Host` headers, and shuts itself down when it finishes.
- Square, Invoice Ninja and Mailchimp are configured later inside the dashboard, not in setup.
- The business timezone is stored as the default for new locations; each location keeps its own timezone.
- The public website is a separate part of this repository and is not wired to the dashboard by setup.
