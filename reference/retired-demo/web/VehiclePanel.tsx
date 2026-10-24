'use client';
import {useEffect,useRef,useState} from 'react';
import {Car,Plus} from 'lucide-react';

export type Vehicle={id:string;customerId:string;name:string;category:string;identifier:string};
type Customer={id:string;name:string};
export default function VehiclePanel({vehicles,customers,onSaved}:{vehicles:Vehicle[];customers:Customer[];onSaved:()=>Promise<void>}){
 const [open,setOpen]=useState(false),[saving,setSaving]=useState(false),[errors,setErrors]=useState<Record<string,string>>({}),[notice,setNotice]=useState('');
 const formRef=useRef<HTMLFormElement>(null),addRef=useRef<HTMLButtonElement>(null);
 useEffect(()=>{if(!saving&&Object.keys(errors).length)formRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()},[errors,saving]);
 useEffect(()=>{if(notice&&!saving&&!open)addRef.current?.focus()},[notice,saving,open]);
 async function save(e:React.FormEvent<HTMLFormElement>){
  e.preventDefault();const form=e.currentTarget;setSaving(true);setErrors({});setNotice('');
  try{const response=await fetch('/api/v1/vehicles',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(Object.fromEntries(new FormData(form)))});const body=await response.json();if(!response.ok){setErrors(body.errors??{form:body.error??'Could not save vehicle'});return}await onSaved();form.reset();setOpen(false);setNotice('Demo vehicle saved');addRef.current?.focus()}
  catch{setErrors({form:'Cannot reach the API. Your entered details are preserved.'})}finally{setSaving(false)}
 }
 return <><section className="panel detail-card"><div className="section-head" style={{padding:0}}><div><h2>Customer vehicles & equipment</h2><p>Fictional demo records linked to a customer. Category descriptions do not set service eligibility or pricing.</p></div><button ref={addRef} className="secondary" onClick={()=>{setOpen(!open);setErrors({})}} disabled={saving}><Plus size={16}/> {open?'Close form':'Add vehicle'}</button></div>
 {notice&&<p role="status">{notice}</p>}
 {open&&<form ref={formRef} onSubmit={save} className="vehicle-form"><label className="form-field">Customer<select autoFocus name="customerId" required aria-invalid={!!errors.customerId} aria-describedby={errors.customerId?'vehicle-customer-error':undefined}><option value="">Choose a demo customer</option>{customers.map(c=><option value={c.id} key={c.id}>{c.name}</option>)}</select>{errors.customerId&&<small id="vehicle-customer-error" className="field-error">{errors.customerId}</small>}</label>
 {([{name:'name',label:'Vehicle or equipment description',max:120},{name:'category',label:'Category description (optional)',max:60},{name:'identifier',label:'Fictional identifier (optional)',max:60}] as const).map(f=><label key={f.name} className="form-field">{f.label}<input name={f.name} required={f.name==='name'} minLength={f.name==='name'?2:undefined} maxLength={f.max} aria-invalid={!!errors[f.name]} aria-describedby={errors[f.name]?'vehicle-'+f.name+'-error':undefined}/>{errors[f.name]&&<small className="field-error" id={'vehicle-'+f.name+'-error'}>{errors[f.name]}</small>}</label>)}
 {errors.form&&<p className="field-error" role="alert">{errors.form}</p>}<button className="primary" disabled={saving}>{saving?'Saving…':'Save demo vehicle'}</button></form>}
 </section><div className="card-grid">{vehicles.map(v=><section className="panel detail-card" key={v.id}><Car size={28}/><h2>{v.name}</h2><p>{customers.find(c=>c.id===v.customerId)?.name??'Customer unavailable'}</p><p>{v.category||'Category not specified'}{v.identifier&&` · ${v.identifier}`}</p><span className="status scheduled">Synthetic vehicle / equipment</span></section>)}</div></>;
}
