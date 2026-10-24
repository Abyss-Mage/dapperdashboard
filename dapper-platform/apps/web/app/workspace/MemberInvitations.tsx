'use client';
import {useCallback, useEffect, useRef, useState} from 'react';

type Invitation = {id: string; email: string; status: string; deliveryState: string; expiresAt: string};
export default function MemberInvitations({organizationId}: {organizationId: string}) {
  const [items, setItems] = useState<Invitation[]>([]);
  const [email, setEmail] = useState('');
  const [minutes, setMinutes] = useState('');
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
  useEffect(() => {if (error && !busy) errorRef.current?.focus();}, [error, busy]);
  async function create(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setError(''); setNotice('');
    try {
      const response = await fetch(base, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({email: email.trim(), expiresInMinutes: Number(minutes)})});
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? 'Invitation could not be queued. Your input is preserved.');
      setEmail(''); setMinutes(''); setNotice('Invitation queued. Email delivery has not yet been confirmed.'); await load();
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
    <p>Members set their own password upon accepting. Acceptance adds organization membership; an administrator with role assignment authority can then assign one or more named roles (such as Detailer, Shop Lead, or Service Advisor) scoped to specific locations or franchises in Role Assignments below.</p>
    {error && <p ref={errorRef} tabIndex={-1} role="alert" className="field-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {!available ? <p>Invitations are currently unavailable because outbound email delivery (SMTP host, port, credentials, and approved sender address) has not been configured in the system environment.</p> : <>
      <form onSubmit={create} className="management-form">
        <label htmlFor="invitation-email" className="form-field">Recipient email<input id="invitation-email" required type="email" maxLength={254} value={email} onChange={event => setEmail(event.target.value)} disabled={busy} /></label>
        <label htmlFor="invitation-minutes" className="form-field">Link expires in minutes (15–1440)<input id="invitation-minutes" required type="number" min={15} max={1440} step={1} value={minutes} onChange={event => setMinutes(event.target.value)} disabled={busy} /></label>
        <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Queue invitation'}</button>
      </form>
      <ul>{items.map(invitation => <li key={invitation.id} style={{paddingBlock: '.75rem', overflowWrap: 'anywhere'}}>
        <strong>{invitation.email}</strong> · {invitation.status === 'pending' && new Date(invitation.expiresAt).getTime() <= Date.now() ? 'expired' : invitation.status} · Email: {invitation.deliveryState} · Expires {new Date(invitation.expiresAt).toLocaleString()}
        {invitation.status === 'pending' && <button className="secondary" disabled={busy} onClick={() => revoke(invitation)} style={{marginLeft: '.5rem'}}>Revoke invitation</button>}
      </li>)}</ul>
      {items.length === 100 && <p>Showing the most recent 100 invitations.</p>}
    </>}
  </section>;
}
