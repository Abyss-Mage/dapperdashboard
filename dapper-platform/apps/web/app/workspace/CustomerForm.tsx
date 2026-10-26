'use client';
import {useState} from 'react';

export default function CustomerForm({base,onSaved}:{base:string;onSaved:()=>void}) {
 const [name,setName]=useState(''),[email,setEmail]=useState(''),[phone,setPhone]=useState('');
 const [busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[fields,setFields]=useState<Record<string,string>>({});
 async function save(event:React.FormEvent<HTMLFormElement>) {
  event.preventDefault();const form=event.currentTarget;setError('');setNotice('');
  const normalized={name:name.trim(),email:email.trim(),phone:phone.trim()};
  const invalid:Record<string,string>={};
  if(Array.from(normalized.name).length<2||Array.from(normalized.name).length>100)invalid.name='Enter a name between 2 and 100 characters.';
  setFields(invalid);
  if(Object.keys(invalid).length){queueMicrotask(()=>form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus());return}
  setBusy(true);
  try {
   const response=await fetch(base+'/customers',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(normalized)});
   const body=await response.json();
   if(!response.ok){setFields(body.errors??{});setError(response.status===401?'Your session expired. Sign in again before retrying.':response.status===403?'Your account cannot add customers at this location.':body.error??'Check the highlighted customer fields.');return}
   setName('');setEmail('');setPhone('');setNotice('Customer saved to this location. They are now available in the work-order customer selector.');onSaved();
  } catch {setError('Could not confirm that the customer was saved. Your input is preserved. Check this location’s customers before retrying to avoid a duplicate.');}
  finally {setBusy(false);queueMicrotask(()=>form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus());}
 }
 return <section className="panel detail-card"><h2>Add a customer</h2><p>Saved only to the selected location. Contact details are optional; adding a customer does not subscribe them to marketing.</p>
  {error&&<p className="field-error" role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
  <form onSubmit={save}><div className="management-form">
   <label className="form-field">Customer name<input required disabled={busy} maxLength={100} autoComplete="name" value={name} onChange={e=>setName(e.target.value)} aria-invalid={!!fields.name} aria-describedby={fields.name?'customer-name-error':undefined}/>{fields.name&&<small id="customer-name-error" className="field-error">{fields.name}</small>}</label>
   <label className="form-field">Email (optional)<input type="email" disabled={busy} maxLength={254} autoComplete="email" value={email} onChange={e=>setEmail(e.target.value)} aria-invalid={!!fields.email} aria-describedby={fields.email?'customer-email-error':undefined}/>{fields.email&&<small id="customer-email-error" className="field-error">{fields.email}</small>}</label>
   <label className="form-field">Phone (optional)<input type="tel" disabled={busy} maxLength={30} autoComplete="tel" value={phone} onChange={e=>setPhone(e.target.value)} aria-invalid={!!fields.phone} aria-describedby={fields.phone?'customer-phone-error':undefined}/>{fields.phone&&<small id="customer-phone-error" className="field-error">{fields.phone}</small>}</label>
  </div><button className="primary" disabled={busy}>{busy?'Saving…':'Save customer'}</button></form>
 </section>;
}

