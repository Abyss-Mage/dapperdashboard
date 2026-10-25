'use client';
import {useEffect,useRef,useState} from 'react';
import Select from '../GlassSelect';

type Scope={organizationId:string;locationId:string;organizationName:string;locationName:string;actions:string[]};
type Site={slug:string;name:string;published:boolean;version:number};
type Service={serviceId:string;name:string;kind:string;parentId:string|null;serviceVersion:number;category:string;summary:string;published:boolean;version:number};
type Photo={id:string;mediaId:string;title:string;alt:string;category:string;sortOrder:number;published:boolean;version:number};
type Content={site:Site;services:Service[];portfolio:Photo[];limit:number};
const categories=[['2-wheeler','2 Wheeler'],['hatchback','Hatchback'],['sedan','Sedan'],['suv','SUV'],['pickup','Pickup'],['van','Van'],['ceramic','Ceramic coating'],['ppf','PPF'],['aircraft','Aircraft'],['boat','Boat']];
type LegacyImage={title:string;alt:string;file:string};
const initialPhoto:Photo={id:'',mediaId:'',title:'',alt:'',category:'detailing',sortOrder:0,published:false,version:0};
async function result(response:Response){
 if(!response.headers.get('Content-Type')?.includes('application/json'))throw Error('The application service could not be reached. Please try again.');
 let body;try{body=await response.json()}catch{throw Error('The application service returned an incomplete response. Please try again.')}
 if(!response.ok)throw Error(body.error??'The operation could not be completed.');return body;
}

export default function ContentManager(){
 const [scopes,setScopes]=useState<Scope[]>([]),[selected,setSelected]=useState(''),[error,setError]=useState(''),[loading,setLoading]=useState(true);
 useEffect(()=>{const abort=new AbortController();fetch('/api/v1/auth/access',{cache:'no-store',signal:abort.signal}).then(result).then(data=>{const allowed:Scope[]=(data.locations??[]).filter((s:Scope)=>s.actions.includes('catalogue.manage')||s.actions.includes('content.manage'));setScopes(allowed);setSelected(allowed[0]?.locationId??'')}).catch(e=>{if(!abort.signal.aborted)setError(e.message)}).finally(()=>{if(!abort.signal.aborted)setLoading(false)});return()=>abort.abort()},[]);
 const scope=scopes.find(s=>s.locationId===selected);
 return <main className="secure-workspace" style={{maxWidth:1150,margin:'0 auto',padding:'clamp(1rem,4vw,3rem)'}}>
  <header style={{display:'flex',gap:20,flexWrap:'wrap',alignItems:'center',justifyContent:'space-between',marginBottom:30}}><a href="/">Dapper home</a><nav style={{display:'flex',gap:20}} aria-label="Dashboard navigation"><a href="/workspace">Workspace</a><a href="/owner">Master dashboard</a></nav></header>
  <h1>Services &amp; portfolio publishing</h1><p>Manage public presentation without duplicating operational service records.</p>
  {loading?<p role="status">Checking your access…</p>:error?<p role="alert">{error} <a href="/workspace">Sign in to the dashboard</a>.</p>:!scope?<p>No publishing permissions are available for this account. Ask an authorized access manager for the required location capability.</p>:<>
   <label className="form-field" style={{maxWidth:500,marginBlock:24}}>Location<Select value={selected} onChange={e=>setSelected(e.target.value)}>{scopes.map(s=><option key={s.locationId} value={s.locationId}>{s.organizationName} / {s.locationName}</option>)}</Select></label>
   <LocationContent key={scope.organizationId+'/'+scope.locationId} scope={scope}/>
  </>}
 </main>
}

function LocationContent({scope}:{scope:Scope}){
 const base=`/api/v1/organizations/${scope.organizationId}/locations/${scope.locationId}/content`;
 const canContent=scope.actions.includes('content.manage'),canServices=scope.actions.includes('catalogue.manage');
 const [data,setData]=useState<Content|null>(null),[error,setError]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false),[revision,setRevision]=useState(0);
 const [site,setSite]=useState<Site>({slug:'dapper',name:'Dapper Detailing Pros',published:false,version:0});
 const [service,setService]=useState<Service|null>(null),[photo,setPhoto]=useState<Photo>({...initialPhoto}),[file,setFile]=useState<File|null>(null),[rights,setRights]=useState(false);
 const [legacyImages,setLegacyImages]=useState<LegacyImage[]>([]),[filePreview,setFilePreview]=useState('');
 useEffect(()=>{const abort=new AbortController();fetch(base+'/source-images',{signal:abort.signal}).then(response=>response.ok?response.json():null).then(body=>{if(body&&!abort.signal.aborted)setLegacyImages(body.images??[])}).catch(()=>{});return()=>abort.abort()},[]);
 useEffect(()=>{if(!file){setFilePreview('');return}const url=URL.createObjectURL(file);setFilePreview(url);return()=>URL.revokeObjectURL(url)},[file]);
 const mounted=useRef(true),alertRef=useRef<HTMLParagraphElement>(null),fileRef=useRef<HTMLInputElement>(null);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 useEffect(()=>{const abort=new AbortController();setData(null);fetch(base,{signal:abort.signal,cache:'no-store'}).then(result).then(body=>{if(!abort.signal.aborted){setData(body);setSite(body.site.version?body.site:{slug:'dapper',name:'Dapper Detailing Pros',published:false,version:0})}}).catch(e=>{if(!abort.signal.aborted)setError(e.message)});return()=>abort.abort()},[base,revision]);
 useEffect(()=>{if(error&&!busy)alertRef.current?.focus()},[error,busy]);
 async function save(action:()=>Promise<void>){setError('');setNotice('');setBusy(true);try{await action();if(mounted.current){setNotice('Saved to the shared application.');setRevision(v=>v+1)}}catch(e){if(mounted.current)setError(e instanceof Error?e.message:'Save failed. Your input has been kept.')}finally{if(mounted.current)setBusy(false)}}
 const put=(path:string,payload:unknown)=>fetch(base+path,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)}).then(result);
 const noSite=!data?.site.version;
 return <>
  {error&&<p ref={alertRef} tabIndex={-1} role="alert" className="field-error">{error}</p>}{notice&&<p role="status">{notice}</p>}
  {!data&&!error&&<p role="status">Loading this location’s content…</p>}
  {canContent&&<section className="panel detail-card" style={{marginBlock:24}}><h2>Public location</h2><p>Publishing makes the selected public content visible anonymously. It does not publish customer or staff records.</p><form className="management-form" onSubmit={e=>{e.preventDefault();save(async()=>{await put('/site',site)})}}>
   <label className="form-field">Site key<input required pattern="[a-z0-9][a-z0-9-]{1,59}" value={site.slug} readOnly={site.version>0} onChange={e=>setSite({...site,slug:e.target.value})}/><small>The current public pages use the key “dapper”. Existing keys stay stable.</small></label>
   <label className="form-field">Public display name<input required minLength={2} maxLength={120} value={site.name} onChange={e=>setSite({...site,name:e.target.value})}/></label>
   <label><input type="checkbox" checked={site.published} onChange={e=>setSite({...site,published:e.target.checked})}/> Publish this location’s content</label>
   <button className="primary" disabled={busy||!data}>Save public location</button>
  </form></section>}
  {canServices&&<section className="panel detail-card" style={{marginBlock:24}}><h2>Shared service presentation</h2><p>Names, main/add-on relationships and availability come from the same catalogue used by Workspace and Master. Edit services in their existing catalogue editor. Prices and booking rules remain in the shared offer/scheduling system.</p>
   {noSite&&<p>Ask a content manager to configure the public location first.</p>}
   <div className="management-list">{data?.services.map(s=><article key={s.serviceId}><div><strong>{s.name}</strong><small>{s.kind} · service revision {s.serviceVersion} · {s.published?'Published':'Not published'}</small></div><button className="secondary" disabled={busy||noSite} onClick={()=>setService({...s,category:s.category||'sedan'})}>Edit presentation</button></article>)}</div>
   {data?.services.length===0&&<p>No active operational services at this location. Add services through the existing catalogue editor.</p>}
   {service&&<form className="management-form" onSubmit={e=>{e.preventDefault();save(async()=>{await put('/services/'+service.serviceId,{category:service.category,summary:service.summary,published:service.published,version:service.version,serviceVersion:service.serviceVersion});if(mounted.current)setService(null)})}}>
    <h3>{service.name}</h3><label className="form-field">Public category<Select value={service.category} onChange={e=>setService({...service,category:e.target.value})}>{categories.map(([id,label])=><option key={id} value={id}>{label}</option>)}</Select></label>
    <label className="form-field">Description<textarea required maxLength={2000} value={service.summary} onChange={e=>setService({...service,summary:e.target.value})}/></label>
    <label><input type="checkbox" checked={service.published} onChange={e=>setService({...service,published:e.target.checked})}/> Publish this service description</label><button className="primary" disabled={busy}>Save presentation</button><button type="button" className="secondary" disabled={busy} onClick={()=>setService(null)}>Cancel</button>
   </form>}
  </section>}
  {canContent&&<section className="panel detail-card" style={{marginBlock:24}}><h2>Portfolio images</h2><p>Upload JPEG or PNG files up to 6 MB. Images stay private until their portfolio entry and the location are published. Unpublish an entry to remove it from public access while retaining its audit history.</p>
   <form className="management-form" onSubmit={e=>{e.preventDefault();save(async()=>{let mediaId=photo.mediaId;if(file){mediaId=(await result(await fetch(base+'/media',{method:'POST',body:file,headers:{'Content-Type':file.type}}))).id;if(mounted.current){setPhoto(current=>({...current,mediaId}));setFile(null);if(fileRef.current)fileRef.current.value=''}}await put('/portfolio',{...photo,mediaId,rightsReviewed:rights});if(mounted.current){setPhoto({...initialPhoto});setRights(false)}})}}>
    <h3>{photo.id?'Edit image':'Add an image'}</h3>
    {(filePreview||photo.mediaId)&&<img src={filePreview||base+'/media/'+photo.mediaId} alt={photo.alt||'Uploaded portfolio preview'} style={{maxWidth:240,maxHeight:180,objectFit:'contain'}}/>}
    {legacyImages.length>0&&<label className="form-field">Or choose an existing website photo<Select disabled={busy||noSite} value="" onChange={async e=>{const item=legacyImages.find(i=>i.file===e.target.value);if(!item)return;setError('');setBusy(true);try{const response=await fetch(base+'/source-images/'+encodeURIComponent(item.file));if(!response.ok)throw Error('The source photo could not be loaded.');const blob=await response.blob();if(mounted.current){setFile(new File([blob],item.file,{type:'image/jpeg'}));if(!photo.id)setPhoto(current=>({...current,title:item.title,alt:item.alt,category:'other',published:false}));setRights(false);if(fileRef.current)fileRef.current.value=''}}catch(e){if(mounted.current)setError(e instanceof Error?e.message:'Source photo unavailable')}finally{if(mounted.current)setBusy(false)}}}><option value="">Choose a photo to review</option>{legacyImages.map(image=><option key={image.file} value={image.file}>{image.title}</option>)}</Select><small>Imported from the current public portfolio. Review its title, description, category and publication permission before saving.</small></label>}
    <label className="form-field">{photo.mediaId?'Replace image (optional)':'Image'}<input ref={fileRef} type="file" accept="image/jpeg,image/png" required={!photo.mediaId&&!file} disabled={busy||noSite} onChange={e=>{const next=e.target.files?.[0]??null;if(next&&next.size>6*1024*1024){setError('Choose an image no larger than 6 MB.');e.target.value='';setFile(null);return}setFile(next)}}/></label>
    <label className="form-field">Title<input required minLength={2} maxLength={120} value={photo.title} onChange={e=>setPhoto({...photo,title:e.target.value})}/></label>
    <label className="form-field">Image description (alt text)<textarea required minLength={2} maxLength={300} value={photo.alt} onChange={e=>setPhoto({...photo,alt:e.target.value})}/></label>
    <label className="form-field">Category<Select value={photo.category} onChange={e=>setPhoto({...photo,category:e.target.value})}>{['detailing','ceramic','ppf','other'].map(c=><option key={c}>{c}</option>)}</Select></label>
    <label className="form-field">Display order<input type="number" required min={0} max={10000} value={photo.sortOrder} onChange={e=>setPhoto({...photo,sortOrder:Number(e.target.value)})}/></label>
    <label><input type="checkbox" checked={photo.published} onChange={e=>setPhoto({...photo,published:e.target.checked})}/> Publish image</label>
    <label><input type="checkbox" required={photo.published} checked={rights} onChange={e=>setRights(e.target.checked)}/> I have reviewed permission to publish this image and any identifiable people or vehicle details.</label>
    <button className="primary" disabled={busy||noSite}>{busy?'Saving…':'Save image'}</button><button type="button" className="secondary" disabled={busy} onClick={()=>{setPhoto({...initialPhoto});setRights(false);setFile(null);if(fileRef.current)fileRef.current.value=''}}>Cancel / clear</button>
   </form><div className="management-list">{data?.portfolio.map(p=><article key={p.id}><img src={base+'/media/'+p.mediaId} alt={p.alt} width={100} height={75} style={{objectFit:'cover'}}/><div><strong>{p.title}</strong><small>{p.published?'Published':'Draft'} · revision {p.version} · order {p.sortOrder}</small></div><button className="secondary" disabled={busy} onClick={()=>{setPhoto({...p});setRights(false);setFile(null);if(fileRef.current)fileRef.current.value=''}}>Edit</button></article>)}</div>
   {data?.portfolio.length===0&&<p>No uploaded portfolio images yet.</p>}
  </section>}
  {data&&<p>Showing up to {data.limit} entries per section. Publishing does not change booking confirmation or payment status.</p>}
 </>
}
