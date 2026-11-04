#!/usr/bin/env python3
"""Import legacy BookingPress customers and appointments (history only) into the current Dapper schema.

Approved mapping (2026-11-04): status 6 -> completed, 1 -> confirmed, 3 and 4 -> cancelled (unverified, flagged in the note);
every date and time read as America/Toronto local time; a phone shared by several customers stays on the first only;
appointments whose service is missing from the export keep the recorded service name as text; no invoices or payments.
Bookings are history: nothing here sends mail or text, takes payment, or holds capacity.

It writes a SQL file (it contains customer details: keep it under .cache/, never commit it) and, with --apply, runs it in one
transaction through psql. Re-running is safe: every record is mapped in legacy_import_map and skipped when already imported.
  python scripts/import_bookingpress_history.py --org <uuid> --loc <uuid> --out .cache/import.sql        (dry run: SQL + report)
Then run the SQL with psql -v ON_ERROR_STOP=1 -1 -f .cache/import.sql
"""
import argparse, hashlib, json, re, sys, uuid
from collections import Counter
from datetime import datetime
from zoneinfo import ZoneInfo

NS = uuid.UUID("3fa85f64-5717-4562-b3fc-2c963f66afa6")
SRC = ".cache/bookingpress-export/bookingpress_export_raw.json"
BATCH = "bookingpress-history-2026-11-04"
STATUS = {"6": "completed", "1": "confirmed", "3": "cancelled", "4": "cancelled"}
TORONTO = ZoneInfo("America/Toronto")

def q(v):
    return "NULL" if v is None else "'" + str(v).replace("'", "''") + "'"

def uid(kind, sid):
    return str(uuid.uuid5(NS, f"{kind}:{sid}"))

def clean(v):
    return re.sub(r"\s+", " ", (v or "").strip())

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--org", required=True); ap.add_argument("--loc", required=True)
    ap.add_argument("--source", default=SRC); ap.add_argument("--out", required=True)
    a = ap.parse_args()
    d = json.load(open(a.source, encoding="utf8"))
    svc = {s["bookingpress_service_id"]: s for s in d["services"]}
    out, rep = [], Counter()
    seen_phone, seen_email = set(), set()
    names = {}
    for c in d["customers"]:
        email = clean(c["bookingpress_user_email"]).lower()
        name = clean(c["bookingpress_customer_full_name"]) or clean((c["bookingpress_user_firstname"] or "") + " " + (c["bookingpress_user_lastname"] or ""))
        if len(name) < 2: name = "Customer " + c["bookingpress_customer_id"]; rep["customer name filled"] += 1
        name = name[:100]
        phone = clean(c["bookingpress_user_phone"]); digits = re.sub(r"\D", "", phone)
        if digits and digits in seen_phone:
            phone = ""; rep["phone blanked (shared)"] += 1
        elif digits: seen_phone.add(digits)
        if email in seen_email: rep["customer skipped (duplicate email)"] += 1; continue
        seen_email.add(email)
        cid = c["bookingpress_customer_id"]; names[cid] = name
        created = clean(c["bookingpress_user_created"]) or "2024-01-01 00:00:00"
        tid = uid("customer", cid)
        out.append(
            f"INSERT INTO customers(id,organization_id,location_id,name,email,phone,created_at) SELECT {q(tid)},{q(a.org)},{q(a.loc)},{q(name)},{q(email)},"
            f"CASE WHEN regexp_replace({q(phone)},'\\D','','g')<>'' AND EXISTS(SELECT 1 FROM customers WHERE organization_id={q(a.org)} AND location_id={q(a.loc)} AND removed_at IS NULL AND regexp_replace(phone,'\\D','','g')=regexp_replace({q(phone)},'\\D','','g')) THEN '' ELSE {q(phone)} END,"
            f"{q(created)}::timestamptz WHERE NOT EXISTS(SELECT 1 FROM legacy_import_map WHERE source_system='bookingpress' AND source_kind='customer' AND source_id={q(cid)} AND organization_id={q(a.org)})"
            f" AND NOT EXISTS(SELECT 1 FROM customers WHERE organization_id={q(a.org)} AND location_id={q(a.loc)} AND lower(btrim(email))={q(email)});")
        # Map to the new row, or to the customer who already had this email.
        out.append(
            f"INSERT INTO legacy_import_map(source_system,source_kind,source_id,organization_id,target_id,batch,note) SELECT 'bookingpress','customer',{q(cid)},{q(a.org)},id,{q(BATCH)},CASE WHEN id={q(tid)} THEN '' ELSE 'matched an existing customer by email' END "
            f"FROM customers WHERE organization_id={q(a.org)} AND location_id={q(a.loc)} AND lower(btrim(email))={q(email)} ON CONFLICT DO NOTHING;")
        rep["customers"] += 1
    for ap_ in d["appointments"]:
        bid = ap_["bookingpress_appointment_booking_id"]
        cid = ap_["bookingpress_customer_id"]
        if cid not in names: rep["booking skipped (no customer)"] += 1; continue
        st = STATUS.get(ap_["bookingpress_appointment_status"])
        if not st: rep["booking skipped (unknown status)"] += 1; continue
        pay = ap_.get("payment_data") or {}
        sv = svc.get(ap_["bookingpress_service_id"])
        sname = clean(sv["bookingpress_service_name"] if sv else pay.get("bookingpress_service_name")) or "Service (name not recorded)"
        if not sv: rep["booking with missing service (name kept as text)"] += 1
        when = datetime.strptime(ap_["bookingpress_appointment_date"] + " " + ap_["bookingpress_appointment_time"], "%Y-%m-%d %H:%M:%S").replace(tzinfo=TORONTO)
        extras = []
        try:
            raw = ap_["bookingpress_extra_service_details"]
            for e in (json.loads(raw.replace('\\"', '"')) if isinstance(raw, str) and raw.strip() not in ("", "[]") else (raw if isinstance(raw, list) else [])):
                nm = clean(e.get("bookingpress_extra_service_details", {}).get("bookingpress_extra_service_name") if isinstance(e, dict) and isinstance(e.get("bookingpress_extra_service_details"), dict) else e.get("bookingpress_extra_service_name") if isinstance(e, dict) else "")
                if nm: extras.append(nm)
        except Exception:
            rep["add-on details unreadable"] += 1
        note = [f"Imported from BookingPress appointment #{bid}.", f"Recorded service price ${ap_['bookingpress_service_price']}, total ${ap_['bookingpress_total_amount']}, paid ${ap_['bookingpress_paid_amount']}."]
        if extras: note.append("Add-ons: " + ", ".join(extras) + ".")
        if ap_["bookingpress_appointment_status"] in ("3", "4"): note.append("Legacy status " + ap_["bookingpress_appointment_status"] + " read as cancelled (not verified).")
        internal = clean(ap_.get("bookingpress_appointment_internal_note"))
        if internal and internal != "None": note.append("Legacy staff note: " + internal)
        text = " ".join(note)[:4000]
        tid = uid("booking", bid)
        created = clean(ap_["bookingpress_created_at"]) or "2024-01-01 00:00:00"
        out.append(
            f"INSERT INTO bookings(id,organization_id,location_id,customer_id,service_name,addons,channel,status,booking_time,notes,created_at,updated_at) "
            f"SELECT {q(tid)},{q(a.org)},{q(a.loc)},m.target_id,{q(sname[:120])},'[]'::jsonb,'online',{q(st)},{q(when.isoformat())}::timestamptz,{q(text)},{q(created)}::timestamptz,now() "
            f"FROM legacy_import_map m WHERE m.source_system='bookingpress' AND m.source_kind='customer' AND m.source_id={q(cid)} AND m.organization_id={q(a.org)} "
            f"AND NOT EXISTS(SELECT 1 FROM legacy_import_map WHERE source_system='bookingpress' AND source_kind='booking' AND source_id={q(bid)} AND organization_id={q(a.org)});")
        out.append(
            f"INSERT INTO legacy_import_map(source_system,source_kind,source_id,organization_id,target_id,batch) SELECT 'bookingpress','booking',{q(bid)},{q(a.org)},id,{q(BATCH)} FROM bookings WHERE id={q(tid)} ON CONFLICT DO NOTHING;")
        out.append(
            f"INSERT INTO booking_events(booking_id,organization_id,location_id,from_status,to_status,notes) SELECT id,organization_id,location_id,NULL,status,'Imported from BookingPress' FROM bookings WHERE id={q(tid)} AND NOT EXISTS(SELECT 1 FROM booking_events WHERE booking_id={q(tid)});")
        rep["bookings " + st] += 1
    open(a.out, "w", encoding="utf8").write("-- BookingPress history import. Contains customer details; do not commit.\nBEGIN;\n" + "\n".join(out) + "\nCOMMIT;\n")
    print(json.dumps(dict(rep), indent=1))
    print("sql sha256", hashlib.sha256(open(a.out, "rb").read()).hexdigest()[:16])

main()
