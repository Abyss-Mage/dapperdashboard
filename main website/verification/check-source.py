"""Static document checks. Does not establish browser or production acceptance."""
from html.parser import HTMLParser
from pathlib import Path
import json
root = Path(__file__).resolve().parents[1]
class Document(HTMLParser):
    def __init__(self):
        super().__init__(); self.ids=[]; self.links=[]; self.assets=[]; self.refs=[]
    def handle_starttag(self, tag, attrs):
        a=dict(attrs)
        if 'id' in a: self.ids.append(a['id'])
        if tag=='a': self.links.append(a.get('href',''))
        if tag in ('script','img','link'):
            value=a.get('src',a.get('href',''))
            if value and not value.startswith(('http:','https:','#','/')): self.assets.append(value)
        for key in ('aria-controls','aria-labelledby','aria-describedby'):
            self.refs.extend(a.get(key,'').split())
doc=Document(); text=(root/'index.html').read_text(encoding='utf-8'); doc.feed(text)
checks={
 'unique_ids':len(doc.ids)==len(set(doc.ids)),
 'fragment_targets':all(h[1:] in doc.ids for h in doc.links if h.startswith('#')),
 'accessible_references':all(r in doc.ids for r in doc.refs),
 'local_assets_present':all((root/a).is_file() for a in doc.assets),
 'no_legacy_booking_handoff':'https://dapperdetailing.ca/book-appointment' not in text,
 'account_and_booking_routes':'/account' in doc.links and '/booking' in doc.links,
 'contact_links':'tel:+17059776217' in doc.links and 'mailto:info@dapperdetailing.ca' in doc.links,
 'prelaunch_noindex':'noindex,nofollow' in text,
 'model_attribution':'Ddiaz Design' in text and 'creativecommons.org/licenses/by/4.0/' in text,
 'no_active_demo_copy':'demo' not in text.lower(),
}
assert all(checks.values()), checks
report={'date':'2026-10-24','evidence':'source checks and Node VM interaction checks only','checks':checks,'browser':'Baseline inspected in Brave. Revised desktop/mobile acceptance pending: browser debugger detached/timeouts.','production':'Not accepted or deployed. Same-origin account/booking gateway integration pending.'}
(root/'verification/public-site-checks.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
print(f'PASS: {len(checks)} document/source checks')
