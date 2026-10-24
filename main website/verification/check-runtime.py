"""Read-only checks against the isolated built Next.js server on loopback 4175."""
from urllib.request import build_opener,HTTPRedirectHandler
from urllib.error import HTTPError
from pathlib import Path
import json
root=Path(__file__).resolve().parents[1]
class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):return None
opener=build_opener(NoRedirect)
manifest=json.loads((root/'verification/page-manifest.json').read_text(encoding='utf-8'))
results=[]
paths=['/']+[p.rstrip('/') for p in manifest['routes'] if p not in ['/','/my-bookings/','/book-appointment/']]
for path in paths:
    with opener.open('http://127.0.0.1:4175'+path,timeout=15) as response:
        text=response.read().decode('utf-8');status=response.status
        assert status==200,(path,status)
        assert 'text/html' in response.headers.get('Content-Type',''),path
        assert 'demo' not in text.lower(),path
        if path=='/':assert 'hero-title' in text and 'Porsche' in text
        if path not in ['/account','/booking']:
            assert response.headers.get('X-Robots-Tag')=='noindex, nofollow'
            assert '<link rel="canonical"' in text
        results.append({'path':path,'status':status})
for path,target,status in [('/book-appointment','/booking',307),('/my-bookings','/account',307),('/home','/',308)]:
    try:opener.open('http://127.0.0.1:4175'+path,timeout=15);raise AssertionError('Expected redirect')
    except HTTPError as error:
        assert error.code==status and error.headers['Location']==target,(path,error.code,error.headers.get('Location'))
        results.append({'path':path,'status':error.code,'relativeLocation':target})
for path in ['/site/studio-loader.js','/site/assets/three/three.module.js','/site/assets/three/porsche-carrera-gt.glb','/site/pages.css']:
    with opener.open('http://127.0.0.1:4175'+path,timeout=15) as response:
        assert response.status==200,path
        if path.endswith('.js'):assert 'javascript' in response.headers['Content-Type'],path
        results.append({'path':path,'status':response.status,'contentType':response.headers['Content-Type']})
for path in ['/site/portfolio-import.json','/site/assets/portfolio/portfolio-01.jpg']:
    try:opener.open('http://127.0.0.1:4175'+path,timeout=15);raise AssertionError('Private source archive exposed')
    except HTTPError as error:
        assert error.code==404,(path,error.code)
        results.append({'path':path,'status':404,'sourceArchive':'not public'})
for path in ['/robots.txt','/sitemap.xml']:
    with opener.open('http://127.0.0.1:4175'+path,timeout=15) as response:
        content=response.read().decode('utf-8')
        if path=='/robots.txt':assert 'Disallow: /' in content
        else:assert '<url>' not in content
        results.append({'path':path,'status':response.status,'prelaunchIndexing':'blocked'})
report={'date':'2026-10-24','server':'Isolated Next.js production build on loopback4175','checks':results,'boundary':'No shared application deployment, API proxy, account login, provider operation or live cutover performed.'}
(root/'verification/public-runtime-checks.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
print('PASS:',len(results),'built-route/asset/relative-redirect checks')
