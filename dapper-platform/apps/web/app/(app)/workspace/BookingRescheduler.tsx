'use client';
import {useEffect, useRef, useState} from 'react';
import Select from '../GlassSelect';
import { DatePicker, DateTimePicker } from '../Pickers';

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
    dialog.current?.show(); let disposed = false;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !document.querySelector('[role="listbox"], [data-picker-open]')) onClose(); };
    window.addEventListener('keydown', onKey);
    fetch(`${base}/bookings/reschedule-options`, {cache:'no-store'}).then(async response => {
      if (!response.ok) throw new Error('Scheduling choices are unavailable or your access has changed.');
      const body = await response.json(); if (disposed) return;
      const expected = booking.addons.map(item => item.serviceId).sort().join(',');
      setRules(body.rules.filter((item: Rule) => item.serviceId === booking.serviceId && [...item.addonIds].sort().join(',') === expected)); setResources(body.resources);
    }).catch(failure => {if (!disposed) setError(failure instanceof Error ? failure.message : 'Scheduling choices unavailable.');}).finally(() => {if (!disposed) setLoading(false);});
    return () => {disposed = true; window.removeEventListener('keydown', onKey);};
  }, [base, booking]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {if (error && !busy && !loading) errorRef.current?.focus();}, [error, busy, loading]);
  const usable = resources.filter(item => rules.some(choice => choice.resourceId === item.id));
  useEffect(() => {if (!resource && usable.length === 1) setResource(usable[0].id);}, [usable, resource]);
  useEffect(() => {const mine = rules.filter(item => item.resourceId === resource); if (resource && !rule && mine.length === 1) setRule(mine[0].id);}, [rules, resource, rule]);
  const selectedRule = rules.find(item => item.id === rule && item.resourceId === resource);
  const zone = resources.find(item => item.id === resource)?.timezone ?? 'America/Toronto';
  // With no service timing rule to follow, the booking is moved by hand: it gives its calendar slot back and keeps the new time.
  const detached = !loading && !rules.length;
  // With a service timing rule the new time is chosen from the times the calendar can actually offer that day, so a time outside
  // opening hours or on a full slot cannot be picked.
  const [day, setDay] = useState(''), [slots, setSlots] = useState<string[] | null>(null), [picked, setPicked] = useState('');
  useEffect(() => {
    setPicked(''); setSlots(null);
    if (detached || !selectedRule || !day) return;
    let live = true;
    const q = new URLSearchParams({resourceId: resource, ruleId: rule, version: String(selectedRule.version), from: day, days: '1', past: '1'});
    fetch(`${base}/bookings/availability?${q}`, {cache: 'no-store'}).then(async r => { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'The available times could not be loaded.'); return b; })
      .then(b => { if (live) setSlots((b.days?.[0]?.starts ?? []) as string[]); })
      .catch(e => { if (live) { setSlots([]); setError(e instanceof Error ? e.message : 'The available times could not be loaded.'); } });
    return () => { live = false; };
  }, [day, resource, rule, selectedRule?.version, detached]); // eslint-disable-line react-hooks/exhaustive-deps
  const slotLabel = (iso: string) => new Date(iso).toLocaleTimeString('en-CA', {hour: 'numeric', minute: '2-digit', timeZone: zone});
  async function save(event: React.FormEvent) {
    event.preventDefault();
    const startIso = detached ? zonedIso(start, zone) : picked;
    if (!startIso) {setError(detached ? 'Pick the new date and time.' : 'Pick a day, then one of the times offered.'); return;}
    if (detached) {
      setBusy(true); setError('');
      try {
        const response = await fetch(`${base}/bookings/${booking.id}/move`, {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({bookingTime:startIso})});
        const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'The booking could not be moved.');
        await onSaved(); onClose();
      } catch (failure) {setError(failure instanceof Error ? failure.message : 'The booking could not be moved.');}
      finally {setBusy(false);}
      return;
    }
    if (!selectedRule || !booking.reservationVersion) return;
    setBusy(true); setError('');
    try {
      const payload = {resourceId:resource, ruleId:rule, ruleVersion:selectedRule.version, reservationVersion:booking.reservationVersion, start:startIso, notes:notes.trim()};
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify(payload)));
      const fingerprint = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2,'0')).join('');
      const storageKey = `dapper-booking-reschedule:${base}:${booking.id}:${fingerprint}`;
      let requestKey = sessionStorage.getItem(storageKey); if (!requestKey) {requestKey = crypto.randomUUID(); sessionStorage.setItem(storageKey,requestKey);}
      const response = await fetch(`${base}/bookings/${booking.id}/reschedule`, {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...payload,requestKey})});
      const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(response.status === 409 ? 'That time is no longer available, or the booking changed while you were choosing. Pick a time from the list again.' : (body.error ?? body.errors?.booking ?? 'The booking could not be moved. Your details are preserved.'));
      await onSaved(); onClose();
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'The booking could not be moved. Your details are preserved.');}
    finally {setBusy(false);}
  }
  return <><div aria-hidden="true" onClick={() => { if (!busy) onClose(); }} style={{position:'fixed',inset:0,zIndex:1400,background:'rgb(0 0 0 / 45%)'}} />
  <dialog className="booking-reschedule" ref={dialog} aria-labelledby="reschedule-booking-title" style={{position:'fixed',zIndex:1500,top:'50%',left:'50%',transform:'translate(-50%,-50%)',margin:0,width:'min(34rem, calc(100vw - 2rem))',padding:'1.5rem'}}>
    <form onSubmit={save} style={{display:'grid',gap:'.85rem'}}>
      <h3 id="reschedule-booking-title">Move this booking</h3>
      <p><strong>{booking.customerName}</strong> · {booking.serviceName}</p><p>Now booked for {new Date(booking.bookingTime).toLocaleString('en-CA', {weekday: 'long', month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZone: zone})}. {detached ? '' : ' If the move cannot be made, the current time stays reserved.'}</p>
      {usable.length > 1 && <label>Where<Select autoFocus disabled={loading || busy} required value={resource} onChange={event => {setResource(event.target.value); setRule('');}}><option value="">Choose where</option>{usable.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</Select></label>}
      {rules.filter(item => item.resourceId===resource).length > 1 && <label>Service timing<Select disabled={loading || busy || !resource} required value={rule} onChange={event => setRule(event.target.value)}><option value="">Choose</option>{rules.filter(item => item.resourceId===resource).map(item => <option key={item.id} value={item.id}>{item.durationMinutes} min</option>)}</Select></label>}
      {detached
        ? <label>New date and time<DateTimePicker autoFocus={usable.length <= 1} required disabled={busy} value={start} onChange={event => setStart(event.target.value)}/></label>
        : <>
            <label>New day<DatePicker value={day} disabled={busy || !selectedRule} onChange={event => setDay(event.target.value)} aria-label="New day"/></label>
            {day && slots === null && <p>Loading the times for that day…</p>}
            {day && slots !== null && slots.length === 0 && <p>No times are open on that day. Pick another day.</p>}
            {slots !== null && slots.length > 0 && <div role="group" aria-label="Available times" style={{display:'flex',flexWrap:'wrap',gap:'.4rem'}}>
              {slots.map(t => <button key={t} type="button" className={picked === t ? 'primary' : 'secondary'} aria-pressed={picked === t} disabled={busy} onClick={() => setPicked(t)} style={{minHeight:40,padding:'.35rem .8rem'}}>{slotLabel(t)}</button>)}
            </div>}
          </>}
      <label>Reason or notes (optional)<textarea disabled={busy} maxLength={2000} value={notes} onChange={event => setNotes(event.target.value)}/></label>
      {loading && <p>Loading current scheduling choices…</p>}{detached && !error && <p>This service has no active timing rules, so the booking will be moved by hand to the time you pick. It will no longer hold a calendar slot.</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1}>{error}</p>}
      <div style={{display:'flex',gap:'.75rem',marginTop:'1rem',flexWrap:'wrap'}}><button type="button" className="secondary" disabled={busy} onClick={onClose}>Keep it as is</button><button type="submit" className="primary" disabled={busy || loading || (detached ? !start : (!picked || !selectedRule || !booking.reservationVersion))}>{busy?'Moving…':'Move booking'}</button></div>
    </form>
  </dialog></>;
}
