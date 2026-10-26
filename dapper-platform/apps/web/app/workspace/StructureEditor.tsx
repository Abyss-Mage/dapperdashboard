'use client';
import { useCallback, useEffect, useState } from 'react';
import ConfirmDialog from './ConfirmDialog';

type Address = { street: string; city: string; region: string; postalCode: string; country: string };
type Franchise = { id: string; franchiseId: string | null; code: string; name: string; address: Address };
const emptyAddress: Address = { street: '', city: '', region: '', postalCode: '', country: '' };
const addressLine = (a: Address) => [a.street, a.city, a.region, a.postalCode, a.country].filter(Boolean).join(', ');

/* A franchise is one site: a unique code, a name and an optional address. There is no separate "location" to manage;
   the franchise id shown here is the id used for bookings, staff, scheduling, invoices and GPS. */
export default function StructureEditor({ organization, onSaved }: { organization: { id: string; name: string; actions: string[] }; onSaved: () => Promise<void> }) {
  const [franchises, setFranchises] = useState<Franchise[]>([]);
  const [code, setCode] = useState(''), [name, setName] = useState(''), [address, setAddress] = useState<Address>(emptyAddress), [showAddress, setShowAddress] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<Franchise | null>(null);
  const [error, setError] = useState(''), [notice, setNotice] = useState(''), [busy, setBusy] = useState(false), [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const base = '/api/v1/organizations/' + organization.id;
  const canManage = organization.actions.includes('structure.manage');

  const load = useCallback(async () => {
    const r = await fetch(base + '/structure', { cache: 'no-store' });
    if (!r.ok) throw Error('Could not load franchises');
    setFranchises((await r.json()).franchises);
  }, [base]);
  useEffect(() => { load().catch(e => setError(e.message)); }, [load]);

  function reset() { setEditing(null); setCode(''); setName(''); setAddress(emptyAddress); setShowAddress(false); setFieldErrors({}); }
  function edit(f: Franchise) { setEditing(f.id); setCode(f.code); setName(f.name); setAddress(f.address); setShowAddress(!!addressLine(f.address)); setError(''); setNotice(''); setFieldErrors({}); }

  async function save(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError(''); setNotice(''); setFieldErrors({});
    try {
      const r = await fetch(editing ? `${base}/franchises/${editing}` : `${base}/franchises`, { method: editing ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code, name, address }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) { setFieldErrors(body.errors ?? {}); setError(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'The franchise could not be saved.'); return; }
      setNotice(editing ? 'Franchise updated.' : 'Franchise created.'); reset(); await load(); await onSaved();
    } catch { setError('Could not save the franchise. Your input is kept.'); }
    finally { setBusy(false); }
  }
  async function confirmDelete() {
    if (!confirmTarget) return; setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(`${base}/franchises/${confirmTarget.id}`, { method: 'DELETE' });
      if (!r.ok) { const body = await r.json().catch(() => ({})); throw new Error(r.status === 409 ? (body.error ?? 'This franchise is in use and cannot be deleted.') : (body.error ?? 'The franchise could not be deleted.')); }
      setNotice(`${confirmTarget.name} deleted.`); setConfirmTarget(null); await load(); await onSaved();
    } catch (err) { setError(err instanceof Error ? err.message : 'The franchise could not be deleted.'); setConfirmTarget(null); }
    finally { setBusy(false); }
  }
  const setField = (k: keyof Address) => (e: React.ChangeEvent<HTMLInputElement>) => setAddress({ ...address, [k]: e.target.value });

  return <section className="panel detail-card"><h2>Franchises</h2>
    <p>Each franchise is one site, identified by its code. Staff, bookings, scheduling and invoices belong to a franchise. Partner dealerships are separate accounts (B2B Fleet Accounts), not franchises.</p>
    {error && <p role="alert" className="field-error">{error}</p>}{notice && <p role="status">{notice}</p>}
    {canManage && <form onSubmit={save} className="management-form">
      <h3>{editing ? 'Edit franchise' : 'Create a franchise'}</h3>
      <label className="form-field">Franchise code<input required maxLength={12} value={code} onChange={e => setCode(e.target.value.toUpperCase())} disabled={busy} placeholder="e.g. PTBO" autoComplete="off" aria-describedby="code-help" aria-invalid={!!fieldErrors.code} /><small id="code-help">2–12 letters, numbers or hyphens. Unique within the organization.</small>{fieldErrors.code && <span className="field-error">{fieldErrors.code}</span>}</label>
      <label className="form-field">Franchise name<input required minLength={2} maxLength={120} value={name} onChange={e => setName(e.target.value)} disabled={busy} aria-invalid={!!fieldErrors.name} />{fieldErrors.name && <span className="field-error">{fieldErrors.name}</span>}</label>
      <label className="check-label"><input type="checkbox" checked={showAddress} onChange={e => setShowAddress(e.target.checked)} disabled={busy} /> Add an address for this franchise (optional)</label>
      {showAddress && <fieldset className="management-form"><legend>Address</legend>
        <label className="form-field">Street<input maxLength={200} value={address.street} onChange={setField('street')} disabled={busy} autoComplete="street-address" /></label>
        <label className="form-field">City<input maxLength={100} value={address.city} onChange={setField('city')} disabled={busy} autoComplete="address-level2" /></label>
        <label className="form-field">Province / state<input maxLength={100} value={address.region} onChange={setField('region')} disabled={busy} autoComplete="address-level1" /></label>
        <label className="form-field">Postal / ZIP code<input maxLength={20} value={address.postalCode} onChange={setField('postalCode')} disabled={busy} autoComplete="postal-code" /></label>
        <label className="form-field">Country<input maxLength={100} value={address.country} onChange={setField('country')} disabled={busy} autoComplete="country-name" /></label>
        {fieldErrors.address && <span className="field-error">{fieldErrors.address}</span>}
      </fieldset>}
      <div><button className="primary" disabled={busy || !code.trim() || name.trim().length < 2}>{busy ? 'Saving…' : editing ? 'Save franchise' : 'Create franchise'}</button>{editing && <button type="button" className="secondary" disabled={busy} onClick={reset}>Cancel</button>}</div>
    </form>}
    <h3>Your franchises</h3>
    {!franchises.length ? <p>No franchises yet.</p> : <ul>{franchises.map(f => <li key={f.id}>
      <strong>{f.code || 'No code yet'}</strong> · {f.name}{addressLine(f.address) ? <><br /><small>{addressLine(f.address)}</small></> : null}{!f.code && <><br /><small>This franchise needs a code. Edit it to add one.</small></>}
      {canManage && <> <button className="secondary" disabled={busy} onClick={() => edit(f)}>Edit</button> <button className="secondary" disabled={busy} onClick={() => { setError(''); setConfirmTarget(f); }}>Delete</button></>}
    </li>)}</ul>}
    <ConfirmDialog isOpen={confirmTarget !== null} title="Delete franchise" entityName={confirmTarget ? `${confirmTarget.code} · ${confirmTarget.name}` : ''} entityType="Franchise" actionType="delete" isBusy={busy}
      explanation="An unused franchise is permanently removed along with its settings. A franchise with bookings, customers, invoices, attendance, photo uploads or role assignments cannot be deleted. This cannot be undone."
      impactNotice="Staff assigned to this franchise must be reassigned first." confirmLabel="Delete franchise" onConfirm={confirmDelete} onCancel={() => { if (!busy) setConfirmTarget(null); }} />
  </section>;
}
