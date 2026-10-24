'use client';
import {useCallback, useEffect, useRef, useState} from 'react';

type Resource = {id: string; name: string; timezone: string};
type Rule = {id: string; resourceId: string; serviceName: string; addons: {serviceId: string; name: string}[]; durationMinutes: number; beforeMinutes: number; afterMinutes: number; units: number; version: number};
type Customer = {id: string; name: string};
type Vehicle = {id: string; customerId: string; description: string};

export default function ReservedBookingForm({base, actorId, onSaved}: {base: string; actorId: string; onSaved?: () => Promise<void>}) {
  const [resources, setResources] = useState<Resource[]>([]), [rules, setRules] = useState<Rule[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]), [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [resource, setResource] = useState(''), [rule, setRule] = useState(''), [customer, setCustomer] = useState(''), [vehicle, setVehicle] = useState('');
  const [start, setStart] = useState(''), [channel, setChannel] = useState('internal'), [notes, setNotes] = useState('');
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
    // Offset-bearing timestamps avoid silently interpreting a location time in
    // the operator browser's timezone, including repeated DST wall times.
    if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d{1,3})?)?(Z|[+-]\d{2}:\d{2})$/.test(start) || !Number.isFinite(new Date(start).getTime())) {setError('Enter a valid appointment timestamp with Z or an explicit UTC offset.'); return;}
    setBusy(true); setError(''); setNotice('');
    try {
      const payload = {customerId: customer, vehicleId: vehicle || null, resourceId: resource, ruleId: rule, ruleVersion: selectedRule.version, start: new Date(start).toISOString(), channel, notes: notes.trim()};
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify({actorId, payload})));
      const fingerprint = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
      const storageKey = `dapper-booking-capacity:${base}:${fingerprint}`;
      let requestKey = sessionStorage.getItem(storageKey);
      if (!requestKey) {requestKey = crypto.randomUUID(); sessionStorage.setItem(storageKey, requestKey);}
      const response = await fetch(`${base}/bookings/reserve`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({...payload, requestKey})});
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? body.errors?.booking ?? 'Booking could not be reserved. Your details are preserved.');
      sessionStorage.removeItem(storageKey);
      setNotice(`Pending booking ${body.id} saved with its capacity reservation. No payment or customer confirmation has been recorded.`);
      setStart(''); setNotes('');
      try {await onSaved?.();} catch {setError('Booking saved, but the list could not refresh. Refresh bookings before taking another action.');}
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Request could not be confirmed. Retry unchanged details to reuse the request key.');}
    finally {setBusy(false);}
  }
  return <section className="panel detail-card" aria-label="Booking with capacity">
    <h2>Booking with capacity</h2>
    <p>Create one pending booking and its service reservation together. Timing, buffers and resource units come from the selected configured rule.</p>
    {error && <p ref={errorRef} role="alert" tabIndex={-1} className="field-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    <button className="secondary" disabled={busy || loading} onClick={load}>Refresh booking choices</button>
    {loading ? <p role="status">Loading booking choices…</p> : <form onSubmit={save} className="management-form">
      <label className="form-field">Booking customer<select required value={customer} disabled={busy} onChange={event => {setCustomer(event.target.value); setVehicle('');}}><option value="">Choose customer</option>{customers.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <label className="form-field">Owned equipment (optional)<select value={vehicle} disabled={busy} onChange={event => setVehicle(event.target.value)}><option value="">No equipment selected</option>{vehicles.filter(item => item.customerId === customer).map(item => <option key={item.id} value={item.id}>{item.description}</option>)}</select></label>
      <label className="form-field">Booking resource<select required value={resource} disabled={busy} onChange={event => {setResource(event.target.value); setRule('');}}><option value="">Choose resource</option>{resources.map(item => <option key={item.id} value={item.id}>{item.name} · {item.timezone}</option>)}</select></label>
      <label className="form-field">Configured service combination<select required value={rule} disabled={busy || !resource} onChange={event => setRule(event.target.value)}><option value="">Choose combination</option>{rules.filter(item => item.resourceId === resource).map(item => <option key={item.id} value={item.id}>{item.serviceName}{item.addons.length ? ` + ${item.addons.map(addon => addon.name).join(', ')}` : ''} · {item.durationMinutes} min</option>)}</select></label>
      {selectedRule && <p>{selectedRule.durationMinutes} minutes · Buffers {selectedRule.beforeMinutes}/{selectedRule.afterMinutes} minutes · {selectedRule.units} resource units · Rule revision {selectedRule.version}</p>}
      <label className="form-field">Appointment timestamp with UTC offset<input required type="text" value={start} disabled={busy} onChange={event => setStart(event.target.value)} aria-describedby="booking-offset-guidance" /></label>
      <p id="booking-offset-guidance">Include Z or ±HH:MM after the time. Resource timezone: {selectedResource?.timezone ?? 'select a resource'}. Capacity is checked when saved.</p>
      <label className="form-field">Intake channel<select value={channel} disabled={busy} onChange={event => setChannel(event.target.value)}><option value="internal">Internal</option><option value="phone">Phone</option><option value="walk_in">Walk-in</option><option value="dealership">Dealership</option></select></label>
      <label className="form-field">Booking notes<textarea maxLength={2000} value={notes} disabled={busy} onChange={event => setNotes(event.target.value)} /></label>
      <button className="primary" disabled={busy || !selectedRule || !customer}>{busy ? 'Reserving…' : 'Reserve capacity and save pending booking'}</button>
      {!rules.length && <p>No active scheduling combinations are configured. Ask an authorized operator to configure approved service timing and resource windows.</p>}
    </form>}
  </section>;
}
