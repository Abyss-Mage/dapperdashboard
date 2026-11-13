"""Unit checks for the legacy import MCP server (no network, no database).  python tools/legacy-import-mcp/test_server.py"""
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import server


class Pure(unittest.TestCase):
    def test_items_of(self):
        self.assertEqual(server.items_of([1, 2]), [1, 2])
        self.assertEqual(server.items_of({"items": [1]}), [1])
        self.assertEqual(server.items_of({"data": [1, 2, 3]}), [1, 2, 3])
        self.assertEqual(server.items_of({"total": 3}), [])

    def test_pagination_stops(self):
        pages = {1: [{"id": i} for i in range(3)], 2: [{"id": 9}]}
        calls = []

        def fake(url, auth):
            calls.append(url)
            n = int(url.split("page=")[1].split("&")[0])
            return pages.get(n, []), {}
        out = server.fetch_section("content", "x", get=fake, per_page=3)
        self.assertEqual(len(out), 4)
        self.assertEqual(len(calls), 2)

    def test_media_guard(self):
        ok = "https://dapperdetailing.ca/wp-content/uploads/a.jpg"
        self.assertTrue(server.allowed_media(ok))
        for bad in ("http://dapperdetailing.ca/a.jpg", "https://evil.example/a.jpg", "https://dapperdetailing.ca.evil.example/a.jpg", "file:///etc/passwd", ""):
            self.assertFalse(server.allowed_media(bad), bad)
        self.assertEqual(server.media_url({"source_url": ok}), ok)
        self.assertEqual(server.media_url({"guid": {"rendered": ok}}), ok)

    def test_export_check(self):
        with tempfile.TemporaryDirectory() as d:
            bad = Path(d) / "bad.json"
            bad.write_text(json.dumps({"customers": []}), encoding="utf-8")
            with mock.patch.object(server, "CACHE", Path(d) / "cache"):
                with self.assertRaises(RuntimeError):
                    server.load_export(str(bad))
                good = Path(d) / "good.json"
                good.write_text(json.dumps({k: [{}] for k in server.EXPORT_KEYS}), encoding="utf-8")
                r = server.load_export(str(good))
                self.assertEqual(r["counts"]["customers"], 1)

    def test_fingerprint_hides_password(self):
        a = server.url_fingerprint("postgres://u:secret1@127.0.0.1:5432/dapper")
        b = server.url_fingerprint("postgres://u:secret2@127.0.0.1:5432/dapper")
        self.assertEqual(a, b)
        self.assertNotEqual(a, server.url_fingerprint("postgres://u:secret1@127.0.0.1:5432/other"))

    def test_apply_needs_matching_confirm(self):
        with tempfile.TemporaryDirectory() as d, mock.patch.object(server, "CACHE", Path(d)):
            (Path(d) / "plans").mkdir()
            server.plan_path("billing-20261113-120000").write_text(json.dumps({"sha256": "abcdef123456789", "applied": False}), encoding="utf-8")
            with self.assertRaises(RuntimeError):
                server.apply_plan("billing-20261113-120000", "wrong")
            with self.assertRaises(RuntimeError):
                server.plan_path("../../etc")

    def test_undo_only_own_batches(self):
        with self.assertRaises(RuntimeError):
            server.undo_batch("bookingpress-history-2026-11-04")


if __name__ == "__main__":
    unittest.main()
