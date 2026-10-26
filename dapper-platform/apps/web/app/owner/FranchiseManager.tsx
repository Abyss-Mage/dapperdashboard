'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

type Address = { street: string; city: string; region: string; postalCode: string; country: string };
type Contact = { id?: string; name: string; email: string; phone: string };
type Franchise = { id: string; franchiseId: string | null; code: string; name: string; address: Address; contacts?: Contact[] };
const blankContact: Contact = { name: '', email: '', phone: '' };
const addressComplete = (a: Address) => !!(a.street.trim() && a.city.trim() && a.region.trim() && a.postalCode.trim() && a.country.trim());
const blank: Address = { street: '', city: '', region: '', postalCode: '', country: '' };
const line = (a: Address) => [a.street, a.city, a.region, a.postalCode, a.country].filter(Boolean).join(', ');

/* A franchise is one site with a unique code, a full address and its contact people. Staff, bookings, scheduling and invoices belong to it. */
export default function FranchiseManager({ organization, selectedId, onSaved }: { organization: { id: string; name: string; actions: string[] }; selectedId?: string; onSaved: () => Promise<void> }) {
  const base = `/api/v1/organizations/${organization.id}`;
  const canManage = organization.actions.includes('structure.manage');
  const [items, setItems] = useState<Franchise[] | null>(null);
  const [editing, setEditing] = useState<Franchise | 'new' | null>(null);
  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [address, setAddress] = useState<Address>(blank);
  const [contacts, setContacts] = useState<Contact[]>([{ ...blankContact }]);
  const [query, setQuery] = useState('');
  const [target, setTarget] = useState<Franchise | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    try {
      const r = await fetch(`${base}/structure`, { cache: 'no-store' });
      if (!r.ok) throw new Error('Franchises could not be loaded.');
      setItems((await r.json()).franchises);
    } catch (e) {
      setItems(null);
      setError(e instanceof Error ? e.message : 'Franchises could not be loaded.');
    }
  }, [base]);
  useEffect(() => {
    void load();
  }, [load]);

  function open(next: Franchise | 'new') {
    setEditing(next);
    setError('');
    setNotice('');
    setFields({});
    setCode(next === 'new' ? '' : next.code);
    setName(next === 'new' ? '' : next.name);
    setAddress(next === 'new' ? blank : next.address);
    setContacts(next !== 'new' && next.contacts?.length ? next.contacts.map((c) => ({ ...c })) : [{ ...blankContact }]);
  }
  const setField = (k: keyof Address) => (e: React.ChangeEvent<HTMLInputElement>) => setAddress({ ...address, [k]: e.target.value });

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    setFields({});
    try {
      const isNew = editing === 'new';
      const r = await fetch(isNew ? `${base}/franchises` : `${base}/franchises/${(editing as Franchise).id}`, { method: isNew ? 'POST' : 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code, name, address, contacts: contacts.filter((c) => c.name.trim() || c.email.trim() || c.phone.trim()) }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) {
        setFields(body.errors ?? {});
        setError(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The franchise could not be saved.'));
        return;
      }
      setNotice(isNew ? 'Franchise created.' : 'Franchise updated.');
      setEditing(null);
      await load();
      await onSaved();
    } catch {
      setError('Could not save the franchise. Your input is kept.');
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!target) return;
    setBusy(true);
    setError('');
    try {
      const r = await fetch(`${base}/franchises/${target.id}`, { method: 'DELETE' });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The franchise could not be deleted.');
      setNotice(`${target.name} deleted.`);
      setTarget(null);
      await load();
      await onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The franchise could not be deleted.');
      setTarget(null);
    } finally {
      setBusy(false);
    }
  }

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (items ?? []).filter((f) => !q || f.name.toLowerCase().includes(q) || f.code.toLowerCase().includes(q) || line(f.address).toLowerCase().includes(q));
  }, [items, query]);

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <p className={styles.personSub}>Each franchise is one site, identified by its code. Partner dealerships are separate accounts, not franchises.</p>
        {canManage && !editing && <button type="button" className="primary" onClick={() => open('new')}>New franchise</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'New franchise' : `Edit ${editing.code || editing.name}`}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Code (2–12 letters, numbers or hyphens)
              <input required maxLength={12} value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} disabled={busy} placeholder="e.g. PTBO" autoComplete="off" aria-invalid={!!fields.code} />
              {fields.code && <span className="field-error">{fields.code}</span>}
            </label>
            <label className="form-field" style={{ flexBasis: 280 }}>Name
              <input required minLength={2} maxLength={120} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} aria-invalid={!!fields.name} />
              {fields.name && <span className="field-error">{fields.name}</span>}
            </label>
          </div>
          <fieldset style={{ border: '1px solid var(--border)', borderRadius: 14, padding: 14, margin: 0 }}>
            <legend style={{ padding: '0 6px', fontWeight: 700 }}>Address (required)</legend>
            <div className={styles.inlineForm}>
              <label className="form-field" style={{ flexBasis: 300 }}>Street<input required maxLength={200} value={address.street} onChange={setField('street')} disabled={busy} autoComplete="street-address" /></label>
              <label className="form-field">City<input required maxLength={100} value={address.city} onChange={setField('city')} disabled={busy} autoComplete="address-level2" /></label>
              <label className="form-field">Province / state<input required maxLength={100} value={address.region} onChange={setField('region')} disabled={busy} autoComplete="address-level1" /></label>
              <label className="form-field">Postal / ZIP<input required maxLength={20} value={address.postalCode} onChange={setField('postalCode')} disabled={busy} autoComplete="postal-code" /></label>
              <label className="form-field">Country<input required maxLength={100} value={address.country} onChange={setField('country')} disabled={busy} autoComplete="country-name" /></label>
              {fields.address && <span className="field-error">{fields.address}</span>}
            </div>
          </fieldset>
          <fieldset style={{ border: '1px solid var(--border)', borderRadius: 14, padding: 14, margin: 0, display: 'grid', gap: 10 }}>
            <legend style={{ padding: '0 6px', fontWeight: 700 }}>Contact people</legend>
            {contacts.map((c, i) => (
              <div key={i} className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
                <label className="form-field">Name<input maxLength={120} value={c.name} onChange={(e) => setContacts(contacts.map((x, xi) => (xi === i ? { ...x, name: e.target.value } : x)))} disabled={busy} /></label>
                <label className="form-field">Email<input type="email" maxLength={254} value={c.email} onChange={(e) => setContacts(contacts.map((x, xi) => (xi === i ? { ...x, email: e.target.value } : x)))} disabled={busy} /></label>
                <label className="form-field">Phone<input type="tel" maxLength={40} value={c.phone} onChange={(e) => setContacts(contacts.map((x, xi) => (xi === i ? { ...x, phone: e.target.value } : x)))} disabled={busy} /></label>
                {contacts.length > 1 && <button type="button" className="secondary" disabled={busy} onClick={() => setContacts(contacts.filter((_, xi) => xi !== i))} aria-label={`Remove contact ${i + 1}`}>Remove</button>}
              </div>
            ))}
            {contacts.length < 10 && <div><button type="button" className="secondary" disabled={busy} onClick={() => setContacts([...contacts, { ...blankContact }])}>Add another contact person</button></div>}
          </fieldset>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || !code.trim() || name.trim().length < 2 || !addressComplete(address)}>{busy ? 'Saving…' : editing === 'new' ? 'Create franchise' : 'Save franchise'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </form>
      )}

      <div className={styles.rowBetween}>
        <span className={styles.badge}>{items ? `${items.length} franchise${items.length === 1 ? '' : 's'}` : 'Loading'}</span>
        <input type="search" className={styles.searchBox} placeholder="Search by code, name or address" aria-label="Search franchises" value={query} onChange={(e) => setQuery(e.target.value)} />
      </div>

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading franchises" />}
      {items && visible.length === 0 && <p className={styles.status}>{items.length ? 'Nothing matches your search.' : 'No franchises yet. Create the first one.'}</p>}
      <div className={styles.franchiseGrid}>
        {visible.map((f) => (
          <section key={f.id} className={styles.resource}>
            <div className={styles.resourceHead}>
              <div>
                <span className={styles.codeTag}>{f.code || 'NO CODE'}</span>
                <h3 style={{ marginTop: 8 }}>{f.name}</h3>
              </div>
              {selectedId === f.id && <span className={`${styles.badge} ${styles.badge_complete}`}>Selected</span>}
            </div>
            <p className={styles.personSub}>{line(f.address) || 'No address added: edit this franchise to add one.'}</p>
            {(f.contacts?.length ?? 0) > 0 && <p className={styles.personSub}>{f.contacts!.map((c) => [c.name, c.email, c.phone].filter(Boolean).join(' · ')).join('  |  ')}</p>}
            {!f.code && <p className={styles.personSub}>This franchise needs a code. Edit it to add one.</p>}
            {canManage && (
              <div className={styles.rowBetween}>
                <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(f)}>Edit</button>
                <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => { setError(''); setTarget(f); }}>Delete</button>
              </div>
            )}
          </section>
        ))}
      </div>

      <ConfirmDialog isOpen={target !== null} title="Delete franchise" entityName={target ? `${target.code} · ${target.name}` : ''} entityType="Franchise" actionType="delete" isBusy={busy}
        explanation="An unused franchise is permanently removed along with its settings. A franchise with bookings, customers, invoices, attendance, photo uploads or role assignments cannot be deleted. This cannot be undone."
        impactNotice="Staff assigned to this franchise must be reassigned first." confirmLabel="Delete franchise" onConfirm={remove} onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
