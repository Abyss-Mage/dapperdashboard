'use client';

import { Suspense, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import AuthShell from '../AuthShell';

const json = { 'Content-Type': 'application/json' };

/* Forgot-password page: without a token it asks for an email address and sends a link; with the token from that link it
   lets the person choose a new password. The answer to the first step is the same whether or not the address has an account. */
function ResetPassword() {
  const token = useSearchParams().get('token') ?? '';
  const [email, setEmail] = useState(''), [password, setPassword] = useState(''), [again, setAgain] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState(''), [done, setDone] = useState(false);

  async function request(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError('');
    try {
      const r = await fetch('/api/v1/public/auth/password-reset/request', { method: 'POST', headers: json, body: JSON.stringify({ email }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? 'The request could not be sent.');
      setMessage(body.message ?? 'If that address has an account, a reset link is on its way.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The request could not be sent.'); }
    finally { setBusy(false); }
  }
  async function confirm(e: React.FormEvent) {
    e.preventDefault();
    if (password !== again) { setError('The two passwords are not the same.'); return; }
    setBusy(true); setError('');
    try {
      const r = await fetch('/api/v1/public/auth/password-reset/confirm', { method: 'POST', headers: json, body: JSON.stringify({ token, newPassword: password }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'The password could not be changed.');
      setDone(true);
    } catch (err) { setError(err instanceof Error ? err.message : 'The password could not be changed.'); }
    finally { setBusy(false); }
  }

  return (
    <AuthShell kind="Account help" title={token ? 'Choose a new password' : 'Forgot your password?'}>
        {done ? (
          <>
            <p>Your password was changed. You can sign in with it now.</p>
            {/* error404 #101: customers only see their own sign-in here, not the staff or owner links. */}
            <p><a className="primary" href="/account">Sign in to your account</a></p>
          </>
        ) : token ? (
          <form onSubmit={confirm} style={{ display: 'grid', gap: 14 }}>
            <label className="form-field">New password (at least 15 characters)<input type="password" autoComplete="new-password" required minLength={15} maxLength={1024} value={password} onChange={(e) => setPassword(e.target.value)} disabled={busy} /></label>
            <label className="form-field">New password again<input type="password" autoComplete="new-password" required minLength={15} maxLength={1024} value={again} onChange={(e) => setAgain(e.target.value)} disabled={busy} /></label>
            {error && <p role="alert" className="field-error">{error}</p>}
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Change password'}</button>
          </form>
        ) : message ? (
          <p role="status">{message}</p>
        ) : (
          <form onSubmit={request} style={{ display: 'grid', gap: 14 }}>
            <p>Enter the email you sign in with. We will email you a link to choose a new password.</p>
            <label className="form-field">Email<input type="email" autoComplete="email" required maxLength={254} value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} /></label>
            {error && <p role="alert" className="field-error">{error}</p>}
            <button className="primary" disabled={busy}>{busy ? 'Sending…' : 'Email me a reset link'}</button>
          </form>
        )}
    </AuthShell>
  );
}

export default function Page() {
  return <Suspense fallback={null}><ResetPassword /></Suspense>;
}
