#!/usr/bin/env python3
"""Fill the on-demand billing of the BookingPress history that import_bookingpress_history.py brought in.

Approved choices (2026-11-05): every imported booking has an invoice, but it is built when someone opens it: this script stores only one small row of the
figures the old site recorded per booking (legacy_booking_billing), never an invoice, payment or event row, so nothing is added to reports.
Amounts are the totals the old site recorded; invoices are numbered <issuer prefix> + the old site's invoice number (a number shared by several
appointments gets -2, -3 on the later ones); completed bookings read as paid in full for the recorded paid amount (on-site and manual as cash, Square as card);
cancelled bookings read as void. The issuer block comes from the franchise's issuer when the invoice is opened, so correcting Business details later corrects them all.

It writes a SQL file (it names no customers, but keep it under .cache/ anyway). Re-running is safe: a booking that already has a row is skipped.
  python scripts/import_bookingpress_invoices.py --org <uuid> --loc <uuid> --out .cache/billing.sql
Then run it with psql -v ON_ERROR_STOP=1 -1 -f .cache/billing.sql
"""
import argparse, hashlib, json, re, uuid
from collections import Counter
from decimal import Decimal, ROUND_HALF_UP

NS = uuid.UUID("3fa85f64-5717-4562-b3fc-2c963f66afa6")
SRC = ".cache/bookingpress-export/bookingpress_export_raw.json"
METHOD = {"square": "square_card"}  # everything else the old site recorded (on-site, manual) is read as cash


def q(v):
    return "NULL" if v is None else "'" + str(v).replace("'", "''") + "'"


def uid(kind, sid):
    return str(uuid.uuid5(NS, f"{kind}:{sid}"))


def clean(v):
    return re.sub(r"\s+", " ", (v or "").strip())


def cents(v):
    try:
        return int((Decimal(str(v or "0").strip() or "0") * 100).quantize(Decimal(1), rounding=ROUND_HALF_UP))
    except Exception:
        return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--org", required=True); ap.add_argument("--loc", required=True)
    ap.add_argument("--source", default=SRC); ap.add_argument("--out", required=True)
    a = ap.parse_args()
    d = json.load(open(a.source, encoding="utf8"))
    svc = {s["bookingpress_service_id"]: s for s in d["services"]}
    out, rep = [], Counter()
    used = Counter()
    top = 0
    for ap_ in d["appointments"]:
        bid = ap_["bookingpress_appointment_booking_id"]
        pay = ap_.get("payment_data") or {}
        st = ap_["bookingpress_appointment_status"]
        if st not in ("6", "1", "3", "4"):
            rep["skipped (unknown status)"] += 1; continue
        cancelled = st in ("3", "4")
        legacy = clean(str(pay.get("bookingpress_invoice_id") or "")) or bid
        top = max(top, int(legacy)) if legacy.isdigit() else top
        used[legacy] += 1
        suffix = "" if used[legacy] == 1 else f"-{used[legacy]}"
        if suffix: rep["number shared with an earlier appointment (suffix added)"] += 1
        total = cents(ap_["bookingpress_total_amount"]); tax = cents(ap_["bookingpress_tax_amount"]); disc = cents(ap_["bookingpress_coupon_discount_amount"])
        subtotal = max(0, total - tax + disc)
        paid = 0 if cancelled else min(cents(ap_["bookingpress_paid_amount"]), total)
        gateway = clean(pay.get("bookingpress_payment_gateway")).strip("- ") or "not recorded"
        method = METHOD.get(gateway.lower(), "cash")
        sv = svc.get(ap_["bookingpress_service_id"])
        sname = clean(sv["bookingpress_service_name"] if sv else pay.get("bookingpress_service_name")) or "Service (name not recorded)"
        price = min(cents(ap_["bookingpress_service_price"]), subtotal)
        extras = []
        try:
            raw = ap_["bookingpress_extra_service_details"]
            for e in (json.loads(raw.replace('\\"', '"')) if isinstance(raw, str) and raw.strip() not in ("", "[]") else (raw if isinstance(raw, list) else [])):
                nm = clean(e.get("bookingpress_extra_service_details", {}).get("bookingpress_extra_service_name") if isinstance(e, dict) and isinstance(e.get("bookingpress_extra_service_details"), dict) else e.get("bookingpress_extra_service_name") if isinstance(e, dict) else "")
                if nm: extras.append(nm)
        except Exception:
            rep["add-on details unreadable"] += 1
        items = [(sname[:255], price)]
        rest = subtotal - price
        if rest > 0:
            items.append((("Add-ons and extras (as recorded): " + ", ".join(extras))[:255] if extras else "Add-ons and adjustments (as recorded by the old site)", rest))
        code = ""
        try:
            cd = json.loads(str(ap_.get("bookingpress_coupon_details") or "").replace('\\"', '"') or "{}")
            code = clean(cd.get("coupon_data", {}).get("bookingpress_coupon_code"))[:32] if disc else ""
        except Exception:
            pass
        note = f"Imported from BookingPress appointment #{bid} (old invoice #{legacy}). Recorded total ${total/100:.2f}, paid ${cents(ap_['bookingpress_paid_amount'])/100:.2f}, payment gateway: {gateway}."
        if cancelled: note += " The booking was cancelled, so this invoice is void."
        final = "void" if cancelled else ("paid" if total - paid == 0 else "partially_paid")
        item_json = json.dumps([{"description": d_, "quantity": 1, "unitPriceCents": amt} for d_, amt in items])
        out.append(
            f"INSERT INTO legacy_booking_billing(booking_id,organization_id,location_id,source_id,invoice_number,status,items,subtotal_cents,discount_cents,coupon_code,tax_cents,total_cents,paid_cents,payment_method,gateway,note) "
            f"SELECT b.id,b.organization_id,b.location_id,{q(bid)},"
            f"COALESCE((SELECT s.number_prefix FROM location_invoice_issuers li JOIN invoice_issuers s ON s.organization_id=li.organization_id AND s.id=li.issuer_id WHERE li.organization_id=b.organization_id AND li.location_id=b.location_id),'DDP')||{q(legacy + suffix)},"
            f"{q(final)},{q(item_json)}::jsonb,{subtotal},{disc},{q(code)},{tax},{total},{paid},{q('' if cancelled or paid == 0 else method)},{q(gateway)},{q(note[:2000])} "
            f"FROM bookings b JOIN legacy_import_map m ON m.target_id=b.id AND m.source_system='bookingpress' AND m.source_kind='booking' AND m.source_id={q(bid)} AND m.organization_id={q(a.org)} "
            f"ON CONFLICT DO NOTHING;")
        rep["invoices " + final] += 1
    out.append(
        f"UPDATE invoice_issuers s SET next_number=GREATEST(s.next_number,{top + 1}) FROM location_invoice_issuers li "
        f"WHERE li.organization_id={q(a.org)} AND li.location_id={q(a.loc)} AND s.organization_id=li.organization_id AND s.id=li.issuer_id;")
    rep["highest old invoice number"] = top
    open(a.out, "w", encoding="utf8").write("-- BookingPress on-demand billing import. Do not commit.\nBEGIN;\n" + "\n".join(out) + "\nCOMMIT;\n")
    print(json.dumps(dict(rep), indent=1))
    print("sql sha256", hashlib.sha256(open(a.out, "rb").read()).hexdigest()[:16])


main()
