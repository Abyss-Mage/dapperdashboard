"""Build public catalogue references from an already authorized local export.

No network access, private records, policy approval or transactional import.
"""
import hashlib
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SERVICE_FIELDS = {
    "bookingpress_service_id", "bookingpress_category_id", "bookingpress_service_name",
    "bookingpress_service_description", "bookingpress_service_price",
    "bookingpress_service_duration_val", "bookingpress_service_duration_unit",
}
CATEGORY_FIELDS = {"bookingpress_category_id", "bookingpress_category_name"}


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def identifier(value):
    if not isinstance(value, str) or not re.fullmatch(r"[1-9][0-9]*", value):
        raise ValueError("Missing or invalid original source ID")
    return value


def text(value, maximum=20000):
    if not isinstance(value, str) or len(value) > maximum:
        raise ValueError("Invalid source text field")
    return value


def rows(resource):
    if resource.get("available") is not True or resource.get("total_pages") != 1:
        raise ValueError("Complete single-page catalogue export required")
    items = resource.get("items")
    if not isinstance(items, list) or len(items) != resource.get("total"):
        raise ValueError("Source row count mismatch")
    return items


def build(payload, manifest, source_hash):
    if payload.get("pricing_approval") != "reference_only_pending_client_approval":
        raise ValueError("Unreviewed source approval state")
    if manifest.get("site_url") != "https://dapperdetailing.ca/" or manifest.get("read_only") is not True:
        raise ValueError("Unexpected source site or export mode")
    categories, services, seen = [], [], set()
    for row in rows(payload["resources"]["categories"]):
        if set(row) != CATEGORY_FIELDS:
            raise ValueError("Unexpected category columns; refuse to copy them")
        source_id = identifier(row["bookingpress_category_id"])
        if source_id in seen:
            raise ValueError("Duplicate category ID")
        seen.add(source_id)
        categories.append({"sourceId": source_id, "name": text(row["bookingpress_category_name"], 250)})
    category_ids, seen = seen, set()
    for row in rows(payload["resources"]["services"]):
        if set(row) != SERVICE_FIELDS:
            raise ValueError("Unexpected service columns; refuse to copy them")
        source_id = identifier(row["bookingpress_service_id"])
        if source_id in seen:
            raise ValueError("Duplicate service ID")
        seen.add(source_id)
        category_id = identifier(row["bookingpress_category_id"])
        price = text(row["bookingpress_service_price"], 50)
        duration = text(row["bookingpress_service_duration_val"], 30)
        unit = text(row["bookingpress_service_duration_unit"], 30)
        issues = []
        if category_id not in category_ids:
            issues.append("unmapped_category")
        if not re.fullmatch(r"[0-9]+(?:\.[0-9]+)?", price):
            issues.append("unrecognized_price")
        if not re.fullmatch(r"[0-9]+", duration) or int(duration) <= 0:
            issues.append("invalid_duration")
        elif unit == "m" and int(duration) == 1:
            issues.append("one_minute_duration_review")
        if unit not in ("m", "h", "d"):
            issues.append("unrecognized_duration_unit")
        services.append({
            "sourceId": source_id, "categorySourceId": category_id,
            "name": text(row["bookingpress_service_name"], 500),
            "description": text(row["bookingpress_service_description"]),
            "priceRaw": price, "durationRaw": duration, "durationUnitRaw": unit,
            "reviewFlags": issues,
        })
    return {
        "schemaVersion": 1, "approvalStatus": "reference_only_pending_client_approval",
        "transactionEnabled": False, "currency": "not_explicitly_confirmed",
        "source": "https://dapperdetailing.ca/wp-json/dapper-export/v1/catalogue",
        "sourceSha256": source_hash, "capturedAt": manifest["generated_at"],
        "sourceTimezone": manifest["timezone"], "categories": categories, "services": services,
        "reviewCount": sum(bool(s["reviewFlags"]) for s in services),
    }


def main():
    cache = ROOT / ".cache" / "wordpress-export"
    evidence = read_json(ROOT / "Dapper Detailing Knowledge" / "99 Sources" / "wordpress-export-evidence.json")
    for filename in ("manifest.json", "catalogue-page-1.json"):
        expected = next(item["sha256"] for item in evidence["files"] if item["file"] == filename)
        if hashlib.sha256((cache / filename).read_bytes()).hexdigest() != expected:
            raise ValueError("Cached export hash differs from verified evidence")
    source_hash = hashlib.sha256((cache / "catalogue-page-1.json").read_bytes()).hexdigest()
    result = build(read_json(cache / "catalogue-page-1.json"), read_json(cache / "manifest.json"), source_hash)
    target = ROOT / "dapper-platform" / "apps" / "api" / "data" / "bookingpress-reference.json"
    target.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"Built {len(result['services'])} service / {len(result['categories'])} category references; {result['reviewCount']} services flagged for review. Checkout disabled.")


if __name__ == "__main__":
    main()
