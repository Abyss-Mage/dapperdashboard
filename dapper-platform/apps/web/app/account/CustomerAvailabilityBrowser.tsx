'use client';
import {useEffect, useRef, useState} from 'react';
import Select from '../GlassSelect';

type Equipment={id:string;description:string;categoryDescription:string};
type Offer={id:string;version:number;title:string;equipmentCategory:string;mode:string;currency:string;subtotalCents:number;taxCents:number;depositCents:number;terms:string;cancellationTerms:string;durationMinutes:number;timezone:string};
type Availability={starts:string[];nextFrom?:string;to:string;timezone:string;reserved:boolean};
const money=(cents:number,currency:string)=>new Intl.NumberFormat('en-CA',{style:'currency',currency}).format(cents/100);

export default function CustomerAvailabilityBrowser({customerId,vehicles,onSubmitted}:{customerId:string;vehicles:Equipment[];onSubmitted?:()=>void}){
  const [vehicle,setVehicle]=useState(''),[offers,setOffers]=useState<Offer[]>([]),[offerId,setOfferId]=useState(''),[day,setDay]=useState('');
  const [page,setPage]=useState<Availability|null>(null),[loading,setLoading]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const [revision,setRevision]=useState(0);
  const [selectedStart,setSelectedStart]=useState(''),[customerNote,setCustomerNote]=useState(''),[reviewed,setReviewed]=useState(false),[sending,setSending]=useState(false),[receipt,setReceipt]=useState<{id:string}|null>(null);
  const receiptKey=useRef('');
  useEffect(()=>{setSelectedStart('');setReviewed(false);setReceipt(null);},[vehicle,offerId]);
  const errorRef=useRef<HTMLParagraphElement>(null);
  const base=`/api/v1/portal/accounts/${encodeURIComponent(customerId)}/vehicles/${encodeURIComponent(vehicle)}/booking-offers`;
  useEffect(()=>{
    let current=true;setOffers([]);setOfferId('');setPage(null);setError('');
    if(!vehicle){setLoading(false);return;}setLoading(true);
    fetch(base,{cache:'no-store'}).then(async response=>{if(!response.ok)throw new Error('Services are unavailable or your account access has changed.');const body=await response.json();if(current)setOffers(body.items);}).catch(failure=>{if(current)setError(failure.message);}).finally(()=>{if(current)setLoading(false);});
    return()=>{current=false;};
  },[base,vehicle,revision]);
  useEffect(()=>{if(error&&!busy&&!loading&&!sending)errorRef.current?.focus();},[error,busy,loading,sending]);
  const offer=offers.find(item=>item.id===offerId);
  async function search(more=false,selectedDay=day){
    if(!offer||(!more&&!selectedDay))return;setBusy(true);setError('');
    if(!more){setPage(null);setSelectedStart('');setReviewed(false);}
    try{
      const query=more&&page?.nextFrom?`from=${encodeURIComponent(page.nextFrom)}&to=${encodeURIComponent(page.to)}`:`day=${encodeURIComponent(selectedDay)}`;
      const response=await fetch(`${base}/${offer.id}/availability?offerVersion=${offer.version}&${query}`,{cache:'no-store'});const body=await response.json();if(!response.ok)throw new Error(body.error??'Available times could not be loaded.');
      if(body.reserved!==false)throw new Error('Available times could not be verified.');
      setPage(current=>({...body,starts:more&&current?[...new Set([...current.starts,...body.starts])]:body.starts}));
    }catch(failure){setError(failure instanceof Error?failure.message:'Available times could not be loaded.');}finally{setBusy(false);}
  }
  async function submitRequest(event:React.FormEvent){
    event.preventDefault();if(!offer||!page?.starts.includes(selectedStart)||!reviewed||receipt)return;setSending(true);setError('');
    try{
      const payload={offerId:offer.id,offerVersion:offer.version,start:selectedStart,customerNote:customerNote.trim(),termsReviewed:true};
      const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(JSON.stringify(payload)));const fingerprint=Array.from(new Uint8Array(digest),value=>value.toString(16).padStart(2,'0')).join('');
      const endpoint=`/api/v1/portal/accounts/${encodeURIComponent(customerId)}/vehicles/${encodeURIComponent(vehicle)}/booking-requests`;
      const key=`dapper-customer-request:${endpoint}:${fingerprint}`;let requestKey=sessionStorage.getItem(key);if(!requestKey){requestKey=crypto.randomUUID();sessionStorage.setItem(key,requestKey);}receiptKey.current=key;
      const response=await fetch(endpoint,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...payload,requestKey})});const body=await response.json();if(!response.ok)throw new Error(body.error??'Your request could not be submitted. Your details are preserved.');
      if(body.status!=='requested'||body.reserved!==false||!body.id)throw new Error('The request result could not be verified. Keep these details and retry.');setReceipt({id:body.id});onSubmitted?.();
    }catch(failure){setError(failure instanceof TypeError||failure instanceof SyntaxError?'The request result is uncertain. Check your request history or retry with the same details.':failure instanceof Error?failure.message:'Your request could not be submitted. Your details are preserved.');}finally{setSending(false);}
  }
  return <section className="panel detail-card"><h3>Service availability</h3><p>View services and available appointment times for your equipment. Checking times does not reserve an appointment.</p>
    {error&&<p ref={errorRef} tabIndex={-1} role="alert" className="field-error">{error}</p>}
    <label className="form-field">Your equipment<Select value={vehicle} disabled={busy||sending} onChange={event=>setVehicle(event.target.value)}><option value="">Choose your vehicle or equipment</option>{vehicles.map(item=><option key={item.id} value={item.id}>{item.description} · {item.categoryDescription||'Category not provided'}</option>)}</Select></label>
    {vehicle&&<button type="button" className="secondary" disabled={busy||loading||sending} onClick={()=>setRevision(current=>current+1)}>Refresh services</button>}
    {loading&&<p role="status">Loading services…</p>}{vehicle&&!loading&&!offers.length&&!error&&<p>No online services are currently published for this equipment category. Please contact Dapper.</p>}
    {!!offers.length&&<label className="form-field">Service<Select value={offerId} disabled={busy||sending} onChange={event=>{setOfferId(event.target.value);setPage(null);setError('');}}><option value="">Choose service</option>{offers.map(item=><option key={item.id} value={item.id}>{item.title}</option>)}</Select></label>}
    {offer&&<>
      <h4>{offer.title}</h4><p>{offer.durationMinutes} minutes · Appointment times in {offer.timezone}</p>
      {offer.mode==='quote_required'?<p>A reviewed quote is required before a price and appointment can be confirmed.</p>:<p>Service {money(offer.subtotalCents,offer.currency)} + tax {money(offer.taxCents,offer.currency)} = {money(offer.subtotalCents+offer.taxCents,offer.currency)}. Required deposit: {money(offer.depositCents,offer.currency)}.</p>}
      <details><summary>Service and payment terms</summary><p style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{offer.terms}</p></details>
      <details><summary>Cancellation and reschedule terms</summary><p style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{offer.cancellationTerms}</p></details>
      <form onSubmit={event=>{event.preventDefault();const selectedDay=String(new FormData(event.currentTarget).get('appointmentDay')??'');setDay(selectedDay);search(false,selectedDay);}}><label className="form-field">Appointment date ({offer.timezone})<input name="appointmentDay" type="date" required defaultValue={day} disabled={busy||sending} onInput={event=>{setDay(event.currentTarget.value);setPage(null);}} onChange={event=>{setDay(event.target.value);setPage(null);}}/></label><button className="primary" disabled={busy||sending}>{busy?'Checking times…':'Check available times'}</button></form>
      {page&&<div aria-live="polite"><h4>Available starts</h4>{page.starts.length?<ul>{page.starts.map(start=><li key={start}>{new Intl.DateTimeFormat('en-CA',{dateStyle:undefined,year:'numeric',month:'short',day:'numeric',hour:'numeric',minute:'2-digit',timeZone:page.timezone,timeZoneName:'shortOffset'}).format(new Date(start))}</li>)}</ul>:<p>No available starts found for this date.</p>}{page.nextFrom&&<button type="button" className="secondary" disabled={busy||sending} onClick={()=>search(true)}>More times</button>}<p>Availability can change. No booking, charge or payment confirmation has been created.</p></div>}
      {receipt?<div role="status"><h4>Unconfirmed request submitted</h4><p>Reference: {receipt.id}. No appointment capacity is reserved and no payment has been collected.</p><button type="button" className="secondary" onClick={()=>{try{if(receiptKey.current)sessionStorage.removeItem(receiptKey.current);setReceipt(null);setSelectedStart('');setReviewed(false);setCustomerNote('');}catch{setError('A new request could not be started. Please retry.');}}}>Start another request</button></div>:!!page?.starts.length&&<form onSubmit={submitRequest} style={{marginTop:'1.25rem',display:'grid',gap:'.75rem'}}><h4>Request an appointment</h4><p>This sends an unconfirmed request for review. Quote approval, payment and reservation are separate steps.</p><label className="form-field">Requested start<Select required value={selectedStart} disabled={busy||sending} onChange={event=>{setSelectedStart(event.target.value);setReviewed(false);}}><option value="">Choose an available start</option>{page.starts.map(start=><option key={start} value={start}>{new Intl.DateTimeFormat('en-CA',{year:'numeric',month:'short',day:'numeric',hour:'numeric',minute:'2-digit',timeZone:page.timezone,timeZoneName:'shortOffset'}).format(new Date(start))}</option>)}</Select></label><label className="form-field">Request note (optional)<textarea maxLength={2000} disabled={sending} value={customerNote} onChange={event=>{setCustomerNote(event.target.value);setReviewed(false);}}/></label><label><input type="checkbox" checked={reviewed} disabled={sending} onChange={event=>setReviewed(event.target.checked)}/>I reviewed the displayed service, amounts, payment and cancellation information.</label><button className="primary" disabled={busy||sending||!reviewed||!selectedStart||!page.starts.includes(selectedStart)}>{sending?'Submitting request…':'Send unconfirmed request'}</button></form>}
    </>}
  </section>;
}

