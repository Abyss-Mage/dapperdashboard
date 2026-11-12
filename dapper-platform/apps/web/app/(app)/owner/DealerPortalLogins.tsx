'use client';

import { useCallback, useEffect, useState } from 'react';
import { Copy } from 'lucide-react';
import styles from './owner.module.css';
import { askConfirm } from '../confirm';

/* A dealership's portal logins (error404 #26, owner decision "login + email link"): the people at the dealership who can sign in at
   /dealer to see, download and pay their invoices, see payments and statements, and save cards. The link in each invoice email also
   opens the portal for 30 days without a password. */
type User = { id: string; name: string; email: string; createdAt: string };

async function read<T>(r: Response): Promise<T> {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'The change could not be saved.');
  return b as T;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

export default function DealerPortalLogins({ base, dealership }: { base: string; dealership: { id: string; accountName: string; contactName: string; contactEmail: string } }) {
  const url = `${base}/dealerships/${dealership.id}/portal-users`;
  const link = typeof window === 'undefined' ? '/dealer' : `${window.location.origin}/dealer`;
  const [users, setUsers] = useState<User[] | null>(null);
  const [form, setForm] = useState({ name: dealership.contactName, email: dealership.contactEmail, password: '' });
  const [reset, setReset] = useState(''), [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const load = useCallback(async () => setUsers((await read<{ items: User[] }>(await fetch(url, { cache: 'no-store' }))).items), [url]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function act(fn: () => Promise<string | void>) {
    setBusy(true); setError(''); setNotice('');
    try { const m = await fn(); if (m) setNotice(m); await load(); } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); } finally { setBusy(false); }
  }

  return (
    <div className={styles.resource} style={{ marginTop: 12 }}>
      <h4 style={{ margin: '0 0 6px', fontSize: 17 }}>Portal logins for {dealership.accountName}</h4>
      <p className={styles.personSub} style={{ margin: '0 0 10px' }}>They sign in at <a href={link} target="_blank" rel="noreferrer">{link}</a> <button type="button" className="secondary" style={{ minHeight: 30, padding: '0 8px' }} onClick={() => void navigator.clipboard?.writeText(link).then(() => setNotice('Link copied.'))} aria-label="Copy the portal link"><Copy size={14} aria-hidden="true" /></button>. Tell them their password yourself; it is not shown again.</p>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {users && users.length > 0 && (
        <ul className={styles.upList}>
          {users.map((u) => (
            <li key={u.id} className={styles.upItem}>
              <div><strong>{u.name}</strong><p>{u.email}</p></div>
              {reset === u.id ? (
                <form className={styles.enqAct} onSubmit={(e) => { e.preventDefault(); void act(async () => { await read(await fetch(`${url}/${u.id}/password`, send('PUT', { password }))); setReset(''); setPassword(''); return `New password saved for ${u.name}; they were signed out everywhere.`; }); }}>
                  <input type="password" autoComplete="new-password" aria-label={`New password for ${u.name}`} minLength={10} required value={password} onChange={(e) => setPassword(e.target.value)} placeholder="At least 10 characters" disabled={busy} />
                  <button className="primary" disabled={busy}>Save</button>
                  <button type="button" className="secondary" disabled={busy} onClick={() => setReset('')}>Cancel</button>
                </form>
              ) : (
                <div className={styles.enqAct}>
                  <button type="button" className="secondary" disabled={busy} onClick={() => { setReset(u.id); setPassword(''); }}>New password</button>
                  <button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => { if (!(await askConfirm({ title: `Remove ${u.name}'s login?`, message: 'They can no longer sign in to the dealership portal. Email links still open it.', confirmLabel: 'Remove login', tone: 'danger' }))) return; await read(await fetch(`${url}/${u.id}`, { method: 'DELETE' })); return 'Login removed.'; })}>Remove</button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      {users && users.length === 0 && <p className={styles.status}>No logins yet.</p>}
      <form className={styles.inlineForm} style={{ marginTop: 10, alignItems: 'flex-end' }} onSubmit={(e) => { e.preventDefault(); void act(async () => { await read(await fetch(url, send('POST', form))); setForm({ name: '', email: '', password: '' }); return 'Login added.'; }); }}>
        <label className="form-field">Name<input required minLength={2} maxLength={120} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} disabled={busy} /></label>
        <label className="form-field">Email<input required type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} disabled={busy} /></label>
        <label className="form-field">Password<input required type="password" autoComplete="new-password" minLength={10} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} placeholder="At least 10 characters" disabled={busy} /></label>
        <button className="primary" disabled={busy}>Add login</button>
      </form>
    </div>
  );
}
