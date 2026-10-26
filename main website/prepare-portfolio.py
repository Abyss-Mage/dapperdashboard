"""Read-only source migration. Downloads only inventoried public Dapper images.
Does not create dashboard records or publish them. Copyright remains with the source.
"""
from pathlib import Path
from urllib.request import urlopen,Request
from urllib.parse import urlsplit
from PIL import Image,ImageOps
from io import BytesIO
import hashlib,json
root=Path(__file__).resolve().parent
inventory=json.loads((root/'verification/live-public-media.json').read_text(encoding='utf-8'))
target=root/'assets/portfolio';target.mkdir(exist_ok=True)
items=[]
for n,item in enumerate(inventory['images'],1):
    url=item['source'];parsed=urlsplit(url)
    if parsed.scheme!='https' or parsed.netloc!='dapperdetailing.ca' or not parsed.path.startswith('/wp-content/uploads/'):raise ValueError('Source outside Dapper public media')
    data=urlopen(Request(url,headers={'User-Agent':'Mozilla/5.0'}),timeout=30).read(20*1024*1024+1)
    if len(data)>20*1024*1024:raise ValueError('Source image too large')
    with Image.open(BytesIO(data)) as original:
        if original.width*original.height>40000000:raise ValueError('Source image dimensions exceed limit')
        image=ImageOps.exif_transpose(original).convert('RGB');image.thumbnail((1600,1600))
        image.save(target/item['file'],'JPEG',quality=88,optimize=True)
    converted=(target/item['file']).read_bytes()
    items.append({'file':item['file'],'url':'/site/assets/portfolio/'+item['file'],'title':f'Dapper portfolio {n:02d}','alt':item['alt'] or 'Vehicle photographed for the Dapper Detailing Pros portfolio','source':url,'sourceSha256':hashlib.sha256(data).hexdigest(),'localSha256':hashlib.sha256(converted).hexdigest(),'rights':'Existing client-site reference; review publication permissions before importing/publishing.'})
manifest={'sourcePage':inventory['sourcePage'],'retrieved':inventory['retrieved'],'images':items}
(root/'portfolio-import.json').write_text(json.dumps(manifest,indent=2)+'\n',encoding='utf-8')
print('Prepared',len(items),'original-public-gallery images for reviewed dashboard import; no application records created.')
