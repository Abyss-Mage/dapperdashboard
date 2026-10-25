'use client';
import {useCallback,useEffect,useState} from 'react';
import Select from '../GlassSelect';
type Issuer={id:string;legalName:string;address:string;taxRegistration:string;currency:string;numberPrefix:string;version:number;nextNumber:number};
type Configuration={issuers:Issuer[];locations:{id:string;name:string}[];bindings:{locationId:string;issuerId:string}[]};
const emptyForm={legalName:'',address:'',taxRegistration:'',currency:'',numberPrefix:'',startNumber:''};
export default function InvoiceIssuerEditor({organizationId,revision}:{organizationId:string;revision:number}){
 const [data,setData]=useState<Configuration>({issuers:[],locations:[],bindings:[]});
 const [id,setId]=useState('');const [version,setVersion]=useState(0);const [form,setForm]=useState(emptyForm);
 const [location,setLocation]=useState('');const [boundIssuer,setBoundIssuer]=useState('');
 const [busy,setBusy]=useState(false);const [error,setError]=useState('');const [notice,setNotice]=useState('');
 const base=`/api/v1/organizations/${organizationId}/invoice-issuers`;
 const load=useCallback(async()=>{const response=await fetch(base,{cache:'no-store'});if(!response.ok)throw new Error('Invoice issuer settings are unavailable or access has changed.');setData(await response.json());},[base]);
 useEffect(()=>{load().catch(err=>setError(err.message));},[load,revision]);
 useEffect(()=>{setBoundIssuer(data.bindings.find(b=>b.locationId===location)?.issuerId??'');},[data,location]);
 function select(next:string){const issuer=data.issuers.find(i=>i.id===next);setId(next);setVersion(issuer?.version??0);setForm(issuer?{legalName:issuer.legalName,address:issuer.address,taxRegistration:issuer.taxRegistration,currency:issuer.currency,numberPrefix:issuer.numberPrefix,startNumber:''}:emptyForm);setNotice('');}
 async function save(event:React.FormEvent){
  event.preventDefault();const startNumber=Number(form.startNumber);
  if(!id&&(!Number.isSafeInteger(startNumber)||startNumber<1)){setError('Enter an explicit positive starting number within the supported integer range.');return;}
  setBusy(true);setError('');setNotice('');
  try{const {startNumber:unused,...fields}=form;void unused;const response=await fetch(base+(id?'/'+id:''),{method:id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...fields,version,...(!id?{startNumber}:{})})});const body=await response.json();if(!response.ok)throw new Error(body.error??'Could not save issuer. Input is preserved.');setId(body.id);setVersion(body.version);await load();setNotice('Issuer saved. Issued invoice details remain unchanged.');}catch(err){setError(err instanceof Error?err.message:'Issuer save failed.');}finally{setBusy(false);}
 }
 async function bind(event:React.FormEvent){event.preventDefault();setBusy(true);setError('');setNotice('');try{const response=await fetch(base+'/locations/'+location,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({issuerId:boundIssuer})});if(!response.ok){const body=await response.json();throw new Error(body.error??'Could not link issuer.');}await load();setNotice('Location linked. Locations using this issuer share its number sequence.');}catch(err){setError(err instanceof Error?err.message:'Issuer link failed.');}finally{setBusy(false);}}
 const issuer=data.issuers.find(i=>i.id===id);
 return <section className="panel detail-card"><h2>Invoice issuers</h2><p>Use the actual legal issuer and numbering approved for your business. Each location must be linked explicitly.</p>
  {error&&<p className="field-error" role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
  <label className="form-field">Edit issuer<Select disabled={busy} value={id} onChange={e=>select(e.target.value)}><option value="">Create issuer</option>{data.issuers.map(i=><option key={i.id} value={i.id}>{i.legalName} · revision {i.version}</option>)}</Select></label>
  <form onSubmit={save}><div className="management-form">
   {([{key:'legalName',label:'Legal name',min:2,max:200},{key:'taxRegistration',label:'Tax registration (if applicable)',min:0,max:100},{key:'numberPrefix',label:'Number prefix (uppercase letters, digits, hyphens)',min:1,max:24}] as const).map(f=><label className="form-field" key={f.key}>{f.label}<input required={f.min>0} minLength={f.min} maxLength={f.max} pattern={f.key==='numberPrefix'?'[A-Z0-9-]{1,24}':undefined} value={form[f.key]} disabled={busy} onChange={e=>setForm({...form,[f.key]:e.target.value})}/></label>)}
   <label className="form-field">Currency<Select required disabled={busy} value={form.currency} onChange={e=>setForm({...form,currency:e.target.value})}><option value="">Choose currency</option><option value="CAD">CAD</option></Select></label>
  </div><label className="form-field">Legal address<textarea required minLength={5} maxLength={1000} value={form.address} disabled={busy} onChange={e=>setForm({...form,address:e.target.value})}/></label>
   {!id&&<label className="form-field">First sequence number<input required type="number" min="1" step="1" value={form.startNumber} disabled={busy} onChange={e=>setForm({...form,startNumber:e.target.value})}/></label>}
   {issuer&&<p>Next number: {Number.isSafeInteger(issuer.nextNumber)?issuer.numberPrefix+issuer.nextNumber:'Outside supported display range'}. Editing issuer details cannot reset the sequence.</p>}
   <button className="primary" disabled={busy}>{busy?'Saving…':id?'Save revision':'Create issuer'}</button>
  </form><h3>Location issuer</h3><form className="management-form" onSubmit={bind}>
   <label className="form-field">Location<Select required disabled={busy} value={location} onChange={e=>setLocation(e.target.value)}><option value="">Choose location</option>{data.locations.map(l=><option key={l.id} value={l.id}>{l.name}</option>)}</Select></label>
   <label className="form-field">Issuer<Select required disabled={busy} value={boundIssuer} onChange={e=>setBoundIssuer(e.target.value)}><option value="">Choose legal issuer</option>{data.issuers.map(i=><option key={i.id} value={i.id}>{i.legalName}</option>)}</Select></label>
   <button className="primary" disabled={busy||!location||!boundIssuer}>Link issuer</button>
  </form></section>;
}
