'use client';

import { useCallback, useEffect, useState } from 'react';
import { Copy, ExternalLink } from 'lucide-react';
import styles from './owner.module.css';
import { askConfirm } from '../confirm';

type Person = { principalId: string; name: string; hasPin: boolean };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* The shop iPad set-up (error404 #20): the franchise's own shop username and password (they open only the worker list on the iPad), the
   link to open there, and each person's 6-digit PIN (set by the owner; only whether one is set is shown). */
export default function ShopClockSetup({ base, canManage }: { base: string; canManage: boolean }) {
  const link = typeof window === 'undefined' ? '' : `${window.location.origin}/workspace`;
  const [people, setPeople] = useState<Person[] | null>(null);
  const [username, setUsername] = useState<string | null>(null);
  const [login, setLogin] = useState({ username: '', password: '' });
  const [editingLogin, setEditingLogin] = useState(false);
  const [editing, setEditing] = useState(''), [pin, setPin] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const [staff, shop] = await Promise.all([fetch(`${base}/shop-clock/staff`, { cache: 'no-store' }).then(readJson), fetch(`${base}/shop-clock/login`, { cache: 'no-store' }).then(readJson)]);
    setPeople(staff.items ?? []); setUsername(shop.username ?? '');
  }, [base]);
  useEffect(() => { if (canManage) load().catch((e) => setError(e.message)); }, [load, canManage]);

  async function act(fn: () => Promise<string | void>) {
    setBusy(true); setError(''); setNotice('');
    try { const m = await fn(); if (m) setNotice(m); await load(); } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); } finally { setBusy(false); }
  }
  const saveLogin = () => act(async () => {
    await readJson(await fetch(`${base}/shop-clock/login`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(login) }));
    setEditingLogin(false); setLogin({ username: '', password: '' });
    return 'Shop login saved. Sign the iPad in with it; any iPad signed in with the old one has been signed out.';
  });
  const removeLogin = () => act(async () => {
    if (!(await askConfirm({ title: 'Remove the shop login?', message: 'The shop iPad is signed out and staff cannot clock in there until you set a new login.', confirmLabel: 'Remove login', tone: 'danger' }))) return;
    await readJson(await fetch(`${base}/shop-clock/login`, { method: 'DELETE' }));
    return 'Shop login removed and the iPad signed out.';
  });
  const save = (p: Person) => act(async () => {
    await readJson(await fetch(`${base}/shop-clock/staff/${p.principalId}/pin`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pin }) }));
    setEditing(''); setPin('');
    return `PIN saved for ${p.name}. Tell them in person; it is not shown again.`;
  });
  const clear = (p: Person) => act(async () => {
    await readJson(await fetch(`${base}/shop-clock/staff/${p.principalId}/pin`, { method: 'DELETE' }));
    return `${p.name} can no longer use the shop iPad until they get a new PIN.`;
  });

  return (
    <section className={styles.card} style={{ padding: 20 }}>
      <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Shop iPad</h2>
      <p className={styles.personSub}>One iPad in the shop for everyone. Sign in to the employee workspace on it with this franchise's shop login: it opens every workspace page for the franchise, with Clock in / out first. Staff tap their name and type their 6-digit PIN to clock in, take a break or clock out, or on the Availability page to set their availability, and they can open their own dashboard there. Done, or 2 minutes without a tap, signs them out.</p>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {canManage && username !== null && (
        <div className={styles.resource} style={{ margin: '14px 0' }}>
          <h3 style={{ margin: '0 0 6px' }}>Shop login</h3>
          {!editingLogin ? (
            <div className={styles.rowBetween}>
              <p className={styles.personSub} style={{ margin: 0 }}>{username ? <>Username <strong>{username}</strong>. The password is not shown again.</> : 'No shop login yet: the iPad cannot be signed in.'}</p>
              <div className={styles.enqAct}>
                <button type="button" className="secondary" disabled={busy} onClick={() => { setEditingLogin(true); setLogin({ username: username || '', password: '' }); setNotice(''); }}>{username ? 'Change login' : 'Set shop login'}</button>
                {username && <button type="button" className="secondary" disabled={busy} onClick={() => void removeLogin()}>Remove</button>}
              </div>
            </div>
          ) : (
            <form className={styles.inlineForm} onSubmit={(e) => { e.preventDefault(); void saveLogin(); }}>
              <label className="form-field">Username<input autoComplete="off" autoCapitalize="none" spellCheck={false} value={login.username} onChange={(e) => setLogin({ ...login, username: e.target.value.toLowerCase().replace(/[^a-z0-9._-]/g, '').slice(0, 40) })} placeholder="e.g. barrie-shop" minLength={3} required disabled={busy} /></label>
              <label className="form-field">Password (at least 10 characters)<input type="password" autoComplete="new-password" value={login.password} onChange={(e) => setLogin({ ...login, password: e.target.value })} minLength={10} maxLength={200} required disabled={busy} /></label>
              <div className={styles.enqAct} style={{ alignSelf: 'end' }}>
                <button className="primary" disabled={busy}>Save login</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => setEditingLogin(false)}>Cancel</button>
              </div>
            </form>
          )}
        </div>
      )}

      <div className={styles.clockLink}>
        <code>{link}</code>
        <button type="button" className="secondary" onClick={() => { void navigator.clipboard?.writeText(link).then(() => setNotice('Link copied.')); }}><Copy size={15} aria-hidden="true" /> Copy</button>
        <a className="secondary" href={link} target="_blank" rel="noreferrer"><ExternalLink size={15} aria-hidden="true" /> Open</a>
      </div>
      <p className={styles.personSub}>Open this link on the shop iPad, sign in there with the shop username and password, and add it to the Home Screen. Its clock page then shows your staff. After 5 wrong PINs for one person they wait 15 minutes. Owners and administrators cannot open their dashboard with a PIN; they use their own login.</p>

      {canManage && !people && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading staff" />}
      {people && people.length === 0 && <p className={styles.status}>No one records time at this franchise yet. Add staff under Employees first.</p>}
      {people && people.length > 0 && (
        <ul className={styles.upList}>
          {people.map((p) => (
            <li key={p.principalId} className={styles.upItem}>
              <div><strong>{p.name}</strong><p>{p.hasPin ? 'PIN set' : 'No PIN yet: not on the shop iPad'}</p></div>
              {editing === p.principalId ? (
                <form className={styles.enqAct} onSubmit={(e) => { e.preventDefault(); void save(p); }}>
                  <input aria-label={`New 6-digit PIN for ${p.name}`} inputMode="numeric" autoComplete="off" pattern="[0-9]{6}" maxLength={6} required value={pin} onChange={(e) => setPin(e.target.value.replace(/\D/g, '').slice(0, 6))} disabled={busy} style={{ width: 140, letterSpacing: '.3em', textAlign: 'center' }} placeholder="000000" />
                  <button className="primary" disabled={busy || pin.length !== 6}>Save</button>
                  <button type="button" className="secondary" disabled={busy} onClick={() => { setEditing(''); setPin(''); }}>Cancel</button>
                </form>
              ) : (
                <div className={styles.enqAct}>
                  <button type="button" className="secondary" disabled={busy} onClick={() => { setEditing(p.principalId); setPin(''); setNotice(''); }}>{p.hasPin ? 'Change PIN' : 'Set PIN'}</button>
                  {p.hasPin && <button type="button" className="secondary" disabled={busy} onClick={() => void clear(p)}>Remove PIN</button>}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
