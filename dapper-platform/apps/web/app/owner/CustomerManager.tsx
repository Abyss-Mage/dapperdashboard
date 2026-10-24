'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

type Customer = { id: string; name: string; email: string; phone: string; createdAt: string; vehicles: number; bookings: number; jobs: number; invoices: number };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw Object.assign(new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The change could not be saved.')), { fields: body.errors as Record<string, string> | undefined });
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

/* Customers of the selected franchise: search, add, edit, delete. A customer with bookings, work orders or invoices cannot be deleted
   (their history must stay intact); edit their details instead. */
export default function CustomerManager({ base, canCreate, canDelete, onChanged }: { base: string; canCreate: boolean; canDelete: boolean; onChanged: () => void }) {
  const [items, setItems] = useState<Customer[] | null>(null);
  const [query, setQuery] = useState('');
  const [editing, setEditing] = useState<Customer | 'new' | null>(null);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [target, setTarget] = useState<Customer | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    try {
      setItems((await readJson(await fetch(`${base}/customer-directory`, { cache: 'no-store' }))).items);
      setError('');
    } catch (e) {
      setItems(null);
      setError(e instanceof Error ? e.message : 'Customers could not be loaded.');
    }
  }, [base]);
  useEffect(() => {
    setEditing(null);
    setNotice('');
    void load();
  }, [load]);

  function open(c: Customer | 'new') {
    setEditing(c);
    setError('');
    setNotice('');
    setFields({});
    setName(c === 'new' ? '' : c.name);
    setEmail(c === 'new' ? '' : c.email);
    setPhone(c === 'new' ? '' : c.phone);
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    setFields({});
    try {
      const was = editing;
      await readJson(await fetch(was === 'new' || !was ? `${base}/customers` : `${base}/customers/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', { name, email, phone })));
      setNotice(was === 'new' ? 'Customer added.' : 'Customer updated.');
      setEditing(null);
      await load();
      onChanged();
    } catch (err) {
      setFields((err as { fields?: Record<string, string> }).fields ?? {});
      setError(err instanceof Error ? err.message : 'The customer could not be saved.');
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!target) return;
    setBusy(true);
    setError('');
    try {
      await readJson(await fetch(`${base}/customers/${target.id}`, send('DELETE')));
      setNotice(`${target.name} deleted.`);
      setTarget(null);
      await load();
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The customer could not be deleted.');
      setTarget(null);
    } finally {
      setBusy(false);
    }
  }

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (items ?? []).filter((c) => !q || c.name.toLowerCase().includes(q) || c.email.toLowerCase().includes(q) || c.phone.toLowerCase().includes(q));
  }, [items, query]);

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Customers</h2>
          <p className={styles.personSub}>People this franchise serves. Adding a customer does not subscribe them to marketing.</p>
        </div>
        {canCreate && !editing && <button type="button" className="primary" onClick={() => open('new')}>New customer</button>}
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
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2}>{busy ? 'Saving…' : editing === 'new' ? 'Add customer' : 'Save changes'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </form>
      )}

      <div className={styles.rowBetween}>
        <span className={styles.badge}>{items ? `${items.length} customer${items.length === 1 ? '' : 's'}` : 'Loading'}</span>
        <input type="search" className={styles.searchBox} placeholder="Search name, email or phone" aria-label="Search customers" value={query} onChange={(e) => setQuery(e.target.value)} />
      </div>

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading customers" />}
      {items && visible.length === 0 && <p className={styles.status}>{items.length ? 'Nothing matches your search.' : 'No customers yet.'}</p>}
      <ul className={styles.people}>
        {visible.map((c) => {
          const history = c.bookings + c.jobs + c.invoices;
          return (
            <li key={c.id} className={styles.person} style={{ gridTemplateColumns: 'minmax(0,1.2fr) minmax(0,1.2fr) auto', alignItems: 'center' }}>
              <div className={styles.personMain}>
                <strong className={styles.personName}>{c.name}</strong>
                <span className={styles.personSub}>{c.email || 'No email'}{c.phone ? ` · ${c.phone}` : ''}</span>
              </div>
              <div className={styles.svcTags}>
                <span className={styles.chip}>{c.vehicles} vehicle{c.vehicles === 1 ? '' : 's'}</span>
                <span className={styles.chip}>{c.bookings} booking{c.bookings === 1 ? '' : 's'}</span>
                <span className={styles.chip}>{c.jobs} work order{c.jobs === 1 ? '' : 's'}</span>
                <span className={styles.chip}>{c.invoices} invoice{c.invoices === 1 ? '' : 's'}</span>
              </div>
              <div className={styles.svcActions}>
                {canCreate && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(c)}>Edit</button>}
                {canDelete && (
                  <button type="button" className="secondary" disabled={busy || !!editing || history > 0} title={history > 0 ? 'Has bookings, work orders or invoices, so it cannot be deleted. Edit the details instead.' : undefined} onClick={() => { setError(''); setTarget(c); }}>Delete</button>
                )}
              </div>
            </li>
          );
        })}
      </ul>

      <ConfirmDialog isOpen={target !== null} title="Delete customer" entityName={target?.name ?? ''} entityType="Customer" actionType="delete" isBusy={busy}
        explanation="The customer and their saved vehicles are removed for good. Customers with bookings, work orders or invoices cannot be deleted."
        impactNotice={target && target.vehicles > 0 ? `${target.vehicles} saved vehicle${target.vehicles === 1 ? '' : 's'} will be removed too.` : undefined}
        confirmLabel="Delete customer" onConfirm={remove} onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
