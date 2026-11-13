"""Dapper legacy import MCP server (owner 2026-11-13).

Brings the live legacy website's data into the new clean Dapper database, in two steps that never mix:
  1. Read: the live site's own read-only export API (dapper-export/v1: manifest, catalogue, content, media), signed in with a WordPress
     Application Password, and the BookingPress export file the owner downloads from WordPress admin (customers, appointments, payments).
     Everything read is kept under .cache/legacy-import/ (git-ignored; it holds personal data). Nothing on the live site is changed.
  2. Import: every import is planned first (a dry run that writes nothing and reports counts), then applied only when the caller repeats
     the plan's checksum, in one transaction, with a batch id so the run can be reviewed or undone. The mapping is the approved one in
     scripts/import_bookingpress_history.py, scripts/import_bookingpress_invoices.py and cmd/seed-catalogue.

Settings (environment variables, never in chat; on Windows the user environment, on the server a root-only environment file):
  DAPPER_LEGACY_WP_USER, DAPPER_LEGACY_WP_APP_PASSWORD  the WordPress admin user and its Application Password
  DAPPER_IMPORT_DATABASE_URL                             the target database (on the server it defaults to the app's DAPPER_DATABASE_URL)
  DAPPER_IMPORT_MCP_TOKEN                                with --http only: the secret every request must carry (32+ characters)

Run locally over stdio (Claude Code starts it from .mcp.json), or on the production server with --http: it then listens on
127.0.0.1:8765 only, at /legacy-import/mcp, behind Caddy's HTTPS, and refuses any request without the token.
Tools return counts, checksums and file locations only, never customer details or credentials.
"""
from __future__ import annotations

import base64
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CACHE = ROOT / ".cache" / "legacy-import"
LIVE = "https://dapperdetailing.ca"
LIVE_HOSTS = {"dapperdetailing.ca", "www.dapperdetailing.ca"}
SECTIONS = ("manifest", "catalogue", "content", "media")
PARTS = ("catalogue_names", "catalogue_prices", "customers_bookings", "billing")
PSQL_DEFAULT = r"C:\Program Files\PostgreSQL\16\bin\psql.exe"
MEDIA_MAX_FILE = 25 * 1024 * 1024
MEDIA_MAX_TOTAL = 2 * 1024 * 1024 * 1024
UUID = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")


# ---------- settings ----------

def setting(name: str) -> str:
    """A setting from this process, else from the Windows user environment (shells started earlier may not have it)."""
    v = os.environ.get(name, "")
    if v or os.name != "nt":
        return v
    try:
        import winreg
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, "Environment") as k:
            return str(winreg.QueryValueEx(k, name)[0])
    except OSError:
        return ""


def sha256_file(p: Path) -> str:
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for block in iter(lambda: f.read(1 << 16), b""):
            h.update(block)
    return h.hexdigest()


def stamp() -> str:
    return time.strftime("%Y%m%d-%H%M%S")


# ---------- reading the live site ----------

def _auth_header() -> str:
    user, password = setting("DAPPER_LEGACY_WP_USER"), setting("DAPPER_LEGACY_WP_APP_PASSWORD")
    if not user or not password:
        raise RuntimeError("Set DAPPER_LEGACY_WP_USER and DAPPER_LEGACY_WP_APP_PASSWORD (a WordPress Application Password) in your Windows user environment first.")
    return "Basic " + base64.b64encode(f"{user}:{password.replace(' ', '')}".encode()).decode()


def _get_json(url: str, auth: str):
    req = urllib.request.Request(url, headers={"Authorization": auth, "Accept": "application/json", "User-Agent": "dapper-legacy-import"})
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return json.loads(r.read().decode("utf-8")), dict(r.headers)
    except urllib.error.HTTPError as e:
        if e.code in (401, 403):
            raise RuntimeError(f"The live site refused the sign-in (HTTP {e.code}). Check the WordPress user and Application Password.") from None
        raise RuntimeError(f"The live site answered HTTP {e.code} for {urllib.parse.urlsplit(url).path}.") from None


def items_of(page) -> list:
    """The records in one page of an export answer, whatever the wrapper (a list, or a dict holding the list)."""
    if isinstance(page, list):
        return page
    if isinstance(page, dict):
        for key in ("items", "data", "records", "results", "rows"):
            if isinstance(page.get(key), list):
                return page[key]
    return []


def fetch_section(section: str, auth: str, get=_get_json, per_page: int = 100, max_pages: int = 500) -> list | dict:
    base = f"{LIVE}/wp-json/dapper-export/v1/{section}"
    if section == "manifest":
        return get(base, auth)[0]
    out: list = []
    for page in range(1, max_pages + 1):
        body, headers = get(f"{base}?page={page}&per_page={per_page}", auth)
        batch = items_of(body)
        out.extend(batch)
        total_pages = int(headers.get("X-WP-TotalPages") or headers.get("x-wp-totalpages") or 0)
        if not batch or len(batch) < per_page or (total_pages and page >= total_pages):
            break
    return out


def media_url(item) -> str:
    if not isinstance(item, dict):
        return ""
    for key in ("source_url", "url", "file_url", "guid"):
        v = item.get(key)
        if isinstance(v, dict):
            v = v.get("rendered")
        if isinstance(v, str) and v:
            return v
    return ""


def allowed_media(url: str) -> bool:
    """Only files on the live site itself, over HTTPS: an export entry cannot make this server fetch from anywhere else."""
    u = urllib.parse.urlsplit(url)
    return u.scheme == "https" and u.hostname in LIVE_HOSTS and ".." not in u.path


def download_media(items: list, folder: Path) -> dict:
    folder.mkdir(parents=True, exist_ok=True)
    saved = skipped = 0
    total = 0
    for item in items:
        url = media_url(item)
        if not allowed_media(url):
            skipped += 1
            continue
        name = re.sub(r"[^A-Za-z0-9._-]", "_", urllib.parse.unquote(Path(urllib.parse.urlsplit(url).path).name))[:150] or "file"
        target = folder / name
        if target.exists():
            continue
        try:
            with urllib.request.urlopen(urllib.request.Request(url, headers={"User-Agent": "dapper-legacy-import"}), timeout=60) as r:
                data = r.read(MEDIA_MAX_FILE + 1)
        except (urllib.error.URLError, TimeoutError):
            skipped += 1
            continue
        if len(data) > MEDIA_MAX_FILE or total + len(data) > MEDIA_MAX_TOTAL:
            skipped += 1
            continue
        target.write_bytes(data)
        total += len(data)
        saved += 1
    return {"downloaded": saved, "skipped": skipped, "bytes": total, "folder": str(folder)}


# ---------- the BookingPress export file ----------

EXPORT_KEYS = ("customers", "appointments", "services", "category", "extra_services")


def load_export(path: str) -> dict:
    src = Path(path).expanduser()
    if not src.is_file():
        raise RuntimeError("Export file not found.")
    data = json.loads(src.read_text(encoding="utf-8"))
    missing = [k for k in EXPORT_KEYS if not isinstance(data.get(k), list)]
    if missing:
        raise RuntimeError("This is not a BookingPress export (missing: " + ", ".join(missing) + ").")
    CACHE.mkdir(parents=True, exist_ok=True)
    dest = CACHE / f"export-{stamp()}.json"
    shutil.copyfile(src, dest)
    return {"snapshot": dest.name, "sha256": sha256_file(dest), "site": data.get("site_url", ""), "counts": {k: len(data[k]) for k in EXPORT_KEYS}}


def latest_export() -> Path:
    found = sorted(CACHE.glob("export-*.json"))
    if not found:
        raise RuntimeError("Load a BookingPress export first (load_bookingpress_export).")
    return found[-1]


# ---------- planning and applying ----------

def target_url() -> str:
    url = setting("DAPPER_IMPORT_DATABASE_URL") or setting("DAPPER_DATABASE_URL")
    if not url.startswith(("postgres://", "postgresql://")):
        raise RuntimeError("Set DAPPER_IMPORT_DATABASE_URL (postgres://…) to the database the import goes into.")
    return url


def url_fingerprint(url: str) -> str:
    """Which database a plan was made for, without keeping its password."""
    u = urllib.parse.urlsplit(url)
    return hashlib.sha256(f"{u.hostname}:{u.port}{u.path}:{u.username}".encode()).hexdigest()[:16]


def psql() -> str:
    return shutil.which("psql") or PSQL_DEFAULT


def run(cmd: list[str], env: dict | None = None, cwd: Path = ROOT) -> str:
    p = subprocess.run(cmd, cwd=cwd, env={**os.environ, **(env or {})}, capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=1800)
    if p.returncode != 0:
        raise RuntimeError((p.stderr or p.stdout).strip()[-1500:] or "The command failed.")
    return p.stdout


def plan_path(plan_id: str) -> Path:
    if not re.fullmatch(r"[a-z_]+-\d{8}-\d{6}", plan_id):
        raise RuntimeError("Unknown plan.")
    return CACHE / "plans" / f"{plan_id}.json"


def make_plan(part: str, organization_id: str, location_id: str, actor_email: str = "", franchise_name: str = "") -> dict:
    if part not in PARTS:
        raise RuntimeError("part must be one of: " + ", ".join(PARTS))
    if not UUID.match(organization_id) or not UUID.match(location_id):
        raise RuntimeError("organization_id and location_id must be the target database's ids.")
    url = target_url()
    export = latest_export()
    plan_id = f"{part}-{stamp()}"
    (CACHE / "plans").mkdir(parents=True, exist_ok=True)
    plan = {"id": plan_id, "part": part, "export": export.name, "exportSha256": sha256_file(export), "target": url_fingerprint(url),
            "organizationId": organization_id, "locationId": location_id, "batch": f"bookingpress-{plan_id}", "applied": False}
    if part in ("customers_bookings", "billing"):
        script = "import_bookingpress_history.py" if part == "customers_bookings" else "import_bookingpress_invoices.py"
        sql = CACHE / "plans" / f"{plan_id}.sql"
        args = [sys.executable, str(ROOT / "scripts" / script), "--org", organization_id, "--loc", location_id, "--source", str(export), "--out", str(sql)]
        if part == "customers_bookings":
            args += ["--batch", plan["batch"]]
        out = run(args)
        plan["report"] = json.loads(out[: out.rfind("}") + 1] or "{}")
        plan["sql"] = sql.name
        plan["sha256"] = sha256_file(sql)
    else:
        if not actor_email or not franchise_name:
            raise RuntimeError("Catalogue imports need actor_email (an owner account in the target database) and franchise_name.")
        args = ["go", "run", "./cmd/seed-catalogue", "-email", actor_email, "-franchise", franchise_name, "-export", str(export)]
        if part == "catalogue_prices":
            args.append("-prices")
        plan["report"] = run(args, env={"DAPPER_DATABASE_URL": url}, cwd=ROOT / "dapper-platform" / "apps" / "api")[-4000:]
        plan["command"] = args
        plan["sha256"] = hashlib.sha256(json.dumps([args, plan["exportSha256"], plan["target"]]).encode()).hexdigest()
    plan_path(plan_id).write_text(json.dumps(plan, indent=1), encoding="utf-8")
    return {"plan": plan_id, "confirm": plan["sha256"][:12], "report": plan["report"], "writes": "nothing yet"}


def apply_plan(plan_id: str, confirm: str) -> dict:
    p = plan_path(plan_id)
    if not p.exists():
        raise RuntimeError("Unknown plan.")
    plan = json.loads(p.read_text(encoding="utf-8"))
    if plan.get("applied"):
        raise RuntimeError("This plan was already applied.")
    if confirm != plan["sha256"][:12]:
        raise RuntimeError("The confirmation does not match this plan. Use the 'confirm' value plan_import returned.")
    url = target_url()
    if url_fingerprint(url) != plan["target"]:
        raise RuntimeError("DAPPER_IMPORT_DATABASE_URL now points at a different database than the plan was made for. Plan again.")
    export = CACHE / plan["export"]
    if sha256_file(export) != plan["exportSha256"]:
        raise RuntimeError("The export changed since the plan was made. Plan again.")
    if "sql" in plan:
        sql = CACHE / "plans" / plan["sql"]
        if sha256_file(sql) != plan["sha256"]:
            raise RuntimeError("The planned SQL changed. Plan again.")
        out = run([psql(), "-v", "ON_ERROR_STOP=1", "-q", "-f", str(sql), url], env={"PGCLIENTENCODING": "UTF8"})
    else:
        out = run(plan["command"] + ["-apply"], env={"DAPPER_DATABASE_URL": url}, cwd=ROOT / "dapper-platform" / "apps" / "api")[-4000:]
    plan["applied"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    p.write_text(json.dumps(plan, indent=1), encoding="utf-8")
    return {"plan": plan_id, "applied": True, "batch": plan["batch"], "output": out.strip()[-2000:]}


# Undo one customers_bookings batch: its billing rows, events and bookings, then the customers it created (not the ones it only matched by
# email). One transaction; if anything has since been attached to those records (a new invoice, a new booking), PostgreSQL refuses and
# nothing is removed.
UNDO_COUNT_SQL = """SELECT
 (SELECT count(*) FROM legacy_import_map WHERE batch=:'batch' AND source_kind='booking') AS bookings,
 (SELECT count(*) FROM legacy_import_map WHERE batch=:'batch' AND source_kind='customer' AND note='') AS customers_created,
 (SELECT count(*) FROM legacy_booking_billing WHERE booking_id IN (SELECT target_id FROM legacy_import_map WHERE batch=:'batch' AND source_kind='booking')) AS billing_rows;"""
UNDO_SQL = """BEGIN;
DELETE FROM legacy_booking_billing WHERE booking_id IN (SELECT target_id FROM legacy_import_map WHERE batch=:'batch' AND source_kind='booking');
DELETE FROM booking_events WHERE booking_id IN (SELECT target_id FROM legacy_import_map WHERE batch=:'batch' AND source_kind='booking');
DELETE FROM bookings WHERE id IN (SELECT target_id FROM legacy_import_map WHERE batch=:'batch' AND source_kind='booking');
DELETE FROM customers WHERE id IN (SELECT target_id FROM legacy_import_map WHERE batch=:'batch' AND source_kind='customer' AND note='');
DELETE FROM legacy_import_map WHERE batch=:'batch';
COMMIT;"""


def undo_batch(batch: str, confirm: str = "") -> dict:
    if not re.fullmatch(r"bookingpress-[a-z_]+-\d{8}-\d{6}", batch):
        raise RuntimeError("Only batches made by this server can be undone here.")
    url = target_url()
    tmp =CACHE / "plans" / f"undo-count-{stamp()}.sql"
    tmp.parent.mkdir(parents=True, exist_ok=True)
    tmp.write_text(UNDO_COUNT_SQL, encoding="utf-8")
    row = run([psql(), "-v", "ON_ERROR_STOP=1", "-v", f"batch={batch}", "-At", "-F", ",", "-f", str(tmp), url]).strip().split(",")
    tmp.unlink(missing_ok=True)
    found = dict(zip(("bookings", "customersCreated", "billingRows"), (int(x) for x in row)))
    code = hashlib.sha256(f"{batch}:{url_fingerprint(url)}".encode()).hexdigest()[:12]
    if confirm != code:
        return {"batch": batch, "wouldRemove": found, "confirm": code, "removed": "nothing yet"}
    tmp.write_text(UNDO_SQL, encoding="utf-8")
    run([psql(), "-v", "ON_ERROR_STOP=1", "-v", f"batch={batch}", "-q", "-f", str(tmp), url])
    tmp.unlink(missing_ok=True)
    return {"batch": batch, "removed": found}


def status() -> dict:
    out = {
        "liveSiteSignIn": bool(setting("DAPPER_LEGACY_WP_USER") and setting("DAPPER_LEGACY_WP_APP_PASSWORD")),
        "targetDatabaseSet": bool(setting("DAPPER_IMPORT_DATABASE_URL") or setting("DAPPER_DATABASE_URL")),
        "psql": Path(psql()).exists(),
        "go": bool(shutil.which("go")),
        "exports": [p.name for p in sorted(CACHE.glob("export-*.json"))],
        "liveSnapshots": [p.name for p in sorted(CACHE.glob("live-*")) if p.is_dir()],
        "plans": [],
    }
    for p in sorted((CACHE / "plans").glob("*.json")):
        try:
            d = json.loads(p.read_text(encoding="utf-8"))
            out["plans"].append({"plan": d["id"], "applied": d["applied"], "batch": d["batch"]})
        except (OSError, ValueError, KeyError):
            pass
    if out["targetDatabaseSet"] and out["psql"]:
        try:
            out["targetDatabase"] = run([psql(), "-At", "-c", "SELECT current_database()||', migration '||coalesce((SELECT max(version) FROM dapper_schema_migrations)::text,'unknown')", target_url()]).strip()
            # Where an import can go: organization and franchise ids with their names (no customer data).
            rows = run([psql(), "-At", "-F", "|", "-c", "SELECT o.id, o.name, l.id, l.name FROM organizations o JOIN locations l ON l.organization_id=o.id ORDER BY o.name, l.name LIMIT 50", target_url()])
            out["franchises"] = [dict(zip(("organizationId", "organization", "locationId", "franchise"), r.split("|", 3))) for r in rows.splitlines() if r.count("|") == 3]
        except RuntimeError as e:
            out["targetDatabase"] = "not reachable: " + str(e)[:200]
    return out


# ---------- MCP ----------

HTTP_PATH = "/legacy-import/mcp"


def build_server(http: bool = False):
    from mcp.server.fastmcp import FastMCP
    from mcp.server.transport_security import TransportSecuritySettings

    # Behind Caddy the Host is the public domain, so the library's localhost-only host check is off; the token guards every request.
    mcp = FastMCP("dapper-legacy-import", host="127.0.0.1", port=8765, streamable_http_path=HTTP_PATH,
                  transport_security=TransportSecuritySettings(enable_dns_rebinding_protection=False) if http else None)

    @mcp.tool()
    def legacy_import_status() -> dict:
        """What is set up (live-site sign-in, target database, psql, go), which exports and live snapshots are cached, and every plan with
        whether it was applied. Never shows credentials."""
        return status()

    @mcp.tool()
    def check_live_site() -> dict:
        """Sign in to the live site's read-only export API and return its manifest (what it can export and how much)."""
        return {"manifest": fetch_section("manifest", _auth_header())}

    @mcp.tool()
    def fetch_live_site(sections: list[str] | None = None, download_media_files: bool = False) -> dict:
        """Read the live site's catalogue, content and media list (read-only) into a new local snapshot under .cache/legacy-import/.
        With download_media_files, also download the media files themselves (only from dapperdetailing.ca, max 25 MB each, 2 GB total).
        Returns counts and checksums, not the data."""
        auth = _auth_header()
        chosen = [s for s in (sections or list(SECTIONS)) if s in SECTIONS]
        folder = CACHE / f"live-{stamp()}"
        folder.mkdir(parents=True, exist_ok=True)
        result: dict = {"snapshot": folder.name, "sections": {}}
        for s in chosen:
            data = fetch_section(s, auth)
            f = folder / f"{s}.json"
            f.write_text(json.dumps(data, ensure_ascii=False, indent=1), encoding="utf-8")
            result["sections"][s] = {"records": len(data) if isinstance(data, list) else len(data.keys()), "sha256": sha256_file(f)[:16]}
            if s == "media" and download_media_files and isinstance(data, list):
                result["mediaFiles"] = download_media(data, folder / "media")
        return result

    @mcp.tool()
    def load_bookingpress_export(path: str) -> dict:
        """Check and copy a BookingPress export file (WordPress admin → BookingPress → Settings → Import/Export → Export) into the local
        cache. Returns how many customers, appointments, services, categories and add-ons it holds."""
        return load_export(path)

    @mcp.tool()
    def plan_import(part: str, organization_id: str, location_id: str, actor_email: str = "", franchise_name: str = "") -> dict:
        """Dry run of one import into DAPPER_IMPORT_DATABASE_URL from the newest loaded export. Writes nothing to the database.
        part: catalogue_names (services and add-ons, names only), catalogue_prices (prices onto them), customers_bookings (customers and
        appointment history), billing (what each imported appointment cost and how it was paid). Catalogue parts also need actor_email and
        franchise_name. Run them in that order. Returns a report and the 'confirm' code that apply_import needs."""
        return make_plan(part, organization_id, location_id, actor_email, franchise_name)

    @mcp.tool()
    def apply_import(plan: str, confirm: str) -> dict:
        """Apply exactly one planned import, in one transaction. Only after the person has reviewed the plan's report and agreed; confirm
        is the code plan_import returned. Refuses if the export, the planned SQL or the target database changed since planning."""
        return apply_plan(plan, confirm)

    @mcp.tool()
    def undo_import(batch: str, confirm: str = "") -> dict:
        """Undo one customers_bookings batch. Without confirm: shows what would be removed and the confirm code. With it: removes them in
        one transaction (refused, with nothing removed, if anything new has been attached to those records since)."""
        return undo_batch(batch, confirm)

    return mcp


def token_guard(app, token: str):
    """Every HTTP request must carry 'Authorization: Bearer <token>' (compared in constant time); anything else gets 401."""
    import hmac
    want = ("Bearer " + token).encode()

    async def guarded(scope, receive, send):
        if scope["type"] == "http":
            got = dict(scope.get("headers") or []).get(b"authorization", b"")
            if not hmac.compare_digest(got, want):
                await send({"type": "http.response.start", "status": 401, "headers": [(b"content-type", b"text/plain")]})
                await send({"type": "http.response.body", "body": b"Unauthorized"})
                return
        await app(scope, receive, send)
    return guarded


def serve_http():
    token = setting("DAPPER_IMPORT_MCP_TOKEN")
    if len(token) < 32:
        sys.exit("Set DAPPER_IMPORT_MCP_TOKEN (at least 32 random characters) before serving over HTTP.")
    import uvicorn
    app = token_guard(build_server(http=True).streamable_http_app(), token)
    uvicorn.run(app, host="127.0.0.1", port=8765, log_level="warning")


if __name__ == "__main__":
    if "--http" in sys.argv:
        serve_http()
    else:
        build_server().run()
