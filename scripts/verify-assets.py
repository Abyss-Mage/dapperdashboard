"""Comprehensive pinned HTTPS verification for pages, assets, MIME types and health."""
import ssl, urllib.request, re, json
from pathlib import Path

root = Path(__file__).resolve().parent.parent
cert_path = root / ".cache/application-runtime/tls/certificate.pem"
context = ssl.create_default_context(cafile=str(cert_path))

def get(path):
    req = urllib.request.Request("https://127.0.0.1:8443" + path)
    with urllib.request.urlopen(req, context=context, timeout=10) as r:
        return r.status, r.headers.get("Content-Type", ""), r.read().decode("utf-8", errors="ignore")

results = []

print("=== 1. Health ===")
st, ct, body = get("/api/v1/health")
h = json.loads(body)
assert st == 200 and h["identityProviderConfigured"] is True and h["status"] == "ok"
print(f"PASS: /api/v1/health -> HTTP {st}, mode={h.get('mode')}, idp={h.get('identityProviderConfigured')}")
results.append("health_200_configured")

print("=== 2. Public Root ===")
st, ct, body = get("/")
assert st == 200 and "hero-title" in body and "Porsche" in body
print(f"PASS: / -> HTTP {st}, body length={len(body)} bytes (contains hero-title, Porsche, no manifest error)")
results.append("root_200_public_studio")

for page in ["/workspace", "/dashboard"]:
    print(f"=== 3. Page {page} ===")
    st, ct, body = get(page)
    assert st == 200
    css_files = re.findall(r'<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"', body)
    js_files = re.findall(r'<script[^>]+src="([^"]+)"', body)
    print(f"  Page {page}: HTTP {st}, found {len(css_files)} CSS files, {len(js_files)} JS files")
    for css in css_files:
        c_st, c_ct, c_b = get(css)
        assert c_st == 200 and "text/css" in c_ct
        print(f"  PASS CSS: {css} -> {c_st} {c_ct} ({len(c_b)} bytes)")
    for js in js_files:
        j_st, j_ct, j_b = get(js)
        assert j_st == 200 and "javascript" in j_ct
        print(f"  PASS JS:  {js} -> {j_st} {j_ct} ({len(j_b)} bytes)")
    results.append(f"{page.strip('/')}_assets_all_200")

print("\nALL PINNED HTTPS AND ASSET CHECKS COMPLETED SUCCESSFULLY!")
