'use client';
import {useCallback, useEffect, useRef, useState} from 'react';

type AccountManagement = {
  customers: {id: string; name: string; email: string}[];
  members: {principalId: string; email: string; active: boolean}[];
  links: {customerId: string; principalId: string; customerName: string; email: string; active: boolean}[];
  limit: number;
};
export default function CustomerAccountManager({base, revision = 0}: {base: string; revision?: number}) {
  const [data, setData] = useState<AccountManagement | null>(null);
  const [customer, setCustomer] = useState('');
  const [principal, setPrincipal] = useState('');
  const [reviewed, setReviewed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const errorRef = useRef<HTMLParagraphElement>(null);
  const load = useCallback(async () => {
    const response = await fetch(`${base}/customer-accounts`, {cache: 'no-store'});
    if (!response.ok) {setData(null); throw new Error('Customer account management is unavailable or access was withdrawn.');}
    const body: AccountManagement = await response.json(); setData(body);
  }, [base]);
  useEffect(() => {setError(''); load().catch(failure => setError(failure.message));}, [load, revision]);
  useEffect(() => {if (error && !busy) errorRef.current?.focus();}, [error, busy]);
  const selectedCustomer = data?.customers.find(item => item.id === customer);
  const selectedMember = data?.members.find(item => item.principalId === principal);
  async function change(customerId: string, principalId: string, enabled: boolean) {
    setBusy(true); setError(''); setNotice('');
    try {
      const response = await fetch(`${base}/customer-accounts`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({customerId, principalId, enabled, ownershipReviewed: enabled && reviewed})});
      if (!response.ok) {const body = await response.json(); throw new Error(body.error ?? 'Customer access could not be changed. Your selection is preserved.');}
      setReviewed(false); setNotice(enabled ? 'Reviewed customer account access granted.' : 'Customer account access revoked.'); await load();
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Customer access could not be changed.');}
    finally {setBusy(false);}
  }
  return <section className="panel detail-card" aria-label="Customer account ownership">
    <h2>Customer account ownership</h2>
    <p>Connect a verified enrolled organization member to a customer at this location. Matching email addresses never grant access automatically.</p>
    {error && <p ref={errorRef} tabIndex={-1} role="alert" className="field-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {data && <>
      <form className="management-form" onSubmit={event => {event.preventDefault(); if (selectedCustomer && selectedMember?.active && reviewed) change(customer, principal, true);}}>
        <label className="form-field">Customer record<select required value={customer} disabled={busy} onChange={event => {setCustomer(event.target.value); setReviewed(false);}}><option value="">Choose customer</option>{data.customers.map(item => <option value={item.id} key={item.id}>{item.name}{item.email ? ` · ${item.email}` : ''}</option>)}</select></label>
        <label className="form-field">Enrolled account<select required value={principal} disabled={busy} onChange={event => {setPrincipal(event.target.value); setReviewed(false);}}><option value="">Choose account</option>{data.members.map(item => <option value={item.principalId} key={item.principalId} disabled={!item.active}>{item.email}{item.active ? '' : ' (inactive)'}</option>)}</select></label>
        <p>Selected customer: <strong>{selectedCustomer?.name ?? 'None'}</strong>. Selected account: <strong>{selectedMember?.email ?? 'None'}</strong>.</p>
        <label><input type="checkbox" required checked={reviewed} disabled={busy || !selectedCustomer || !selectedMember?.active} onChange={event => setReviewed(event.target.checked)} /> I verified this account is authorized to access the selected customer record.</label>
        <button className="primary" disabled={busy || !selectedCustomer || !selectedMember?.active || !reviewed}>{busy ? 'Saving…' : 'Grant reviewed customer access'}</button>
      </form>
      <h3>Existing customer access</h3>
      {!data.links.length && <p>No customer account links at this location.</p>}
      <ul>{data.links.map(link => <li key={`${link.customerId}:${link.principalId}`} style={{paddingBlock: '.75rem', overflowWrap: 'anywhere'}}>
        {link.customerName} → {link.email || 'Previously enrolled account'}{link.active ? '' : ' (inactive)'} <button type="button" className="secondary" disabled={busy} onClick={() => change(link.customerId, link.principalId, false)}>Revoke customer access</button>
      </li>)}</ul>
      {(data.customers.length === data.limit || data.members.length === data.limit || data.links.length === data.limit) && <p>Each list shows up to {data.limit} records.</p>}
    </>}
  </section>;
}
