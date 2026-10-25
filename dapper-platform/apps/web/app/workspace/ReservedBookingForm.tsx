'use client';
import {useCallback, useEffect, useRef, useState} from 'react';
import Select from '../GlassSelect';
import { DateTimePicker } from '../Pickers';

type Resource = {id: string; name: string; timezone: string};
type Rule = {id: string; resourceId: string; serviceName: string; addons: {serviceId: string; name: string}[]; durationMinutes: number; beforeMinutes: number; afterMinutes: number; units: number; version: number};
type Customer = {id: string; name: string};
type Vehicle = {id: string; customerId: string; description: string};

// The person types a clock time at the shop; this turns it into the exact moment in the shop's own time zone.
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
// Minutes under an hour stay minutes, then hours, then days.
function span(total: number) {
  if (total < 60) return `${total} min`;
  if (total < 1440) { const h = Math.floor(total / 60), r = total % 60; return `${h} hr${h === 1 ? '' : 's'}${r ? ` ${r} min` : ''}`; }
  const d = Math.floor(total / 1440), h = Math.floor((total % 1440) / 60);
  return `${d} day${d === 1 ? '' : 's'}${h ? ` ${h} hr${h === 1 ? '' : 's'}` : ''}`;
}

export default function ReservedBookingForm({base, actorId, onSaved}: {base: string; actorId: string; onSaved?: () => Promise<void>}) {
  const [resources, setResources] = useState<Resource[]>([]), [rules, setRules] = useState<Rule[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]), [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [resource, setResource] = useState(''), [rule, setRule] = useState(''), [customer, setCustomer] = useState(''), [vehicle, setVehicle] = useState('');
  const [start, setStart] = useState(''), [channel, setChannel] = useState('walk_in'), [notes, setNotes] = useState('');
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const errorRef = useRef<HTMLParagraphElement>(null);
  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const responses = await Promise.all([fetch(`${base}/bookings/reservation-options`, {cache: 'no-store'}), fetch(`${base}/customers`, {cache: 'no-store'}), fetch(`${base}/vehicles`, {cache: 'no-store'})]);
      if (responses.some(response => !response.ok)) throw new Error('Booking choices are unavailable or access has changed.');
      const [options, people, equipment] = await Promise.all(responses.map(response => response.json()));
      setResources(options.resources); setRules(options.rules); setCustomers(people.items); setVehicles(equipment.items);
    } catch (failure) {setResources([]); setRules([]); setError(failure instanceof Error ? failure.message : 'Booking choices unavailable.');}
    finally {setLoading(false);}
  }, [base]);
  useEffect(() => {load();}, [load]);
  useEffect(() => {if (error && !busy && !loading) errorRef.current?.focus();}, [error, busy, loading]);
  const selectedResource = resources.find(item => item.id === resource);
  const selectedRule = rules.find(item => item.id === rule && item.resourceId === resource);
  async function save(event: React.FormEvent) {
    event.preventDefault(); if (!selectedRule || !customer) return;
    const startIso = selectedResource ? zonedIso(start, selectedResource.timezone) : null;
    if (!startIso) {setError('Pick the date and time of the booking.'); return;}
    setBusy(true); setError(''); setNotice('');
    try {
      const payload = {customerId: customer, vehicleId: null, resourceId: resource, ruleId: rule, ruleVersion: selectedRule.version, start: startIso, channel, notes: notes.trim()};
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify({actorId, payload})));
      const fingerprint = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
      const storageKey = `dapper-booking-capacity:${base}:${fingerprint}`;
      let requestKey = sessionStorage.getItem(storageKey);
      if (!requestKey) {requestKey = crypto.randomUUID(); sessionStorage.setItem(storageKey, requestKey);}
      const response = await fetch(`${base}/bookings/reserve`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({...payload, requestKey})});
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? body.errors?.booking ?? 'Booking could not be reserved. Your details are preserved.');
      sessionStorage.removeItem(storageKey);
      setNotice('Booking saved. The time is held for this job. Nothing has been paid yet.');
      setStart(''); setNotes('');
      try {await onSaved?.();} catch {setError('Booking saved, but the list could not refresh. Refresh bookings before taking another action.');}
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Request could not be confirmed. Retry unchanged details to reuse the request key.');}
    finally {setBusy(false);}
  }
  return <section className="panel detail-card" aria-label="New booking">
    <h2>New booking</h2>
    <p>Pick the customer, the service, and the day and time.</p>
    {error && <p ref={errorRef} role="alert" tabIndex={-1} className="field-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {loading ? <p role="status">Loading…</p> : <form onSubmit={save} className="management-form">
      <label className="form-field">Customer<Select required value={customer} disabled={busy} onChange={event => setCustomer(event.target.value)}><option value="">Choose a customer</option>{customers.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</Select></label>
      <label className="form-field">Service<Select required value={rule} disabled={busy} onChange={event => {const next = rules.find(item => item.id === event.target.value); setRule(event.target.value); setResource(next?.resourceId ?? '');}}><option value="">Choose a service</option>{rules.map(item => <option key={item.id} value={item.id}>{item.serviceName}{item.addons.length ? ` + ${item.addons.map(addon => addon.name).join(', ')}` : ''} · {span(item.durationMinutes)}{resources.length > 1 ? ` · ${resources.find(r => r.id === item.resourceId)?.name ?? ''}` : ''}</option>)}</Select></label>
      <label className="form-field">Date and time<DateTimePicker required value={start} disabled={busy} onChange={event => setStart(event.target.value)} /></label>
      <label className="form-field">How did they book?<Select value={channel} disabled={busy} onChange={event => setChannel(event.target.value)}><option value="walk_in">Walk-in</option><option value="online">Online</option></Select></label>
      <label className="form-field">Notes (optional)<textarea maxLength={2000} value={notes} disabled={busy} onChange={event => setNotes(event.target.value)} /></label>
      <button className="primary" disabled={busy || !selectedRule || !customer || !start}>{busy ? 'Saving…' : 'Save booking'}</button>
      {!rules.length && <p>No services are set up for booking yet. Add them in Locations &amp; Setup first.</p>}
    </form>}
  </section>;
}
