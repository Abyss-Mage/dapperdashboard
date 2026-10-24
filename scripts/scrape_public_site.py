"""Bounded read-only discovery of the public Dapper service catalogue.

No forms, private APIs, credentials, customer reviews or booking records are read.
Output is reference data; it never approves transactional prices or policies.
"""
import hashlib
import json
import re
import time
from datetime import datetime, timezone
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urljoin, urlparse
from urllib.request import Request, urlopen, HTTPRedirectHandler, build_opener

BASE = "https://dapperdetailing.ca/"
HOSTS = {"dapperdetailing.ca", "www.dapperdetailing.ca"}
ROOT = Path(__file__).resolve().parents[1]


class SameSiteRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        if urlparse(newurl).hostname not in HOSTS or urlparse(newurl).scheme != "https":
            raise ValueError("Refusing off-site redirect")
        return super().redirect_request(req, fp, code, msg, headers, newurl)


class Page(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.links = set()
        self.headings = []
        self.tables = []
        self.prices = []
        self.capture = None
        self.text = []
        self.skip = 0

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag in {"script", "style", "noscript"}:
            self.skip += 1
        if self.skip:
            return
        if tag == "a" and attrs.get("href"):
            self.links.add(urljoin(BASE, attrs["href"]).split("#")[0])
        if tag in {"h1", "h2", "h3", "td", "th"}:
            self.capture, self.text = tag, []

    def handle_data(self, data):
        if not self.skip and self.capture:
            self.text.append(data)

    def handle_endtag(self, tag):
        if tag in {"script", "style", "noscript"}:
            self.skip = max(0, self.skip - 1)
        if tag == self.capture:
            value = re.sub(r"\s+", " ", " ".join(self.text)).strip()
            if value:
                if tag in {"td", "th"}:
                    self.tables.append(value)
                else:
                    self.headings.append(value)
                self.prices.extend(re.findall(r"\$\s*[\d,]+(?:\.\d{1,2})?", value))
            self.capture, self.text = None, []


def fetch(url):
    req = Request(url, headers={"User-Agent": "DapperReferenceDiscovery/0.1 (read-only client-authorized discovery)"})
    with build_opener(SameSiteRedirect()).open(req, timeout=30) as response:
        raw = response.read(5_000_001)
        if len(raw) > 5_000_000:
            raise ValueError("Page exceeds size bound")
        if "text/html" not in response.headers.get("Content-Type", ""):
            raise ValueError("Not an HTML page")
        page = Page()
        page.feed(raw.decode("utf-8", errors="replace"))
        return page, hashlib.sha256(raw).hexdigest(), response.url


def main():
    home, _, _ = fetch(BASE)
    patterns = ("detailing", "coating", "ppf", "paint-protection", "boat", "aircraft", "wheeler")
    urls = sorted({url.split('?')[0].rstrip('/') + '/' for url in home.links if urlparse(url).hostname in HOSTS
                  and urlparse(url).scheme == "https" and url.rstrip("/") != BASE.rstrip("/")
                  and any(word in urlparse(url).path.lower() for word in patterns)
                  and not any(word in urlparse(url).path.lower() for word in ("wp-content", "privacy", "terms"))})[:20]
    pages, failures = [], []
    for url in urls:
        try:
            page, digest, resolved = fetch(url)
            pages.append({"url": resolved, "sha256": digest, "headings": page.headings,
                          "table_cells": page.tables, "published_price_labels": sorted(set(page.prices))})
            print("Read", resolved)
        except Exception as exc:
            failures.append({"url": url, "error_type": type(exc).__name__})
            print("Unavailable", url, type(exc).__name__)
        time.sleep(0.25)
    data = {"source": BASE, "retrieved_at": datetime.now(timezone.utc).isoformat(),
            "approval_status": "reference_only_pending_client_approval", "currency": "not_explicitly_confirmed",
            "pages": pages, "failures": failures}
    target = ROOT / "Dapper Detailing Knowledge" / "99 Sources" / "website-reference.json"
    target.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print("Reference pages:", len(pages), "failures:", len(failures))


if __name__ == "__main__":
    main()
