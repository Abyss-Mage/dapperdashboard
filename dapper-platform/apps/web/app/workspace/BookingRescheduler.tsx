'use client';
import {useEffect, useRef, useState} from 'react';

type Booking = {id: string; customerName: string; serviceName: string; serviceId?: string | null; addons: {serviceId: string}[]; bookingTime: string; reservationVersion?: number | null};
type Rule = {id: string; resourceId: string; serviceId: string; addonIds: string[]; version: number; durationMinutes: number; beforeMinutes: number; afterMinutes: number};
type Resource = {id: string; name: string; timezone: string};

export default function BookingRescheduler({base, booking, onClose, onSaved}: {base: string; booking: Booking; onClose: () => void; onSaved: () => Promise<void>}) {
  const dialog = useRef<HTMLDialogElement>(null), errorRef = useRef<HTMLParagraphElement>(null);
  const [resources, setResources] = useState<Resource[]>([]), [rules, setRules] = useState<Rule[]>([]);
  const [resource, setResource] = useState(''), [rule, setRule] = useState(''), [start, setStart] = useState(''), [notes, setNotes] = useState('');
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [error, setError] = useState('');
  useEffect(() => {
    dialog.current?.showModal(); let disposed = false;
    fetch(`${base}/bookings/reschedule-options`, {cache:'no-store'}).then(async response => {
      if (!response.ok) throw new Error('Scheduling choices are unavailable or your access has changed.');
      const body = await response.json(); if (disposed) return;
      const expected = booking.addons.map(item => item.serviceId).sort().join(',');
      setRules(body.rules.filter((item: Rule) => item.serviceId === booking.serviceId && [...item.addonIds].sort().join(',') === expected)); setResources(body.resources);
    }).catch(failure => {if (!disposed) setError(failure instanceof Error ? failure.message : 'Scheduling choices unavailable.');}).finally(() => {if (!disposed) setLoading(false);});
    return () => {disposed = true;};
  }, [base, booking]);
  useEffect(() => {if (error && !busy && !loading) errorRef.current?.focus();}, [error, busy, loading]);
  const selectedRule = rules.find(item => item.id === rule && item.resourceId === resource);
  async function save(event: React.FormEvent) {
    event.preventDefault(); if (!selectedRule || !booking.reservationVersion) return;
    if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d{1,3})?)?(Z|[+-]\d{2}:\d{2})$/.test(start) || !Number.isFinite(new Date(start).getTime())) {setError('Enter a valid appointment timestamp with Z or an explicit UTC offset.'); return;}
    setBusy(true); setError('');
    try {
      const payload = {resourceId:resource, ruleId:rule, ruleVersion:selectedRule.version, reservationVersion:booking.reservationVersion, start:new Date(start).toISOString(), notes:notes.trim()};
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify(payload)));
      const fingerprint = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2,'0')).join('');
      const storageKey = `dapper-booking-reschedule:${base}:${booking.id}:${fingerprint}`;
      let requestKey = sessionStorage.getItem(storageKey); if (!requestKey) {requestKey = crypto.randomUUID(); sessionStorage.setItem(storageKey,requestKey);}
      const response = await fetch(`${base}/bookings/${booking.id}/reschedule`, {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...payload,requestKey})});
      const body = await response.json(); if (!response.ok) throw new Error(body.error ?? body.errors?.booking ?? 'The booking could not be moved. Your details are preserved.');
      await onSaved(); onClose();
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'The booking could not be moved. Your details are preserved.');}
    finally {setBusy(false);}
  }
  return <dialog className="booking-reschedule" ref={dialog} aria-labelledby="reschedule-booking-title" onCancel={event => {event.preventDefault(); if (!busy) onClose();}} style={{maxWidth:'min(34rem, calc(100vw - 2rem))',padding:'1.5rem',background:'var(--panel, #fff)',color:'var(--text, var(--text, #1a0b0d))',border:'1px solid var(--border, #f0dcde)',borderRadius:'12px'}}>
    <form onSubmit={save} style={{display:'grid',gap:'.85rem'}}>
      <h3 id="reschedule-booking-title">Reschedule booking</h3>
      <p>{booking.customerName} · {booking.serviceName}</p><p>Current appointment: {booking.bookingTime}. The existing appointment stays reserved if the move fails. Payment and work order changes require their own actions.</p>
      <label>Destination resource<select autoFocus disabled={loading || busy} required value={resource} onChange={event => {setResource(event.target.value); setRule('');}}><option value="">Choose resource</option>{resources.filter(item => rules.some(choice => choice.resourceId===item.id)).map(item => <option key={item.id} value={item.id}>{item.name} · {item.timezone}</option>)}</select></label>
      <label>Service timing rule<select disabled={loading || busy || !resource} required value={rule} onChange={event => setRule(event.target.value)}><option value="">Choose current rule</option>{rules.filter(item => item.resourceId===resource).map(item => <option key={item.id} value={item.id}>{item.durationMinutes} min · {item.beforeMinutes}/{item.afterMinutes} min buffers · revision {item.version}</option>)}</select></label>
      <label>New appointment timestamp<input required disabled={busy} value={start} onChange={event => setStart(event.target.value)} placeholder="YYYY-MM-DDTHH:mm:ssZ"/></label>
      <label>Reschedule notes<textarea disabled={busy} maxLength={2000} value={notes} onChange={event => setNotes(event.target.value)}/></label>
      {loading && <p>Loading current scheduling choices…</p>}{!loading && !rules.length && !error && <p>No matching active service timing rules are configured.</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1}>{error}</p>}
      <div style={{display:'flex',gap:'.75rem',marginTop:'1rem',flexWrap:'wrap'}}><button type="button" className="secondary" disabled={busy} onClick={onClose}>Keep appointment</button><button type="submit" className="primary" disabled={busy || loading || !selectedRule || !booking.reservationVersion}>{busy?'Moving…':'Move appointment'}</button></div>
    </form>
  </dialog>;
}
