import copy
import unittest
from build_bookingpress_extras import reconcile, EXTRA_FIELDS
from test_bookingpress_reference import CatalogueBoundaryTests


class ExtrasBoundaryTests(unittest.TestCase):
    def setUp(self):
        fixture = CatalogueBoundaryTests(); fixture.setUp()
        self.manifest = dict(fixture.manifest, exporter_version='0.1.3')
        self.page = fixture.payload
        self.page.update(page=1, per_page=1)
        extra = {key: '1' for key in EXTRA_FIELDS}
        extra.update(bookingpress_service_id='19', bookingpress_extra_service_name='Synthetic add-on',
                     bookingpress_service_description='', bookingpress_extra_service_price='29.90',
                     bookingpress_extra_service_duration='0', bookingpress_extra_service_duration_unit='m')
        self.page['resources']['extras'] = {'available': True, 'total': 1, 'total_pages': 1, 'items': [extra]}

    def test_values_and_zero_duration_preserved(self):
        result = reconcile([self.page], self.manifest, 'a'*64)
        item = result['extras'][0]
        self.assertEqual((item['parentServiceSourceId'], item['priceRaw'], item['durationRaw']), ('19', '29.90', '0'))
        self.assertIn('missing_or_zero_duration_review', item['reviewFlags'])
        self.assertFalse(result['transactionEnabled'])

    def test_private_columns_rejected(self):
        self.page['resources']['extras']['items'][0]['customer_email'] = 'must-not-copy'
        with self.assertRaises(ValueError): reconcile([self.page], self.manifest, 'x')

    def test_missing_pages_and_changed_totals_rejected(self):
        self.page['resources']['extras'].update(total=2, total_pages=2)
        with self.assertRaises(ValueError): reconcile([self.page], self.manifest, 'x')
        second = copy.deepcopy(self.page); second['page'] = 2
        second['resources']['extras']['total'] = 3
        with self.assertRaises(ValueError): reconcile([self.page, second], self.manifest, 'x')

    def test_duplicate_original_ids_rejected(self):
        second = copy.deepcopy(self.page); second['page'] = 2
        for page in (self.page, second): page['resources']['extras'].update(total=2, total_pages=2)
        second['resources']['services']['items'] = []
        second['resources']['categories']['items'] = []
        with self.assertRaises(ValueError): reconcile([self.page, second], self.manifest, 'x')


if __name__ == '__main__': unittest.main()
