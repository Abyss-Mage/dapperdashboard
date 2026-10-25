'use client';
import {useCallback, useEffect, useRef, useState} from 'react';

type Invitation = {id: string; email: string; status: string; deliveryState: string; expiresAt: string; roleName?: string; locationName?: string};
type Choice = {id: string; name: string};
export default function MemberInvitations({organizationId}: {organizationId: string}) {
  const [items, setItems] = useState<Invitation[]>([]);
  const [email, setEmail] = useState('');
  const [minutes, setMinutes] = useState('1440');
  const [roleId, setRoleId] = useState('');
  const [locationId, setLocationId] = useState('');
  const [roles, setRoles] = useState<Choice[]>([]);
  const [franchises, setFranchises] = useState<Choice[]>([]);
  const [available, setAvailable] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const errorRef = useRef<HTMLParagraphElement>(null);
  const base = `/api/v1/organizations/${organizationId}/access/invitations`;
  const load = useCallback(async () => {
    const response = await fetch(base, {cache: 'no-store'});
    if (response.status === 503) {setAvailable(false); setItems([]); return;}
    if (!response.ok) {setAvailable(false); throw new Error('Invitation access is unavailable or has been withdrawn.');}
    const body = await response.json(); setItems(body.items); setAvailable(true);
  }, [base]);
  useEffect(() => {setError(''); load().catch(failure => setError(failure.message));}, [load]);
  // The role and franchise the person gets when they accept, chosen before sending.
  useEffect(() => {
    let live = true;
    fetch(`/api/v1/organizations/${organizationId}/roles`, {cache: 'no-store'}).then(r => r.ok ? r.json() : []).then((list: Choice[]) => { if (live) setRoles(Array.isArray(list) ? list : []); }).catch(() => undefined);
    fetch(`/api/v1/organizations/${organizationId}/structure`, {cache: 'no-store'}).then(r => r.ok ? r.json() : null).then(b => { if (live && b?.franchises) { const list = b.franchises.map((f: Choice) => ({id: f.id, name: f.name})); setFranchises(list); if (list.length === 1) setLocationId(list[0].id); } }).catch(() => undefined);
    return () => { live = false; };
  }, [organizationId]);
  useEffect(() => {if (error && !busy) errorRef.current?.focus();}, [error, busy]);
  async function create(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setError(''); setNotice('');
    try {
      const response = await fetch(base, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({email: email.trim(), expiresInMinutes: Number(minutes), ...(roleId && locationId ? {roleId, locationId} : {})})});
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? 'Invitation could not be queued. Your input is preserved.');
      setEmail(''); setRoleId(''); setNotice(roleId ? 'Invitation queued. They get the chosen role when they accept. Email delivery has not yet been confirmed.' : 'Invitation queued. Email delivery has not yet been confirmed.'); await load();
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Invitation could not be queued.');}
    finally {setBusy(false);}
  }
  async function revoke(invitation: Invitation) {
    setBusy(true); setError(''); setNotice('');
    try {
      const response = await fetch(`${base}/${invitation.id}/revoke`, {method: 'POST'});
      if (!response.ok) {const body = await response.json(); throw new Error(body.error ?? 'Invitation could not be revoked.');}
      setNotice(`Invitation for ${invitation.email} revoked.`); await load();
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Invitation could not be revoked.');}
    finally {setBusy(false);}
  }
  return <section style={{marginTop: '1.5rem'}} aria-label="Member invitations">
    <h3>Invite a member</h3>
    <p>Choose their role and franchise now; they get it as soon as they accept and set their password. You can change roles later under Assign roles.</p>
    {error && <p ref={errorRef} tabIndex={-1} role="alert" className="field-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {!available ? <p>Invitations are currently unavailable because outbound email delivery (SMTP host, port, credentials, and approved sender address) has not been configured in the system environment.</p> : <>
      <form onSubmit={create} className="management-form">
        <label htmlFor="invitation-email" className="form-field">Recipient email<input id="invitation-email" required type="email" maxLength={254} value={email} onChange={event => setEmail(event.target.value)} disabled={busy} /></label>
        <label className="form-field">Role<select value={roleId} onChange={event => setRoleId(event.target.value)} disabled={busy}><option value="">Choose later</option>{roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}</select></label>
        <label className="form-field">Franchise<select value={locationId} onChange={event => setLocationId(event.target.value)} disabled={busy || !roleId} required={!!roleId}><option value="">Choose franchise</option>{franchises.map(f => <option key={f.id} value={f.id}>{f.name}</option>)}</select></label>
        <label htmlFor="invitation-minutes" className="form-field">Link expires in minutes (15–1440)<input id="invitation-minutes" required type="number" min={15} max={1440} step={1} value={minutes} onChange={event => setMinutes(event.target.value)} disabled={busy} /></label>
        <button className="primary" disabled={busy || (!!roleId && !locationId)}>{busy ? 'Saving…' : 'Send invitation'}</button>
      </form>
      <ul>{items.map(invitation => <li key={invitation.id} style={{paddingBlock: '.75rem', overflowWrap: 'anywhere'}}>
        <strong>{invitation.email}</strong>{invitation.roleName ? ` · ${invitation.roleName} at ${invitation.locationName}` : ''} · {invitation.status === 'pending' && new Date(invitation.expiresAt).getTime() <= Date.now() ? 'expired' : invitation.status} · Email: {invitation.deliveryState} · Expires {new Date(invitation.expiresAt).toLocaleString()}
        {invitation.status === 'pending' && <button className="secondary" disabled={busy} onClick={() => revoke(invitation)} style={{marginLeft: '.5rem'}}>Revoke invitation</button>}
      </li>)}</ul>
      {items.length === 100 && <p>Showing the most recent 100 invitations.</p>}
    </>}
  </section>;
}
