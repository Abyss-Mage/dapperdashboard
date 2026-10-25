'use client';
import {useCallback, useEffect, useRef, useState} from 'react';
import Select from '../GlassSelect';

type Rule = {id:string;resourceId:string;version:number;serviceName:string;durationMinutes:number;addons:{name:string}[]};
type Resource = {id:string;name:string;timezone:string};
type Offer = {id:string;version:number;resourceId:string;ruleId:string;ruleVersion:number;title:string;equipmentCategory:string;mode:string;currency:string;subtotalCents:number;taxCents:number;depositCents:number;terms:string;cancellationTerms:string;leadMinutes:number;advanceDays:number;slotMinutes:number;published:boolean};
type Draft = {resourceId:string;ruleId:string;title:string;equipmentCategory:string;mode:string;subtotalCents:string;taxCents:string;depositCents:string;terms:string;cancellationTerms:string;leadMinutes:string;advanceDays:string;slotMinutes:string;published:boolean};
const empty: Draft = {resourceId:'',ruleId:'',title:'',equipmentCategory:'',mode:'',subtotalCents:'',taxCents:'',depositCents:'',terms:'',cancellationTerms:'',leadMinutes:'',advanceDays:'',slotMinutes:'',published:false};
function draftFromOffer(item:Offer):Draft{return {resourceId:item.resourceId,ruleId:item.ruleId,title:item.title,equipmentCategory:item.equipmentCategory,mode:item.mode,subtotalCents:String(item.subtotalCents),taxCents:String(item.taxCents),depositCents:String(item.depositCents),terms:item.terms,cancellationTerms:item.cancellationTerms,leadMinutes:String(item.leadMinutes),advanceDays:String(item.advanceDays),slotMinutes:String(item.slotMinutes),published:item.published};}

export default function BookingOfferManager({base}:{base:string}) {
  const [items,setItems]=useState<Offer[]>([]),[resources,setResources]=useState<Resource[]>([]),[rules,setRules]=useState<Rule[]>([]);
  const [editing,setEditing]=useState<Offer|null>(null),[draft,setDraft]=useState<Draft>(empty),[reviewed,setReviewed]=useState(false);
  const [busy,setBusy]=useState(false),[loading,setLoading]=useState(true),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const errorRef=useRef<HTMLParagraphElement>(null),titleRef=useRef<HTMLInputElement>(null);
  const load=useCallback(async()=>{
    const responses=await Promise.all([fetch(`${base}/booking-offers`,{cache:'no-store'}),fetch(`${base}/booking-offers/options`,{cache:'no-store'})]);
    if(responses.some(response=>!response.ok))throw new Error('Booking offers or current rule choices could not be loaded.');
    const [list,choices]=await Promise.all(responses.map(response=>response.json()));setItems(list.items);setResources(choices.resources);setRules(choices.rules);
  },[base]);
  useEffect(()=>{load().catch(failure=>setError(failure.message)).finally(()=>setLoading(false));},[load]);
  useEffect(()=>{if(error&&!busy&&!loading)errorRef.current?.focus();},[error,busy,loading]);
  function change<K extends keyof Draft>(key:K,value:Draft[K]){setDraft(current=>({...current,[key]:value}));setReviewed(false);setNotice('');}
  function edit(item:Offer){setEditing(item);setDraft(draftFromOffer(item));setReviewed(false);setError('');titleRef.current?.focus();}
  const selectedRule=rules.find(item=>item.id===draft.ruleId&&item.resourceId===draft.resourceId);
  async function save(event:React.FormEvent){
    event.preventDefault();setError('');setNotice('');
    if(draft.published&&!selectedRule){setError('Choose a current service timing rule before publishing.');return;}
    const numeric=['subtotalCents','taxCents','depositCents','leadMinutes','advanceDays','slotMinutes'] as const;
    if(numeric.some(key=>!/^\d+$/.test(draft[key])||!Number.isSafeInteger(Number(draft[key])))){setError('Enter explicit whole-number cents and scheduling settings.');return;}
    setBusy(true);
    try{
      const response=await fetch(`${base}/booking-offers`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({...draft,id:editing?.id??'',expectedVersion:editing?.version??0,ruleVersion:selectedRule?.version??editing?.ruleVersion,currency:'CAD',reviewed,...Object.fromEntries(numeric.map(key=>[key,Number(draft[key])]))})});
      const body=await response.json();if(!response.ok)throw new Error(body.error??'Offer could not be saved. Your input is preserved.');
      setEditing(body);setDraft(draftFromOffer(body));setReviewed(false);setNotice(`Offer revision ${body.version} saved${body.published?' for customer publication': ' as unpublished'}.`);await load();
    }catch(failure){setError(failure instanceof Error?failure.message:'Offer could not be saved. Your input is preserved.');}finally{setBusy(false);}
  }
  return <section className="panel detail-card"><h2>Customer booking offers</h2><p>Publish exact service terms, equipment eligibility and amounts only after review. Changes create revisions. An offer becomes unavailable to customers when its scheduling rule changes until it is reviewed again.</p>
    {error&&<p ref={errorRef} role="alert" tabIndex={-1} className="field-error">{error}</p>}{notice&&<p role="status">{notice}</p>}
    <form className="management-form" onSubmit={save}><fieldset disabled={busy||loading} style={{border:0,padding:0,margin:0,display:'grid',gap:'1rem',minWidth:0}}>
      <label className="form-field">Offer title<input ref={titleRef} required minLength={2} maxLength={120} value={draft.title} onChange={event=>change('title',event.target.value)}/></label>
      <label className="form-field">Equipment category<input required maxLength={60} value={draft.equipmentCategory} onChange={event=>change('equipmentCategory',event.target.value)}/></label>
      <label className="form-field">Scheduling resource<Select required value={draft.resourceId} onChange={event=>{change('resourceId',event.target.value);change('ruleId','');}}><option value="">Choose resource</option>{resources.map(item=><option key={item.id} value={item.id}>{item.name} · {item.timezone}</option>)}</Select></label>
      <label className="form-field">Exact service combination<Select required disabled={!draft.resourceId} value={draft.ruleId} onChange={event=>change('ruleId',event.target.value)}><option value="">Choose current rule</option>{rules.filter(item=>item.resourceId===draft.resourceId).map(item=><option key={item.id} value={item.id}>{item.serviceName}{item.addons.map(addon=>` + ${addon.name}`).join('')} · {item.durationMinutes} min · revision {item.version}</option>)}{editing&&!selectedRule&&draft.ruleId&&<option value={draft.ruleId}>Previously configured rule · unavailable</option>}</Select></label>
      <label className="form-field">Pricing mode<Select required value={draft.mode} onChange={event=>change('mode',event.target.value)}><option value="">Choose mode</option><option value="quote_required">Quote required</option><option value="fixed">Fixed amount</option></Select></label>
      <p>Amounts are whole Canadian cents. Quote-required offers must explicitly use zero amounts; no payment or confirmation is created by publishing.</p>
      <label className="form-field">Service amount (CAD cents)<input required inputMode="numeric" pattern="[0-9]+" value={draft.subtotalCents} onChange={event=>change('subtotalCents',event.target.value)}/></label>
      <label className="form-field">Tax amount (CAD cents)<input required inputMode="numeric" pattern="[0-9]+" value={draft.taxCents} onChange={event=>change('taxCents',event.target.value)}/></label>
      <label className="form-field">Required deposit (CAD cents)<input required inputMode="numeric" pattern="[0-9]+" value={draft.depositCents} onChange={event=>change('depositCents',event.target.value)}/></label>
      <label className="form-field">Minimum notice (minutes)<input required inputMode="numeric" pattern="[0-9]+" value={draft.leadMinutes} onChange={event=>change('leadMinutes',event.target.value)}/></label>
      <label className="form-field">Advance booking window (days)<input required inputMode="numeric" pattern="[0-9]+" value={draft.advanceDays} onChange={event=>change('advanceDays',event.target.value)}/></label>
      <label className="form-field">Start-time interval (minutes)<input required inputMode="numeric" pattern="[0-9]+" value={draft.slotMinutes} onChange={event=>change('slotMinutes',event.target.value)}/></label>
      <label className="form-field">Service and payment terms<textarea required maxLength={10000} value={draft.terms} onChange={event=>change('terms',event.target.value)}/></label>
      <label className="form-field">Cancellation and reschedule terms<textarea required maxLength={10000} value={draft.cancellationTerms} onChange={event=>change('cancellationTerms',event.target.value)}/></label>
      <label><input type="checkbox" checked={draft.published} onChange={event=>change('published',event.target.checked)}/>Publish for matching customer equipment</label>
      {draft.published&&<label><input type="checkbox" checked={reviewed} onChange={event=>setReviewed(event.target.checked)}/>I reviewed this service rule, equipment category, amounts and customer terms for publication.</label>}
      <button className="primary" disabled={busy||loading||(draft.published&&!reviewed)}>{busy?'Saving…':editing?'Save offer revision':'Create offer'}</button>
      {editing&&<button type="button" className="secondary" onClick={()=>{setEditing(null);setDraft(empty);setReviewed(false);setNotice('');setError('');}}>New offer</button>}
    </fieldset></form>
    <div className="management-list">{items.map(item=><article key={item.id}><div><strong>{item.title}</strong><small>{item.equipmentCategory} · {item.mode==='fixed'?new Intl.NumberFormat('en-CA',{style:'currency',currency:'CAD'}).format((item.subtotalCents+item.taxCents)/100):'Quote required'} · revision {item.version} · {item.published?'Published configuration':'Unpublished'}</small></div><button type="button" className="secondary" disabled={busy} onClick={()=>edit(item)}>Review or edit</button></article>)}</div>{!loading&&!items.length&&<p>No booking offers configured.</p>}
  </section>;
}
