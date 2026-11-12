'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import NumberField from '../NumberField';
import DealerPortalLogins from './DealerPortalLogins';

/* Partner dealerships of a franchise: the Owner / Super Admin adds a dealership with its per-car rate and its contact people,
   and can switch one off. Each dealership is also a customer, so its bookings and invoices stay together. */

type Contact = { id?: string; name: string; email: string; phone: string };
type Dealer = { id: string; customerId: string; accountName: string; accountCode: string; accountType: string; active: boolean; notes?: string; perCarRateCents: number | null; contacts: Contact[]; contactName: string; contactEmail: string; contactPhone: string; address?: string };
type Customer = { id: string; name: string; email: string; phone: string };
type Form = { id?: string; accountName: string; accountCode: string; perCarRate: string; address: string; notes: string; active: boolean; contacts: Contact[] };
const blankContact: Contact = { name: '', email: '', phone: '' };
const blank: Form = { accountName: '', accountCode: '', perCarRate: '', address: '', notes: '', active: true, contacts: [{ ...blankContact }] };
const digits = (v: string) => v.replace(/\D/g, '');
const money = (c: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);

async function read<T>(r: Response): Promise<T> {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? (Object.values(b.errors ?? {}).join('. ') || 'The change could not be saved.'));
  return b as T;
}
const send = (method: string, body: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export default function Dealerships({ organizationId, locationId, canManage }: { organizationId: string; locationId: string; canManage: boolean; canConfigure?: boolean }) {
  const base = `/api/v1/organizations/${organizationId}/locations/${locationId}`;
  const [items, setItems] = useState<Dealer[] | null>(null);
  const [form, setForm] = useState<Form>(blank);
  const [editing, setEditing] = useState(false);
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [portalFor, setPortalFor] = useState('');

  const load = useCallback(async () => {
    const b = await read<{ items: Dealer[] }>(await fetch(`${base}/dealerships`, { cache: 'no-store' }));
    setItems((b.items ?? []).filter((d) => d.accountType === 'dealership'));
  }, [base]);
  useEffect(() => { setItems(null); setError(''); setNotice(''); setEditing(false); load().catch((e) => setError(e.message)); }, [load]);

  const setContact = (i: number, k: keyof Contact, v: string) => setForm((f) => ({ ...f, contacts: f.contacts.map((c, ci) => (ci === i ? { ...c, [k]: v } : c)) }));
  const rateCents = form.perCarRate.trim() === '' ? null : Math.round(Number(form.perCarRate) * 100);
  const rateBad = form.perCarRate.trim() !== '' && !(Number(form.perCarRate) >= 0);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    const contacts = form.contacts.map((c) => ({ name: c.name.trim(), email: c.email.trim(), phone: c.phone.trim() })).filter((c) => c.name || c.email || c.phone);
    try {
      if (form.id) {
        await read(await fetch(`${base}/dealerships/${form.id}`, send('PUT', { accountName: form.accountName, active: form.active, address: form.address, notes: form.notes, perCarRateCents: rateCents, contacts })));
        setNotice(`${form.accountName} updated.`);
      } else {
        // The dealership is also a customer (for bookings and invoices): reuse the one with this email or phone, else add it.
        const primary = contacts[0];
        const list = (await read<{ items: Customer[] }>(await fetch(`${base}/customers`, { cache: 'no-store' }))).items;
        let customer = list.find((c) => (primary?.email && c.email.toLowerCase() === primary.email.toLowerCase()) || (digits(primary?.phone ?? '').length >= 7 && digits(c.phone) === digits(primary.phone)));
        if (!customer) customer = await read<Customer>(await fetch(`${base}/customers`, send('POST', { name: form.accountName, email: primary?.email ?? '', phone: primary?.phone ?? '' })));
        await read(await fetch(`${base}/dealerships`, send('POST', { customerId: customer.id, accountName: form.accountName, accountCode: form.accountCode.toUpperCase().trim(), accountType: 'dealership', address: form.address, notes: form.notes, perCarRateCents: rateCents, contacts })));
        setNotice(`${form.accountName} added.`);
      }
      setEditing(false); setForm(blank);
      await load();
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  async function toggle(d: Dealer) {
    setBusy(true); setError(''); setNotice('');
    try {
      await read(await fetch(`${base}/dealerships/${d.id}`, send('PUT', { accountName: d.accountName, active: !d.active, address: d.address ?? '', notes: d.notes ?? '', perCarRateCents: d.perCarRateCents, contacts: d.contacts.map(({ name, email, phone }) => ({ name, email, phone })) })));
      setNotice(`${d.accountName} ${d.active ? 'switched off' : 'switched on'}.`);
      await load();
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  function edit(d: Dealer) {
    setForm({ id: d.id, accountName: d.accountName, accountCode: d.accountCode, perCarRate: d.perCarRateCents == null ? '' : (d.perCarRateCents / 100).toFixed(2), address: d.address ?? '', notes: d.notes ?? '', active: d.active, contacts: d.contacts.length ? d.contacts.map((c) => ({ ...c })) : [{ name: d.contactName, email: d.contactEmail, phone: d.contactPhone }] });
    setEditing(true); setError('');
  }

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (items ?? []).filter((d) => !q || d.accountName.toLowerCase().includes(q) || d.accountCode.toLowerCase().includes(q) || d.contacts.some((c) => [c.name, c.email, c.phone].some((v) => v.toLowerCase().includes(q))));
  }, [items, query]);
  const primaryOk = form.contacts[0]?.name.trim().length >= 2 && /^\S+@\S+\.\S+$/.test(form.contacts[0]?.email.trim() ?? '');

  return (
    <div className={styles.stack}>
      <div className={styles.dealTop}>
        <p className={styles.dealIntro}>Car dealerships you work with, each with an agreed price per car and the people to contact. Each one is also a customer, so its bookings and invoices stay together.</p>
        {canManage && !editing && <button type="button" className="primary" onClick={() => { setForm(blank); setEditing(true); setError(''); }}>Add a dealership</button>}
      </div>
      {items && (
        <div className={styles.dealStats} aria-label="Dealerships at a glance">
          <div><b>{items.length}</b><span>Dealerships</span></div>
          <div><b>{items.filter((d) => d.active).length}</b><span>Switched on</span></div>
          <div><b>{items.filter((d) => d.perCarRateCents != null).length}</b><span>With a per-car rate</span></div>
        </div>
      )}
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{form.id ? `Edit ${form.accountName}` : 'New dealership'}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Dealership name<input required minLength={2} maxLength={160} value={form.accountName} onChange={(e) => setForm({ ...form, accountName: e.target.value })} disabled={busy} /></label>
            {!form.id && <label className="form-field">Short code<input required minLength={2} maxLength={20} value={form.accountCode} onChange={(e) => setForm({ ...form, accountCode: e.target.value })} placeholder="e.g. PTBOHONDA" disabled={busy} /></label>}
            <label className="form-field">Rate per car (CAD, before tax)
              <NumberField inputMode="decimal" min="0" step="0.01" placeholder="Not set" value={form.perCarRate} onChange={(e) => setForm({ ...form, perCarRate: e.target.value })} disabled={busy} />
            </label>
          </div>
          <fieldset style={{ border: '1px solid var(--border)', borderRadius: 14, padding: 14, margin: 0, display: 'grid', gap: 10 }}>
            <legend style={{ padding: '0 6px', fontWeight: 700 }}>Contact people</legend>
            {form.contacts.map((c, i) => (
              <div key={i} className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
                <label className="form-field">{i === 0 ? 'Main contact name' : 'Name'}<input required={i === 0} minLength={2} maxLength={120} value={c.name} onChange={(e) => setContact(i, 'name', e.target.value)} disabled={busy} /></label>
                <label className="form-field">Email{i === 0 ? '' : ' (optional)'}<input required={i === 0} type="email" maxLength={254} value={c.email} onChange={(e) => setContact(i, 'email', e.target.value)} disabled={busy} /></label>
                <label className="form-field">Phone (optional)<input type="tel" maxLength={40} value={c.phone} onChange={(e) => setContact(i, 'phone', e.target.value)} disabled={busy} /></label>
                {i > 0 && <button type="button" className="secondary" disabled={busy} onClick={() => setForm({ ...form, contacts: form.contacts.filter((_, ci) => ci !== i) })} aria-label={`Remove contact ${i + 1}`}>Remove</button>}
              </div>
            ))}
            {form.contacts.length < 10 && <div><button type="button" className="secondary" disabled={busy} onClick={() => setForm({ ...form, contacts: [...form.contacts, { ...blankContact }] })}>Add another contact person</button></div>}
          </fieldset>
          <label className="form-field">Address (printed on its invoices and statements)<textarea rows={2} maxLength={300} value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} placeholder="e.g. Chemong Rd 1370, Peterborough Ontario K9H 0E7, Canada" disabled={busy} /></label>
          <label className="form-field">Notes (optional)<textarea maxLength={2000} value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} disabled={busy} /></label>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || form.accountName.trim().length < 2 || !primaryOk || rateBad}>{busy ? 'Saving…' : form.id ? 'Save changes' : 'Add dealership'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(false)}>Cancel</button>
          </div>
        </form>
      )}

      <div className={styles.dealBar}>
        <input type="search" className={styles.searchBox} placeholder="Search name, code or contact" aria-label="Search dealerships" value={query} onChange={(e) => setQuery(e.target.value)} />
      </div>
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading dealerships" />}
      {items && visible.length === 0 && <p className={styles.status}>{items.length ? 'Nothing matches your search.' : 'No dealerships yet.'}</p>}
      <ul className={styles.dealGrid}>
        {visible.map((d) => (
          <li key={d.id} className={styles.dealCard} data-off={d.active ? undefined : 'true'}>
            <div className={styles.dealHead}>
              <span className={styles.dealMark} aria-hidden="true">{d.accountName.trim().charAt(0).toUpperCase()}</span>
              <div>
                <strong>{d.accountName}</strong>
                <span className={styles.dealCode}>{d.accountCode}</span>
              </div>
              <span className={styles.dealState} data-on={d.active ? 'true' : undefined}>{d.active ? 'On' : 'Off'}</span>
            </div>
            <p className={styles.dealRate}>{d.perCarRateCents != null ? <><b>{money(d.perCarRateCents)}</b><span>per car, before tax</span></> : <span>No per-car rate set</span>}</p>
            {d.address && <p className={styles.dealRate} style={{ whiteSpace: 'pre-line' }}><span>{d.address}</span></p>}
            <ul className={styles.dealPeople}>
              {(d.contacts.length ? d.contacts : [{ name: d.contactName, email: d.contactEmail, phone: d.contactPhone }]).filter((c) => c.name || c.email || c.phone).map((c, i) => (
                <li key={i}>
                  <span>{c.name}</span>
                  {c.email && <a href={'mailto:' + c.email}>{c.email}</a>}
                  {c.phone && <a href={'tel:' + c.phone}>{c.phone}</a>}
                </li>
              ))}
            </ul>
            {canManage && <div className={styles.dealActions}>
              <button type="button" className="secondary" disabled={busy || editing} onClick={() => edit(d)}>Edit</button>
              <button type="button" className="secondary" disabled={busy || editing} onClick={() => void toggle(d)}>{d.active ? 'Switch off' : 'Switch on'}</button>
              <button type="button" className="secondary" aria-expanded={portalFor === d.id} onClick={() => setPortalFor(portalFor === d.id ? '' : d.id)}>Portal logins</button>
            </div>}
            {canManage && portalFor === d.id && <DealerPortalLogins base={base} dealership={d} />}
          </li>
        ))}
      </ul>
    </div>
  );
}
