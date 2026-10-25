'use client';
import {useEffect, useRef, useState} from 'react';
import Select from '../GlassSelect';
import { DateTimePicker } from '../Pickers';

type Booking = {id: string; customerName: string; serviceName: string; serviceId?: string | null; addons: {serviceId: string}[]; bookingTime: string; reservationVersion?: number | null};
type Rule = {id: string; resourceId: string; serviceId: string; addonIds: string[]; version: number; durationMinutes: number; beforeMinutes: number; afterMinutes: number};
type Resource = {id: string; name: string; timezone: string};

// A clock time typed for the shop, turned into the exact moment in the shop's own time zone.
function zonedIso(local: string, zone: string): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local);
  if (!m) return null;
  const wall = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5]);
  const offsetAt = (utc: number) => {
    const p = Object.fromEntries(new Intl.DateTimeFormat('en-US', {timeZone: zone, hourCycle: 'h23', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit'}).formatToParts(new Date(utc)).map(x => [x.type, x.value]));
    return Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second) - utc;
  };
  let utc = wall - offsetAt(wall);
  utc = wall - offsetAt(utc);
  return Number.isFinite(utc) ? new Date(utc).toISOString() : null;
}

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
  const usable = resources.filter(item => rules.some(choice => choice.resourceId === item.id));
  useEffect(() => {if (!resource && usable.length === 1) setResource(usable[0].id);}, [usable, resource]);
  useEffect(() => {const mine = rules.filter(item => item.resourceId === resource); if (resource && !rule && mine.length === 1) setRule(mine[0].id);}, [rules, resource, rule]);
  const selectedRule = rules.find(item => item.id === rule && item.resourceId === resource);
  const zone = resources.find(item => item.id === resource)?.timezone ?? 'America/Toronto';
  async function save(event: React.FormEvent) {
    event.preventDefault(); if (!selectedRule || !booking.reservationVersion) return;
    const startIso = zonedIso(start, zone);
    if (!startIso) {setError('Pick the new date and time.'); return;}
    setBusy(true); setError('');
    try {
      const payload = {resourceId:resource, ruleId:rule, ruleVersion:selectedRule.version, reservationVersion:booking.reservationVersion, start:startIso, notes:notes.trim()};
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
  return <dialog className="booking-reschedule" ref={dialog} aria-labelledby="reschedule-booking-title" onCancel={event => {event.preventDefault(); if (!busy) onClose();}} style={{width:'min(34rem, calc(100vw - 2rem))',padding:'1.5rem'}}>
    <form onSubmit={save} style={{display:'grid',gap:'.85rem'}}>
      <h3 id="reschedule-booking-title">Move this booking</h3>
      <p><strong>{booking.customerName}</strong> · {booking.serviceName}</p><p>Now booked for {new Date(booking.bookingTime).toLocaleString('en-CA', {weekday: 'long', month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZone: zone})}. If the move cannot be made, the current time stays reserved.</p>
      {usable.length > 1 && <label>Where<Select autoFocus disabled={loading || busy} required value={resource} onChange={event => {setResource(event.target.value); setRule('');}}><option value="">Choose where</option>{usable.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</Select></label>}
      {rules.filter(item => item.resourceId===resource).length > 1 && <label>Service timing<Select disabled={loading || busy || !resource} required value={rule} onChange={event => setRule(event.target.value)}><option value="">Choose</option>{rules.filter(item => item.resourceId===resource).map(item => <option key={item.id} value={item.id}>{item.durationMinutes} min</option>)}</Select></label>}
      <label>New date and time<DateTimePicker autoFocus={usable.length <= 1} required disabled={busy} value={start} onChange={event => setStart(event.target.value)}/></label>
      <label>Reason or notes (optional)<textarea disabled={busy} maxLength={2000} value={notes} onChange={event => setNotes(event.target.value)}/></label>
      {loading && <p>Loading current scheduling choices…</p>}{!loading && !rules.length && !error && <p>No matching active service timing rules are configured.</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1}>{error}</p>}
      <div style={{display:'flex',gap:'.75rem',marginTop:'1rem',flexWrap:'wrap'}}><button type="button" className="secondary" disabled={busy} onClick={onClose}>Keep it as is</button><button type="submit" className="primary" disabled={busy || loading || !selectedRule || !booking.reservationVersion}>{busy?'Moving…':'Move booking'}</button></div>
    </form>
  </dialog>;
}
