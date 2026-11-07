import copy
import unittest
from build_bookingpress_reference import build


class CatalogueBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.manifest = {"site_url": "https://dapperdetailing.ca/", "read_only": True,
                         "generated_at": "2026-10-21T19:28:40Z", "timezone": "America/Toronto"}
        self.payload = {"pricing_approval": "reference_only_pending_client_approval", "resources": {
            "categories": {"available": True, "total_pages": 1, "total": 1, "items": [
                {"bookingpress_category_id": "2", "bookingpress_category_name": "Demo category"}]},
            "services": {"available": True, "total_pages": 1, "total": 1, "items": [{
                "bookingpress_service_id": "19", "bookingpress_category_id": "2",
                "bookingpress_service_name": "Demo service", "bookingpress_service_description": "",
                "bookingpress_service_price": "49.90", "bookingpress_service_duration_val": "1",
                "bookingpress_service_duration_unit": "m"}]}}}

    def test_raw_values_remain_reference_only(self):
        result = build(self.payload, self.manifest, "a" * 64)
        service = result["services"][0]
        self.assertEqual((service["sourceId"], service["categorySourceId"], service["priceRaw"]), ("19", "2", "49.90"))
        self.assertEqual((service["durationRaw"], service["durationUnitRaw"]), ("1", "m"))
        self.assertIn("one_minute_duration_review", service["reviewFlags"])
        self.assertFalse(result["transactionEnabled"])

    def test_private_or_unexpected_columns_rejected(self):
        self.payload["resources"]["services"]["items"][0]["customer_email"] = "must-not-copy"
        with self.assertRaises(ValueError):
            build(self.payload, self.manifest, "a" * 64)

    def test_incomplete_export_and_duplicate_ids_rejected(self):
        self.payload["resources"]["services"]["total_pages"] = 2
        with self.assertRaises(ValueError):
            build(self.payload, self.manifest, "a" * 64)
        self.payload["resources"]["services"]["total_pages"] = 1
        self.payload["resources"]["services"]["items"] *= 2
        self.payload["resources"]["services"]["total"] = 2
        with self.assertRaises(ValueError):
            build(self.payload, self.manifest, "a" * 64)

    def test_unmapped_category_is_visible_without_silent_reassignment(self):
        payload = copy.deepcopy(self.payload)
        payload["resources"]["services"]["items"][0]["bookingpress_category_id"] = "999"
        result = build(payload, self.manifest, "a" * 64)
        self.assertEqual(result["services"][0]["categorySourceId"], "999")
        self.assertIn("unmapped_category", result["services"][0]["reviewFlags"])


if __name__ == "__main__":
    unittest.main()
