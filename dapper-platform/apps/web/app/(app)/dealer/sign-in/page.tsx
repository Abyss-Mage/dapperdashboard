'use client';

import { useEffect, useState } from 'react';
import styles from '../dealer.module.css';
import { API } from '../lib';

/* Sign in to the dealership portal with the email and password the business gave you. */
export default function DealerSignIn() {
  const [next, setNext] = useState<string | null>(null);
  const [expired, setExpired] = useState(false);
  useEffect(() => { const q = new URLSearchParams(window.location.search); setNext(q.get('next')); setExpired(q.get('expired') === '1'); }, []);
  const [form, setForm] = useState({ email: '', password: '' });
  const [busy, setBusy] = useState(false), [error, setError] = useState('');

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError('');
    try {
      const r = await fetch(`${API}/login`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'That did not work. Try again.');
      window.location.assign(next && next.startsWith('/dealer') ? next : '/dealer');
    } catch (err) { setError(err instanceof Error ? err.message : 'That did not work. Try again.'); setBusy(false); }
  }

  return (
    <div className={styles.signin}>
      <form className={styles.signinCard} onSubmit={submit}>
        <h1>Dealership account</h1>
        <p className={styles.note}>{expired ? 'That email link has expired. Sign in to see and pay your invoices.' : 'Sign in to see, download and pay your invoices.'}</p>
        {error && <p role="alert" className={styles.error}>{error}</p>}
        <label className={styles.field}>Email<input type="email" autoComplete="username" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required disabled={busy} /></label>
        <label className={styles.field}>Password<input type="password" autoComplete="current-password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required disabled={busy} /></label>
        <button className={styles.btnRed} disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
        <p className={styles.note}>No login yet? Ask the business to set one up for you, or use the link in your invoice email.</p>
      </form>
    </div>
  );
}
