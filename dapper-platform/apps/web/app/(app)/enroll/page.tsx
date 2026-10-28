'use client';
import {useEffect, useRef, useState} from 'react';

export default function EnrollmentPage() {
  const [token, setToken] = useState('');
  const [existing, setExisting] = useState(false);
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [busy, setBusy] = useState(false);
  const [complete, setComplete] = useState(false);
  const [error, setError] = useState('');
  const errorRef = useRef<HTMLParagraphElement>(null);
  useEffect(() => {
    function readInvitation() {
      const value = new URLSearchParams(window.location.hash.slice(1)).get('invitation') ?? '';
      if (value) {
        setToken(/^[A-Za-z0-9_-]{43}$/.test(value) ? value : '');
        setPassword(''); setConfirmation(''); setExisting(false); setComplete(false); setError('');
      }
      // Keep the bearer token out of subsequent URLs/referrers and browser history.
      window.history.replaceState(null, '', window.location.pathname);
    }
    readInvitation();
    window.addEventListener('hashchange', readInvitation);
    return () => window.removeEventListener('hashchange', readInvitation);
  }, []);
  useEffect(() => {if (error && !busy) errorRef.current?.focus();}, [error, busy]);
  async function accept(event: React.FormEvent) {
    event.preventDefault(); setError('');
    if (!existing && (password !== confirmation || password.length < 15 || new TextEncoder().encode(password).length > 1024)) {setError('Use a password of at least 15 characters within the maximum length, and enter it identically twice.'); return;}
    setBusy(true);
    try {
      const response = await fetch('/api/v1/auth/enroll', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({token, password: existing ? '' : password})});
      if (!response.ok) {const body = await response.json(); throw new Error(body.error ?? 'Invitation could not be accepted.');}
      setToken(''); setPassword(''); setConfirmation(''); setComplete(true);
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Invitation could not be accepted.');}
    finally {setBusy(false);}
  }
  return <main style={{maxWidth: '640px', padding: '2rem 1rem'}}>
    <section className="panel detail-card">
      <h1>Join your Dapper organization</h1>
      {complete ? <><p role="status">Organization membership accepted. Operational permissions are assigned separately by your administrator.</p><a className="primary" href="/workspace">Continue to sign in</a></> : !token ? <p>Open the complete invitation link from your email. If the link has expired or was already used, ask your administrator for a new invitation.</p> : <>
        <p>This invitation adds organization membership. It does not grant access to bookings, customer records or financial operations.</p>
        <p>If another account is signed in, <a href="/workspace" target="_blank" rel="noreferrer">manage or sign out of that session in another tab</a> before creating a new account.</p>
        {error && <p className="field-error" role="alert" tabIndex={-1} ref={errorRef}>{error}</p>}
        <form onSubmit={accept} className="management-form">
          <label style={{display: 'block'}}><input type="checkbox" checked={existing} disabled={busy} onChange={event => {setExisting(event.target.checked); setPassword(''); setConfirmation('');}} /> I already have a Dapper account</label>
          {existing ? <p><a href="/workspace" target="_blank" rel="noreferrer">Sign in to the invited existing account in another tab</a>, then return here and accept. Your current password will remain unchanged.</p> : <>
            <label className="form-field">Choose your password<input type="password" autoComplete="new-password" required minLength={15} maxLength={1024} disabled={busy} value={password} onChange={event => setPassword(event.target.value)} /></label>
            <label className="form-field">Confirm password<input type="password" autoComplete="new-password" required minLength={15} maxLength={1024} disabled={busy} value={confirmation} onChange={event => setConfirmation(event.target.value)} /></label>
          </>}
          <button className="primary" disabled={busy}>{busy ? 'Accepting…' : 'Accept organization invitation'}</button>
        </form>
      </>}
    </section>
  </main>;
}
