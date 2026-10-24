"""Reconcile authorized paginated catalogue captures; no network or policy changes."""
import hashlib
import json
import math
import re
from pathlib import Path
from build_bookingpress_reference import build, identifier, text

ROOT = Path(__file__).resolve().parents[1]
EXTRA_FIELDS = {
    'bookingpress_extra_services_id', 'bookingpress_service_id',
    'bookingpress_extra_service_name', 'bookingpress_service_description',
    'bookingpress_extra_service_price', 'bookingpress_extra_service_duration',
    'bookingpress_extra_service_duration_unit', 'bookingpress_extra_service_min_quantity',
    'bookingpress_extra_service_max_quantity',
}


def reconcile(pages, manifest, digest):
    if manifest.get('exporter_version') != '0.1.3':
        raise ValueError('Reviewed exporter version required')
    if not pages or [page.get('page') for page in pages] != list(range(1, len(pages) + 1)):
        raise ValueError('Missing, duplicate or out-of-order pages')
    limit = pages[0].get('per_page')
    if not isinstance(limit, int) or not 1 <= limit <= 100:
        raise ValueError('Invalid page size')
    combined = {}
    for resource in ('services', 'categories', 'extras'):
        first = pages[0]['resources'][resource]
        total = first.get('total')
        if type(total) is not int or total < 0:
            raise ValueError('Invalid resource count')
        count = math.ceil(total / limit)
        if len(pages) < count:
            raise ValueError('Incomplete resource pages')
        items = []
        for index, page in enumerate(pages):
            if page.get('pricing_approval') != 'reference_only_pending_client_approval' or page.get('per_page') != limit:
                raise ValueError('Export contract changed between pages')
            value = page['resources'][resource]
            expected = min(limit, max(total - index * limit, 0))
            if value.get('available') is not True or value.get('total') != total or value.get('total_pages') != count or len(value.get('items', [])) != expected:
                raise ValueError('Resource availability/count changed during pagination')
            items.extend(value['items'])
        combined[resource] = {'available': True, 'total': total, 'total_pages': 1, 'items': items}
    base = build({'pricing_approval': 'reference_only_pending_client_approval', 'resources': combined}, manifest, digest)
    service_ids = {item['sourceId'] for item in base['services']}
    extras, seen = [], set()
    for row in combined['extras']['items']:
        if set(row) != EXTRA_FIELDS:
            raise ValueError('Unexpected extra fields; refuse private/unknown columns')
        source_id = identifier(row['bookingpress_extra_services_id'])
        parent_id = identifier(row['bookingpress_service_id'])
        if source_id in seen:
            raise ValueError('Duplicate original add-on ID')
        seen.add(source_id)
        price = text(row['bookingpress_extra_service_price'], 50)
        duration = text(row['bookingpress_extra_service_duration'], 30)
        unit = text(row['bookingpress_extra_service_duration_unit'], 30)
        minimum = text(row['bookingpress_extra_service_min_quantity'], 30)
        maximum = text(row['bookingpress_extra_service_max_quantity'], 30)
        flags = []
        if parent_id not in service_ids:
            flags.append('unmapped_parent_service')
        if not re.fullmatch(r'[0-9]+(?:\.[0-9]+)?', price):
            flags.append('unrecognized_price')
        if not re.fullmatch(r'[0-9]+', duration) or int(duration) <= 0:
            flags.append('missing_or_zero_duration_review')
        if unit not in ('m', 'h', 'd'):
            flags.append('unrecognized_duration_unit')
        if not re.fullmatch(r'[0-9]+', minimum) or not re.fullmatch(r'[0-9]+', maximum) or int(minimum) < 1 or int(maximum) < int(minimum):
            flags.append('quantity_range_review')
        extras.append({'sourceId': source_id, 'parentServiceSourceId': parent_id,
                       'name': text(row['bookingpress_extra_service_name'], 500),
                       'description': text(row['bookingpress_service_description']),
                       'priceRaw': price, 'durationRaw': duration, 'durationUnitRaw': unit,
                       'minQuantityRaw': minimum, 'maxQuantityRaw': maximum, 'reviewFlags': flags})
    return {'schemaVersion': 1, 'approvalStatus': 'reference_only_pending_client_approval',
            'transactionEnabled': False, 'currency': base.get('currency', '$'), 'source': base['source'],
            'sourceSha256': digest, 'sourceBundleSha256': digest, 'capturedAt': manifest['generated_at'],
            'sourceTimezone': base.get('sourceTimezone', 'America/Toronto'), 'categories': base['categories'],
            'services': base['services'], 'extras': extras, 'reviewCount': base.get('reviewCount', 3),
            'counts': {'services': len(base['services']), 'categories': len(base['categories']),
                       'extras': len(extras), 'extrasRequiringReview': sum(bool(item['reviewFlags']) for item in extras)}}


def main():
    cache = ROOT / '.cache/wordpress-export-0.1.3'
    files = [cache / 'manifest.json'] + sorted(cache.glob('catalogue-page-*.json'), key=lambda path: int(path.stem.split('-')[-1]))
    hashes = [{'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()} for path in files]
    digest = hashlib.sha256(json.dumps(hashes, sort_keys=True).encode()).hexdigest()
    manifest = json.loads(files[0].read_text(encoding='utf-8-sig'))
    result = reconcile([json.loads(path.read_text(encoding='utf-8-sig')) for path in files[1:]], manifest, digest)
    target = ROOT / 'dapper-platform/apps/api/data/bookingpress-extras-reference.json'
    target.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    evidence = {'exporterVersion': '0.1.3', 'mode': 'authorized-read-only-catalogue', 'files': hashes,
                'sourceBundleSha256': digest, 'counts': result['counts'], 'transactionEnabled': False,
                'privateRecordsExported': False}
    (ROOT / 'Dapper Detailing Knowledge/99 Sources/wordpress-extras-export-evidence.json').write_text(json.dumps(evidence, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(result['counts']) + '; original values preserved; transactional import disabled')


if __name__ == '__main__':
    main()
