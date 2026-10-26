"""Verify generated public page coverage, links and local assets without a browser."""
from pathlib import Path
from html.parser import HTMLParser
import json
root=Path(__file__).resolve().parents[1]
manifest=json.loads((root/'verification/page-manifest.json').read_text(encoding='utf-8'))
known={p.rstrip('/') or '/' for p in manifest['routes']}
class Page(HTMLParser):
    def __init__(self):super().__init__();self.ids=[];self.links=[];self.refs=[];self.assets=[]
    def handle_starttag(self,tag,attrs):
        a=dict(attrs)
        if 'id' in a:self.ids.append(a['id'])
        if tag=='a':self.links.append(a.get('href',''))
        for key in ['aria-controls','aria-labelledby','aria-describedby']:self.refs.extend(a.get(key,'').split())
        if tag in ['link','script','img']:
            value=a.get('src') or a.get('href','')
            if value.startswith('/site/'):self.assets.append(value[len('/site/'):])
results={}
files=list((root/'pages').glob('*.html'))
assert len(files)==14,'Expected ten service details and four directory/information pages'
for file in files:
    p=Page();text=file.read_text(encoding='utf-8');p.feed(text)
    checks={'uniqueIds':len(p.ids)==len(set(p.ids)),'ariaTargets':all(r in p.ids for r in p.refs),'anchors':all(l[1:] in p.ids for l in p.links if l.startswith('#')),'internalRoutes':all((l.rstrip('/') or '/') in known for l in p.links if l.startswith('/') and not l.startswith('/site/')),'localAssets':all((root/a).is_file() for a in p.assets),'noindex':'noindex,nofollow' in text,'noDemoBranding':'demo' not in text.lower()}
    assert all(checks.values()),(file.name,checks)
    results[file.stem]=checks
photos=json.loads((root/'portfolio-import.json').read_text(encoding='utf-8'))['images']
assert len(photos)==18
for photo in photos:
    assert (root/'assets/portfolio'/photo['file']).is_file()
output={'date':'2026-10-24','pages':results,'pageCount':14,'portfolioImportImages':18,'boundaries':'Source checks only; no production/customer/provider acceptance.'}
(root/'verification/public-page-checks.json').write_text(json.dumps(output,indent=2)+'\n',encoding='utf-8')
print('PASS: 14 public pages, route/anchor/ARIA/local-asset checks, 18 staged portfolio images')
