#!/usr/bin/env python3
"""
test_bookingpress_migration.py

Unit tests for assess_bookingpress_migration.py (DEL-22).
"""

import os
import unittest
from assess_bookingpress_migration import (
    MigrationAssessor,
    parse_duration_minutes,
    parse_price_cents,
)


class TestBookingPressMigration(unittest.TestCase):
    def test_parse_price_cents(self):
        cents, flags = parse_price_cents("49.90")
        self.assertEqual(cents, 4990)
        self.assertEqual(flags, [])

        cents, flags = parse_price_cents(125.00)
        self.assertEqual(cents, 12500)
        self.assertEqual(flags, [])

        cents, flags = parse_price_cents("0")
        self.assertEqual(cents, 0)
        self.assertEqual(flags, [])

        cents, flags = parse_price_cents("")
        self.assertEqual(cents, 0)
        self.assertIn("empty_price", flags)

        cents, flags = parse_price_cents("-15.00")
        self.assertEqual(cents, 0)
        self.assertIn("negative_price_rejected", flags)

        cents, flags = parse_price_cents("not-a-number")
        self.assertEqual(cents, 0)
        self.assertIn("invalid_price_format", flags)

    def test_parse_duration_minutes(self):
        mins, flags = parse_duration_minutes("60", "m")
        self.assertEqual(mins, 60)
        self.assertEqual(flags, [])

        mins, flags = parse_duration_minutes("2", "h")
        self.assertEqual(mins, 120)
        self.assertEqual(flags, [])

        # 1-minute flag
        mins, flags = parse_duration_minutes("1", "m")
        self.assertEqual(mins, 1)
        self.assertIn("one_minute_duration_review", flags)

        # Multi-day duration flag
        mins, flags = parse_duration_minutes("2", "d")
        self.assertEqual(mins, 2880)
        self.assertIn("multi_day_duration_review", flags)

    def test_assessor_with_real_reference(self):
        repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        ref_path = os.path.join(
            repo_root, "dapper-platform", "apps", "api", "data", "bookingpress-reference.json"
        )
        extras_path = os.path.join(
            repo_root, "dapper-platform", "apps", "api", "data", "bookingpress-extras-reference.json"
        )

        assessor = MigrationAssessor(ref_path, extras_path)
        assessor.load_reference()

        # Check categories loaded
        self.assertGreater(len(assessor.categories), 0)
        self.assertEqual(assessor.provenance["timezone"], "America/Toronto")

        # Check services loaded
        self.assertGreater(len(assessor.services), 0)

        # Test booking assessment
        sample_appts = [
            {
                "id": "1",
                "customer_email": "john.doe@example.com",
                "bookingpress_service_id": list(assessor.services.keys())[0],
                "status": "approved",
            },
            {
                "id": "2",
                "customer_email": "bad-email",
                "bookingpress_service_id": list(assessor.services.keys())[0],
                "status": "pending",
            },
            {
                "id": "3",
                "customer_email": "jane@example.com",
                "bookingpress_service_id": "non_existent_999",
                "status": "cancelled",
            },
        ]
        result = assessor.assess_bookings(sample_appts)
        self.assertEqual(result["totalAppointmentsScanned"], 3)
        self.assertEqual(result["validAppointmentsCount"], 1)
        self.assertEqual(result["flaggedAppointmentsCount"], 2)
        self.assertEqual(result["uniqueCustomersIdentified"], 2)

        # Test report generation
        report = assessor.generate_report(sample_appts)
        self.assertTrue(report["reconciliationSafeguards"]["zeroDatabasePollution"])
        self.assertTrue(report["reconciliationSafeguards"]["provenancePreserved"])
        self.assertIn("oneMinuteDurationReviewCount", report["catalogueSummary"])

    def test_eastern_to_utc_conversion(self):
        from migrate_bookingpress_to_postgres import eastern_to_utc_iso
        # Winter (EST, UTC-5): 2024-01-15 10:00:00 -> 15:00:00 UTC
        utc_winter = eastern_to_utc_iso("2024-01-15", "10:00:00")
        self.assertEqual(utc_winter, "2024-01-15T15:00:00+00:00")

        # Summer (EDT, UTC-4): 2024-07-15 10:00:00 -> 14:00:00 UTC
        utc_summer = eastern_to_utc_iso("2024-07-15", "10:00:00")
        self.assertEqual(utc_summer, "2024-07-15T14:00:00+00:00")

    def test_bookingpress_migrator_dry_run(self):
        from migrate_bookingpress_to_postgres import BookingPressMigrator
        raw_export = ".cache/bookingpress-export/bookingpress_export_raw.json"
        if not os.path.exists(raw_export):
            self.skipTest("Raw export not found in cache")

        migrator = BookingPressMigrator(raw_export)
        report = migrator.run_dry_run()

        self.assertEqual(report["deliverable"], "DEL-22")
        self.assertEqual(report["metrics"]["total_customers"], 453)
        self.assertEqual(report["metrics"]["total_vehicles"], 453)
        self.assertEqual(report["metrics"]["total_bookings"], 584)
        self.assertEqual(report["metrics"]["status_breakdown"]["completed"], 364)
        self.assertEqual(report["metrics"]["status_breakdown"]["confirmed"], 143)
        self.assertEqual(report["metrics"]["status_breakdown"]["cancelled"], 77)
        self.assertEqual(report["database_mutations"], 0)
        self.assertTrue(report["provenance"]["verified_known_export"])


if __name__ == "__main__":
    unittest.main()
