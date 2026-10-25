'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import NumberField from '../NumberField';

/* Partner dealerships of a franchise: the Owner / Super Admin adds a dealership and its contact, and can switch one off.
   Dealership invoices can be sent to Invoice Ninja (connection settings below; sending is on the Invoices page). */

type Dealer = { id: string; customerId: string; accountName: string; accountCode: string; accountType: string; contactName: string; contactEmail: string; contactPhone: string; billingTermsDays: number; taxExempt: boolean; active: boolean; notes?: string };
type Customer = { id: string; name: string; email: string; phone: string };
type Ninja = { connected: boolean; baseUrl: string; version: number; updatedAt?: string };
const blank = { accountName: '', accountCode: '', contactName: '', contactEmail: '', contactPhone: '', billingTermsDays: 30, taxExempt: false, notes: '' };
const digits = (v: string) => v.replace(/\D/g, '');

async function read<T>(r: Response): Promise<T> {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? (Object.values(b.errors ?? {}).join('. ') || 'The change could not be saved.'));
  return b as T;
}
const send = (method: string, body: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export default function Dealerships({ organizationId, locationId, canManage, canConfigure }: { organizationId: string; locationId: string; canManage: boolean; canConfigure: boolean }) {
  const base = `/api/v1/organizations/${organizationId}/locations/${locationId}`;
  const [items, setItems] = useState<Dealer[] | null>(null);
  const [form, setForm] = useState<typeof blank & { id?: string; active?: boolean }>(blank);
  const [editing, setEditing] = useState(false);
  const [ninja, setNinja] = useState<Ninja | null>(null);
  const [ninjaUrl, setNinjaUrl] = useState('');
  const [ninjaToken, setNinjaToken] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const b = await read<{ items: Dealer[] }>(await fetch(`${base}/dealerships`, { cache: 'no-store' }));
    setItems((b.items ?? []).filter((d) => d.accountType === 'dealership'));
    if (canConfigure) { const n = await read<Ninja>(await fetch(`/api/v1/organizations/${organizationId}/invoice-ninja`, { cache: 'no-store' })); setNinja(n); setNinjaUrl(n.baseUrl); }
  }, [base, organizationId, canConfigure]);
  useEffect(() => { setItems(null); setError(''); setNotice(''); setEditing(false); load().catch((e) => setError(e.message)); }, [load]);

  const set = <K extends keyof typeof blank>(k: K, v: (typeof blank)[K]) => setForm((f) => ({ ...f, [k]: v }));

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      if (form.id) {
        await read(await fetch(`${base}/dealerships/${form.id}`, send('PUT', { accountName: form.accountName, contactName: form.contactName, contactEmail: form.contactEmail, contactPhone: form.contactPhone, billingTermsDays: Number(form.billingTermsDays), taxExempt: form.taxExempt, active: form.active ?? true, notes: form.notes })));
        setNotice(`${form.accountName} updated.`);
      } else {
        // The dealership is also a customer (for bookings and invoices): reuse the one with this email or phone, else add it.
        const list = (await read<{ items: Customer[] }>(await fetch(`${base}/customers`, { cache: 'no-store' }))).items;
        let customer = list.find((c) => (form.contactEmail && c.email.toLowerCase() === form.contactEmail.trim().toLowerCase()) || (digits(form.contactPhone).length >= 7 && digits(c.phone) === digits(form.contactPhone)));
        if (!customer) customer = await read<Customer>(await fetch(`${base}/customers`, send('POST', { name: form.accountName, email: form.contactEmail, phone: form.contactPhone })));
        await read(await fetch(`${base}/dealerships`, send('POST', { customerId: customer.id, accountName: form.accountName, accountCode: form.accountCode.toUpperCase().trim(), accountType: 'dealership', contactName: form.contactName, contactEmail: form.contactEmail, contactPhone: form.contactPhone, billingTermsDays: Number(form.billingTermsDays), taxExempt: form.taxExempt, notes: form.notes })));
        setNotice(`${form.accountName} added.`);
      }
      setEditing(false); setForm(blank);
      await load();
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  async function toggle(d: Dealer) {
    setBusy(true); setError(''); setNotice('');
    try {
      await read(await fetch(`${base}/dealerships/${d.id}`, send('PUT', { accountName: d.accountName, contactName: d.contactName, contactEmail: d.contactEmail, contactPhone: d.contactPhone, billingTermsDays: d.billingTermsDays, taxExempt: d.taxExempt, active: !d.active, notes: d.notes ?? '' })));
      setNotice(`${d.accountName} ${d.active ? 'switched off' : 'switched on'}.`);
      await load();
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  async function saveNinja(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const n = await read<Ninja>(await fetch(`/api/v1/organizations/${organizationId}/invoice-ninja`, send('PUT', { baseUrl: ninjaUrl, token: ninjaToken, version: ninja?.connected ? ninja.version : 0 })));
      setNinja(n); setNinjaToken(''); setNotice('Invoice Ninja connected. Dealership invoices can now be sent from the Invoices page.');
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  async function disconnectNinja() {
    setBusy(true); setError(''); setNotice('');
    try { const r = await fetch(`/api/v1/organizations/${organizationId}/invoice-ninja`, { method: 'DELETE' }); if (!r.ok && r.status !== 204) await read(r); setNinja({ connected: false, baseUrl: '', version: 0 }); setNotice('Invoice Ninja disconnected.'); }
    catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Dealerships</h2>
          <p className={styles.personSub}>Car dealerships you work with. Each one is also a customer, so its bookings and invoices stay together.</p>
        </div>
        {canManage && !editing && <button type="button" className="primary" onClick={() => { setForm(blank); setEditing(true); setError(''); }}>Add a dealership</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{form.id ? `Edit ${form.accountName}` : 'New dealership'}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Dealership name<input required minLength={2} maxLength={160} value={form.accountName} onChange={(e) => set('accountName', e.target.value)} disabled={busy} /></label>
            {!form.id && <label className="form-field">Short code<input required minLength={2} maxLength={20} value={form.accountCode} onChange={(e) => set('accountCode', e.target.value)} placeholder="e.g. PTBOHONDA" disabled={busy} /></label>}
            <label className="form-field">Contact person<input required minLength={2} maxLength={120} value={form.contactName} onChange={(e) => set('contactName', e.target.value)} disabled={busy} /></label>
            <label className="form-field">Contact email<input required type="email" maxLength={254} value={form.contactEmail} onChange={(e) => set('contactEmail', e.target.value)} disabled={busy} /></label>
            <label className="form-field">Contact phone<input type="tel" maxLength={30} value={form.contactPhone} onChange={(e) => set('contactPhone', e.target.value)} disabled={busy} /></label>
            <label className="form-field">Pay within (days)<NumberField min={0} max={365} value={form.billingTermsDays} onChange={(e) => set('billingTermsDays', Number(e.target.value))} disabled={busy} /></label>
          </div>
          <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}><input type="checkbox" checked={form.taxExempt} onChange={(e) => set('taxExempt', e.target.checked)} disabled={busy} /><span>Tax exempt</span></label>
          <label className="form-field">Notes (optional)<textarea maxLength={2000} value={form.notes} onChange={(e) => set('notes', e.target.value)} disabled={busy} /></label>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : form.id ? 'Save changes' : 'Add dealership'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(false)}>Cancel</button>
          </div>
        </form>
      )}

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading dealerships" />}
      {items && items.length === 0 && <p className={styles.status}>No dealerships yet.</p>}
      <ul className={styles.people}>
        {items?.map((d) => (
          <li key={d.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1fr) auto', alignItems: 'center' }}>
            <div className={styles.personMain}>
              <strong className={styles.personName}>{d.accountName} <span className={styles.personSub}>({d.accountCode})</span></strong>
              <span className={styles.personSub}>{d.contactName} · {d.contactEmail}{d.contactPhone ? ` · ${d.contactPhone}` : ''} · pay within {d.billingTermsDays} days{d.taxExempt ? ' · tax exempt' : ''}{d.active ? '' : ' · switched off'}</span>
            </div>
            {canManage && <div className={styles.svcActions}>
              <button type="button" className="secondary" disabled={busy || editing} onClick={() => { setForm({ ...blank, ...d, notes: d.notes ?? '' }); setEditing(true); setError(''); }}>Edit</button>
              <button type="button" className="secondary" disabled={busy || editing} onClick={() => void toggle(d)}>{d.active ? 'Switch off' : 'Switch on'}</button>
            </div>}
          </li>
        ))}
      </ul>

      {canConfigure && ninja && (
        <section className={styles.card} style={{ padding: 20 }}>
          <h3 style={{ marginTop: 0 }}>Invoice Ninja (dealership invoices)</h3>
          <p className={styles.personSub}>Send dealership invoices to your Invoice Ninja account. Dapper adds the dealership as a client there and copies the invoice lines. Create an API token in Invoice Ninja under Settings → Account Management → Integrations → API tokens.</p>
          {ninja.connected && <p className={styles.status}>Connected to {ninja.baseUrl}{ninja.updatedAt ? ` since ${new Date(ninja.updatedAt).toLocaleDateString('en-CA')}` : ''}.</p>}
          <form onSubmit={saveNinja} className={styles.inlineForm}>
            <label className="form-field">Invoice Ninja address<input required type="url" maxLength={200} placeholder="https://invoicing.co" value={ninjaUrl} onChange={(e) => setNinjaUrl(e.target.value)} disabled={busy} /></label>
            <label className="form-field">API token<input required type="password" autoComplete="off" maxLength={500} value={ninjaToken} onChange={(e) => setNinjaToken(e.target.value)} placeholder={ninja.connected ? 'Enter a new token to replace it' : ''} disabled={busy} /></label>
            <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <button className="primary" disabled={busy || !ninjaUrl || !ninjaToken}>{busy ? 'Checking…' : ninja.connected ? 'Replace token' : 'Connect'}</button>
              {ninja.connected && <button type="button" className="secondary" disabled={busy} onClick={() => void disconnectNinja()}>Disconnect</button>}
            </div>
          </form>
        </section>
      )}
    </div>
  );
}
