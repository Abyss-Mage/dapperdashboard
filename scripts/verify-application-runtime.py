"""Read-only application ingress checks; never signs in or creates records."""
import json, ssl, urllib.request, urllib.error
from pathlib import Path
from datetime import datetime, timezone

root=Path(__file__).resolve().parents[1]
context=ssl.create_default_context(cafile=str(root/'.cache/application-runtime/tls/certificate.pem'))
cases=[]
def request(path,expected=200):
    req=urllib.request.Request('https://127.0.0.1:8443'+path)
    try:
        with urllib.request.urlopen(req,context=context,timeout=10) as response:
            status=response.status;body=response.read();url=response.url
    except urllib.error.HTTPError as response:
        status=response.code;body=response.read();url=response.url
    if status!=expected: raise RuntimeError('Unexpected ingress status: '+str(status))
    return body.decode(),url

body,_=request('/api/v1/health')
health=json.loads(body)
assert health['identityProviderConfigured'] is True
assert health['productionReady'] is False # Deployment acceptance is unfinished.
cases.append('tls_verified_go_api_identity_configured')
body,url=request('/')
assert 'hero-title' in body and 'Porsche' in body and not url.endswith('/workspace')
assert 'Synthetic development workspace' not in body
cases.append('root_uses_production_public_studio')
body,_=request('/workspace')
assert 'employee-content' in body and 'Employee workspace' in body
cases.append('employee_workspace_shell_authentication_enforced_by_api')
body,_=request('/account')
assert 'Your customer account' in body and 'Synthetic records only' not in body
cases.append('customer_account_product_route')
body,_=request('/owner')
assert 'Owner & Business Dashboard' in body or 'Sign in' in body
cases.append('owner_dashboard_product_route')
for path in ['/api/v1/auth/me','/api/v1/portal/accounts','/api/v1/demo/session','/api/v1/workspace','/api/v1/employee/workspace','/api/v1/employee/calendar?from=2026-10-01&to=2026-11-01','/api/v1/employee/walk-in/options','/api/v1/organizations/00000000-0000-0000-0000-000000000001/locations/00000000-0000-0000-0000-000000000002/content/source-images']:
    request(path,401)
cases.append('no_anonymous_staff_customer_or_retired_demo_session')
manifest=json.loads((root/'dapper-platform/apps/web/.next/routes-manifest.json').read_text())
assert not any(manifest['rewrites'].values()) if isinstance(manifest['rewrites'],dict) else not manifest['rewrites']
cases.append('no_next_api_fallback_rewrites')
report={'timestamp':datetime.now(timezone.utc).isoformat(),'scope':'unseeded-local-application-ingress',
        'checks':cases,'productionAccepted':False,'authenticatedWorkflowAcceptance':'NOT_RUN: no default accounts provisioned'}
(root/'output/acceptance/application-ingress-verification.json').write_text(json.dumps(report,indent=2))
print('PASS:',len(cases),'application ingress checks; no account, fixture or operational data created')
