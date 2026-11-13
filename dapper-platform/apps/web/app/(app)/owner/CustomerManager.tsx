'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import Select from '../GlassSelect';
import { DatePicker } from '../Pickers';

type Customer = { id: string; name: string; email: string; phone: string; createdAt: string; vehicles: number; bookings: number; jobs: number; invoices: number; address: string; services: string[]; statuses: string[]; bookingDays: string[] };
type Line = { id: string; title: string; status: string; at: string; cents?: number | null };
type Records = { vehicles: Line[]; bookings: Line[]; jobs: Line[]; invoices: Line[] };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw Object.assign(new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The change could not be saved.')), { fields: body.errors as Record<string, string> | undefined });
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
const when = (iso: string) => new Date(iso).toLocaleDateString('en-CA', { year: 'numeric', month: 'short', day: 'numeric' });
const money = (c: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);
const digits = (v: string) => v.replace(/\D/g, '');

/* Customers of the selected franchise: search, add, edit and delete. Each email and phone number belongs to one customer.
   The list stays short; a customer's vehicles, bookings, jobs and invoices open when you click their name. */
// canExport: only the organization's owners and administrators may take the customer list away as a file; franchise owners may not.
export default function CustomerManager({ base, canCreate, canDelete, canExport, onChanged }: { base: string; canCreate: boolean; canDelete: boolean; canExport: boolean; onChanged: () => void }) {
  const [items, setItems] = useState<Customer[] | null>(null);
  const [query, setQuery] = useState('');
  const [editing, setEditing] = useState<Customer | 'new' | null>(null);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [service, setService] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [target, setTarget] = useState<Customer | null>(null);
  const [open, setOpen] = useState<Customer | null>(null);
  const [records, setRecords] = useState<Records | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // Emails the customer a 72-hour link to choose their account password (error404 #85); it also works as a reset for an existing account.
  async function sendSetup(id: string) {
    setBusy(true); setNotice('');
    try {
      const r = await fetch(`${base}/customers/${id}/account-setup`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The email could not be sent.');
      setNotice(b.status === 'sent' ? `Account setup email sent to ${b.email}. The link works for 3 days.` : b.status === 'no_email' ? 'This customer has no email address. Add one with Edit details first.' : b.status === 'not_configured' ? 'Email sending is not set up, so nothing was sent.' : 'The email could not be sent. Check the email settings and try again.');
    } catch (e) { setNotice(e instanceof Error ? e.message : 'The email could not be sent.'); }
    finally { setBusy(false); }
  }

  const load = useCallback(async () => {
    try {
      setItems((await readJson(await fetch(`${base}/customer-directory`, { cache: 'no-store' }))).items);
      setError('');
    } catch (e) {
      setItems(null);
      setError(e instanceof Error ? e.message : 'Customers could not be loaded.');
    }
  }, [base]);
  useEffect(() => { setEditing(null); setOpen(null); setNotice(''); void load(); }, [load]);

  useEffect(() => {
    if (!open) { setRecords(null); return; }
    let live = true;
    setRecords(null);
    fetch(`${base}/customers/${open.id}/records`, { cache: 'no-store' }).then(readJson).then((b) => live && setRecords(b)).catch((e) => live && setError(e.message));
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(null); };
    window.addEventListener('keydown', onKey);
    return () => { live = false; window.removeEventListener('keydown', onKey); };
  }, [open, base]);

  function edit(c: Customer | 'new') {
    setEditing(c); setError(''); setNotice(''); setFields({});
    setName(c === 'new' ? '' : c.name); setEmail(c === 'new' ? '' : c.email); setPhone(c === 'new' ? '' : c.phone); setAddress(c === 'new' ? '' : c.address);
  }

  // Before saving, point out a customer who already has this email or phone (the server refuses duplicates too).
  const clash = useMemo(() => {
    const self = editing && editing !== 'new' ? editing.id : '';
    const e = email.trim().toLowerCase(), p = digits(phone);
    return (items ?? []).find((c) => c.id !== self && ((e && c.email.toLowerCase() === e) || (p.length >= 7 && digits(c.phone) === p)));
  }, [items, editing, email, phone]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setFields({});
    try {
      const was = editing;
      await readJson(await fetch(was === 'new' || !was ? `${base}/customers` : `${base}/customers/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', { name, email, phone, ...(was && was !== 'new' ? { address } : {}) })));
      setNotice(was === 'new' ? 'Customer added.' : 'Customer updated.');
      setEditing(null);
      await load();
      onChanged();
    } catch (err) {
      setFields((err as { fields?: Record<string, string> }).fields ?? {});
      setError(err instanceof Error ? err.message : 'The customer could not be saved.');
    } finally { setBusy(false); }
  }

  async function remove() {
    if (!target) return;
    setBusy(true); setError('');
    try {
      const r = await readJson(await fetch(`${base}/customers/${target.id}`, send('DELETE')));
      setNotice(r.removed ? `${target.name} was deleted from your customers. Their issued invoices and payments are kept for your tax records.` : `${target.name} and their records were deleted.`);
      setTarget(null); setOpen(null);
      await load();
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The customer could not be deleted.');
      setTarget(null);
    } finally { setBusy(false); }
  }

  const serviceChoices = useMemo(() => [...new Set((items ?? []).flatMap((c) => c.services))].sort((x, y) => x.localeCompare(y)), [items]);
  // Status here means the status of any of the customer's bookings; the dates and service narrow to bookings too.
  const STATUS_CHOICES: [string, string][] = [['', 'Any status'], ['confirmed', 'Approved'], ['completed', 'Completed'], ['cancelled', 'Cancelled']];
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase(), qd = digits(query);
    return (items ?? []).filter((c) =>
      (!q || c.name.toLowerCase().includes(q) || c.email.toLowerCase().includes(q) || c.phone.toLowerCase().includes(q) || (qd.length >= 3 && digits(c.phone).includes(qd))) &&
      (!service || c.services.includes(service)) &&
      (!statusFilter || (statusFilter === 'confirmed' ? c.statuses.some((s) => ['pending', 'awaiting_payment', 'confirmed', 'scheduled', 'in_progress'].includes(s)) : c.statuses.includes(statusFilter))) &&
      ((!from && !to) || c.bookingDays.some((d) => (!from || d >= from) && (!to || d <= to))));
  }, [items, query, service, statusFilter, from, to]);
  const filtered = !!(query || service || statusFilter || from || to);

  // CSV of what is on screen (respects the search). Cells that start like a formula are prefixed so spreadsheets do not run them.
  function exportCsv() {
    const lead = ['=', '+', '-', '@', String.fromCharCode(9), String.fromCharCode(13)];
    const cell = (v: string | number) => { let t = String(v ?? ''); if (lead.includes(t.charAt(0))) t = "'" + t; return '"' + t.replaceAll('"', '""') + '"'; };
    const rows = [['Name', 'Email', 'Phone', 'Address', 'Customer since', 'Vehicles', 'Bookings', 'Invoices'], ...visible.map((c) => [c.name, c.email, c.phone, c.address, c.createdAt.slice(0, 10), c.vehicles, c.bookings, c.invoices])];
    const blob = new Blob([String.fromCharCode(0xfeff) + rows.map((r) => r.map(cell).join(',')).join(String.fromCharCode(13, 10))], { type: 'text/csv;charset=utf-8' });
    const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = `customers-${new Date().toISOString().slice(0, 10)}.csv`; a.click(); URL.revokeObjectURL(a.href);
  }

  const section = (title: string, list: Line[], render: (l: Line) => string) => (
    <section style={{ marginTop: 18 }}>
      <h3 style={{ fontSize: 15, margin: '0 0 6px' }}>{title} <span className={styles.personSub}>({list.length})</span></h3>
      {list.length === 0 ? <p className={styles.personSub}>None.</p> : <ul className={styles.people} style={{ gap: 6 }}>{list.map((l) => <li key={l.id} className={styles.personSub} style={{ color: 'var(--text)' }}>{render(l)}</li>)}</ul>}
    </section>
  );

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Customers</h2>
        </div>
        <div className={styles.svcActions}>
          {canExport && <button type="button" className="secondary" disabled={!visible.length} onClick={exportCsv}>Export</button>}
          {canCreate && !editing && <button type="button" className="primary" onClick={() => edit('new')}>New customer</button>}
        </div>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'New customer' : `Edit ${editing.name}`}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Name<input required minLength={2} maxLength={100} autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} disabled={busy} aria-invalid={!!fields.name} />{fields.name && <span className="field-error">{fields.name}</span>}</label>
            <label className="form-field">Email (optional)<input type="email" maxLength={254} autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} aria-invalid={!!fields.email} />{fields.email && <span className="field-error">{fields.email}</span>}</label>
            <label className="form-field">Phone (optional)<input type="tel" maxLength={30} autoComplete="tel" value={phone} onChange={(e) => setPhone(e.target.value)} disabled={busy} aria-invalid={!!fields.phone} />{fields.phone && <span className="field-error">{fields.phone}</span>}</label>
          </div>
          {editing !== 'new' && <label className="form-field">Address (mobile service)<input maxLength={300} autoComplete="street-address" value={address} onChange={(e) => setAddress(e.target.value)} disabled={busy} /></label>}
          {clash && <p role="alert" className="field-error">{clash.name} already has this {clash.email.toLowerCase() === email.trim().toLowerCase() && email.trim() ? 'email' : 'phone number'}. Open that customer instead of adding a second one.</p>}
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2 || !!clash}>{busy ? 'Saving…' : editing === 'new' ? 'Add customer' : 'Save changes'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </form>
      )}

      <div className={styles.custFilters}>
        <input type="search" className={styles.searchBox} placeholder="Search name, phone or email" aria-label="Search customers" value={query} onChange={(e) => setQuery(e.target.value)} />
        <DatePicker value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} aria-label="Booked from" />
        <DatePicker value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} aria-label="Booked until" />
        <Select value={service} onChange={(e) => setService(e.target.value)} aria-label="Service"><option value="">Any service</option>{serviceChoices.map((s) => <option key={s} value={s}>{s}</option>)}</Select>
        <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} aria-label="Booking status">{STATUS_CHOICES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</Select>
        {filtered && <button type="button" className="secondary" onClick={() => { setQuery(''); setFrom(''); setTo(''); setService(''); setStatusFilter(''); }}>Clear filters</button>}
      </div>
      <div className={styles.rowBetween}>
        <span className={styles.badge}>{items ? (filtered ? `${visible.length} of ${items.length} customers` : `${items.length} customer${items.length === 1 ? '' : 's'}`) : 'Loading'}</span>
      </div>

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading customers" />}
      {items && visible.length === 0 && <p className={styles.status}>{items.length ? 'Nothing matches these filters.' : 'No customers yet.'}</p>}
      <ul className={styles.people}>
        {visible.map((c) => (
          <li key={c.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1fr) auto', alignItems: 'center' }}>
            <button type="button" className={styles.personButton} onClick={() => setOpen(c)} aria-label={`Open ${c.name}`}>
              <span className={styles.custRow}>
                <strong className={styles.custName}>{c.name}</strong>
                <span className={styles.custContact}>{c.phone || 'No phone'}</span>
                <span className={styles.custContact}>{c.email || 'No email'}</span>
                <span className={styles.custCount}>{c.bookings} booking{c.bookings === 1 ? '' : 's'}</span>
                {c.address && <span className={styles.custAddress}>{c.address}</span>}
              </span>
            </button>
            <div className={styles.svcActions}>
              {canCreate && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => edit(c)}>Edit</button>}
              {canDelete && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => { setError(''); setTarget(c); }}>Delete</button>}
            </div>
          </li>
        ))}
      </ul>

      {open && <>
        <button type="button" className={styles.drawerScrim} aria-label="Close customer details" onClick={() => setOpen(null)} />
        <aside data-motion="drawer" className={styles.drawer} role="dialog" aria-modal="true" aria-label={`${open.name}, customer details`}>
          <div className={styles.rowBetween}>
            <div>
              <h2 className={styles.setupTitle} style={{ fontSize: 22, margin: 0 }}>{open.name}</h2>
              <p className={styles.personSub}>{open.email || 'No email'}{open.phone ? ` · ${open.phone}` : ''} · customer since {when(open.createdAt)}</p>
            </div>
            <button type="button" className="secondary" onClick={() => setOpen(null)}>Close</button>
          </div>
          {!records && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading records" style={{ marginTop: 16 }} />}
          {records && <>
            {section('Vehicles', records.vehicles, (l) => `${l.title}${l.status ? ` · ${l.status}` : ''}`)}
            {section('Bookings', records.bookings, (l) => `${l.title} · ${when(l.at)} · ${l.status.replaceAll('_', ' ')}`)}
            {section('Jobs', records.jobs, (l) => `${l.title} · ${when(l.at)} · ${l.status === 'completed' ? 'finished' : l.status.replaceAll('_', ' ')}`)}
            {section('Invoices', records.invoices, (l) => `${l.title} · ${when(l.at)} · ${l.status.replaceAll('_', ' ')}${l.cents != null ? ` · ${money(l.cents)}` : ''}`)}
          </>}
          <div className={styles.rowBetween} style={{ marginTop: 24 }}>
            {canCreate && <button type="button" className="secondary" onClick={() => { const c = open; setOpen(null); edit(c); }}>Edit details</button>}
            {canCreate && open.email && <button type="button" className="secondary" disabled={busy} onClick={() => void sendSetup(open.id)}>Send account setup email</button>}
            {canDelete && <button type="button" className="secondary" onClick={() => setTarget(open)}>Delete customer</button>}
          </div>
        </aside>
      </>}

      <ConfirmDialog isOpen={target !== null} title="Delete customer" entityName={target?.name ?? ''} entityType="Customer" actionType="delete" isBusy={busy}
        explanation="The customer is deleted with their vehicles, bookings, jobs and draft invoices. If they have issued invoices or payments, those are kept for your tax records and the customer is removed from every list."
        confirmLabel="Delete customer" onConfirm={remove} onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
