"""Verify synthetic scoped state and an in-memory session across owned restart."""
import sys,json,ssl,urllib.request,http.cookiejar,hashlib,subprocess,shutil
from pathlib import Path
from datetime import datetime,timezone
root=Path(__file__).resolve().parents[1];runtime=root/'.cache/workspace-runtime'
credential=json.load(sys.stdin)
client=urllib.request.build_opener(urllib.request.HTTPSHandler(context=ssl.create_default_context(cafile=str(runtime/'tls/certificate.pem'))),urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
origin='https://127.0.0.1:8443'
def request(path,data=None):
 req=urllib.request.Request(origin+path,data=None if data is None else json.dumps(data).encode(),headers={'Origin':origin,'Content-Type':'application/json'},method='GET' if data is None else 'POST')
 with client.open(req,timeout=10) as response:
  body=response.read();return json.loads(body) if body else None
try:
 request('/api/v1/auth/login',credential);credential.clear()
 actor=request('/api/v1/auth/me')['principalId'];access=request('/api/v1/auth/access');scope=access['locations'][0]
 base='/api/v1/organizations/'+scope['organizationId']+'/locations/'+scope['locationId']
 def fingerprint():
  data={'services':request(base+'/services'),'jobs':request(base+'/jobs'),'structure':request('/api/v1/organizations/'+scope['organizationId']+'/structure')}
  return hashlib.sha256(json.dumps(data,sort_keys=True).encode()).hexdigest()
 before=fingerprint()
 for script in ('stop-workspace.ps1','start-workspace.ps1'):
  result=subprocess.run([shutil.which('pwsh.exe') or 'powershell.exe','-NoProfile','-File',str(root/'scripts'/script)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
  if result.returncode:raise RuntimeError('Owned restart failed')
 assert request('/api/v1/auth/me')['principalId']==actor
 after=fingerprint();assert before==after
 request('/api/v1/auth/logout',{})
 report={'verifiedAt':datetime.now(timezone.utc).isoformat(),'mode':'synthetic-development-only','result':'pass','sameSessionSurvivedRestart':True,'scopedStateDigestUnchanged':True,'stateSha256':after,'certificateAndHostnameValidation':True,'browserVerified':False}
 (root/'output/acceptance/scoped-restart-verification.json').write_text(json.dumps(report,indent=2),encoding='utf-8')
 print('PASS: same verified session and scoped service/job/structure records survived owned API/gateway restart')
except Exception as error:
 print('FAIL: scoped restart acceptance ('+type(error).__name__+'); private values suppressed',file=sys.stderr);sys.exit(1)
