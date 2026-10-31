'use client';
import { KeyRound, ShieldCheck } from 'lucide-react';
import QRCode from 'qrcode';
import { useCallback, useEffect, useRef, useState } from 'react';
import glass from '../glass.module.css';

export type Profile = { fullName: string; phone: string; email: string; complete: boolean };
export type MfaState = { configured: boolean; enabled: boolean; setupStarted: boolean; required: boolean; recoveryRemaining: number };
const json = { 'Content-Type': 'application/json' };

async function call<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(url, init);
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw Object.assign(new Error(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'The request could not be completed.'), { fields: body.errors ?? {} });
  return body as T;
}

/* Change the sign-in email or password. Both need the current password; afterwards every session ends, so the page returns to sign-in. */
export function SignInDetails({ email }: { email?: string }) {
  const [mode, setMode] = useState<'' | 'email' | 'password'>('');
  const [current, setCurrent] = useState(''), [value, setValue] = useState(''), [again, setAgain] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const reset = () => { setCurrent(''); setValue(''); setAgain(''); setError(''); };
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (mode === 'password' && value !== again) { setError('The two new passwords are not the same.'); return; }
    setBusy(true); setError('');
    try {
      await call(mode === 'email' ? '/api/v1/account/email' : '/api/v1/account/password', { method: 'PUT', headers: json, body: JSON.stringify(mode === 'email' ? { password: current, email: value } : { current, newPassword: value }) });
      location.reload();
    } catch (err) { setError(err instanceof Error ? err.message : 'The change could not be saved.'); setBusy(false); }
  }
  if (!mode) return <div style={{ display: 'grid', gap: 10 }}>
    {email && <p className={glass.lede}>You sign in with <strong>{email}</strong>.</p>}
    <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}><button type="button" className="secondary" onClick={() => { reset(); setMode('email'); }}>Change email</button><button type="button" className="secondary" onClick={() => { reset(); setMode('password'); }}>Change password</button></div>
  </div>;
  return <form onSubmit={save} style={{ display: 'grid', gap: 14 }}>
    <label className={glass.field}>Current password<input type="password" autoComplete="current-password" required maxLength={1024} value={current} onChange={e => setCurrent(e.target.value)} disabled={busy} /></label>
    {mode === 'email'
      ? <label className={glass.field}>New email<input type="email" autoComplete="email" required maxLength={254} value={value} onChange={e => setValue(e.target.value)} disabled={busy} /></label>
      : <><label className={glass.field}>New password (at least 15 characters)<input type="password" autoComplete="new-password" required minLength={15} maxLength={1024} value={value} onChange={e => setValue(e.target.value)} disabled={busy} /></label>
        <label className={glass.field}>New password again<input type="password" autoComplete="new-password" required minLength={15} maxLength={1024} value={again} onChange={e => setAgain(e.target.value)} disabled={busy} /></label></>}
    {error && <p role="alert" className="field-error">{error}</p>}
    <p className={glass.lede}>After saving you will be signed out everywhere and asked to sign in again.</p>
    <div style={{ display: 'flex', gap: 10 }}><button className="primary" disabled={busy}>{busy ? 'Saving…' : mode === 'email' ? 'Change email' : 'Change password'}</button><button type="button" className="secondary" disabled={busy} onClick={() => setMode('')}>Cancel</button></div>
  </form>;
}

/* The person's own name and phone. Email is shown for reference (it is their sign-in). Phone is format-checked only. */
export function ProfileForm({ initial, onSaved }: { initial: Profile | null; onSaved: (p: Profile) => void }) {
  const [name, setName] = useState(initial?.fullName ?? ''), [phone, setPhone] = useState(initial?.phone ?? ''), [busy, setBusy] = useState(false), [error, setError] = useState(''), [saved, setSaved] = useState(false);
  useEffect(() => { setName(initial?.fullName ?? ''); setPhone(initial?.phone ?? ''); }, [initial?.fullName, initial?.phone]);
  async function save(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError(''); setSaved(false);
    try { const p = await call<Profile>('/api/v1/account/profile', { method: 'PUT', headers: json, body: JSON.stringify({ fullName: name, phone }) }); setSaved(true); onSaved(p); }
    catch (err) { setError(err instanceof Error ? err.message : 'Could not save your details.'); }
    finally { setBusy(false); }
  }
  return <form onSubmit={save} style={{ display: 'grid', gap: 14 }} noValidate>
    {initial?.email && <label className={glass.field}>Email (your sign-in)<input value={initial.email} readOnly aria-readonly="true" /></label>}
    <label className={glass.field}>Full name<input value={name} onChange={e => setName(e.target.value)} maxLength={120} autoComplete="name" required disabled={busy} /></label>
    <label className={glass.field}>Mobile phone<input value={phone} onChange={e => setPhone(e.target.value)} inputMode="tel" autoComplete="tel" placeholder="+1 416 555 0123" required disabled={busy} /><span className={glass.lede}>Include the country code. The format is checked; it is not verified by text message.</span></label>
    {error && <p className={glass.alert} role="alert">{error}</p>}{saved && !error && <p className={glass.notice} role="status">Your details are saved.</p>}
    <div className={glass.actions}><button className={glass.button} disabled={busy || name.trim().length < 2 || phone.trim().length < 7}>{busy ? 'Saving…' : 'Save my details'}</button></div>
  </form>;
}

/* Authenticator-app setup: scan (or type) the key, confirm with a code, then save the one-time recovery codes. */
export function MfaSetup({ onDone, required }: { onDone: () => void; required: boolean }) {
  const [step, setStep] = useState<'intro' | 'scan' | 'codes'>('intro'), [secret, setSecret] = useState(''), [qr, setQr] = useState(''), [code, setCode] = useState('');
  const [codes, setCodes] = useState<string[]>([]), [saved, setSaved] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const field = useRef<HTMLInputElement>(null);
  useEffect(() => { if (step === 'scan') field.current?.focus(); }, [step]);
  async function start() {
    setBusy(true); setError('');
    try { const r = await call<{ secret: string; uri: string }>('/api/v1/account/mfa/setup', { method: 'POST', headers: json, body: '{}' }); setSecret(r.secret); setQr(await QRCode.toDataURL(r.uri, { margin: 1, width: 224 })); setStep('scan'); }
    catch (err) { setError(err instanceof Error ? err.message : 'Setup could not start.'); }
    finally { setBusy(false); }
  }
  async function confirm(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError('');
    try { const r = await call<{ recoveryCodes: string[] }>('/api/v1/account/mfa/confirm', { method: 'POST', headers: json, body: JSON.stringify({ code: code.replace(/\s/g, '') }) }); setCodes(r.recoveryCodes); setStep('codes'); }
    catch (err) { setError(err instanceof Error ? err.message : 'That code did not work.'); }
    finally { setBusy(false); }
  }
  const download = () => { const url = URL.createObjectURL(new Blob([`Dapper Detailing recovery codes\nEach code works once. Keep them somewhere safe.\n\n${codes.join('\n')}\n`], { type: 'text/plain' })); const a = document.createElement('a'); a.href = url; a.download = 'dapper-recovery-codes.txt'; a.click(); URL.revokeObjectURL(url); };
  if (step === 'intro') return <div style={{ display: 'grid', gap: 14 }}>
    <p className={glass.lede}>{required ? 'Administrators must use two-factor authentication.' : 'Add a second step to your sign-in (recommended).'} You will need an authenticator app such as Google Authenticator, Microsoft Authenticator or 1Password.</p>
    {error && <p className={glass.alert} role="alert">{error}</p>}
    <div className={glass.actions}><button className={glass.button} onClick={start} disabled={busy}><KeyRound size={17} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />{busy ? 'Starting…' : 'Set up authenticator app'}</button></div>
  </div>;
  if (step === 'scan') return <form onSubmit={confirm} style={{ display: 'grid', gap: 14 }}>
    <p className={glass.lede}>1. Scan this code with your authenticator app, or type the key.<br />2. Enter the 6-digit code it shows.</p>
    <div style={{ display: 'flex', gap: 18, flexWrap: 'wrap', alignItems: 'center' }}>{qr && <img src={qr} width={224} height={224} alt="QR code to add Dapper to your authenticator app" style={{ background: '#fff', borderRadius: 14, padding: 6 }} />}
      <div><span className={glass.lede}>Setup key (if you cannot scan):</span><p style={{ fontFamily: 'ui-monospace, Consolas, monospace', fontWeight: 650, letterSpacing: '.08em', wordBreak: 'break-all', margin: '6px 0 0' }}>{secret.match(/.{1,4}/g)?.join(' ')}</p></div></div>
    <label className={glass.field}>6-digit code<input ref={field} value={code} onChange={e => setCode(e.target.value)} inputMode="numeric" autoComplete="one-time-code" maxLength={7} required disabled={busy} /></label>
    {error && <p className={glass.alert} role="alert">{error}</p>}
    <div className={glass.actions}><button className={glass.button} disabled={busy || code.replace(/\s/g, '').length !== 6}>{busy ? 'Checking…' : 'Turn on two-factor'}</button></div>
  </form>;
  return <div style={{ display: 'grid', gap: 14 }}>
    <p className={glass.notice} role="status"><ShieldCheck size={16} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Two-factor authentication is on.</p>
    <p className={glass.lede}>Save these recovery codes now. Each works once if you lose your phone. They will not be shown again.</p>
    <ul className={glass.list} style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))' }}>{codes.map(c => <li key={c} className={glass.row} style={{ fontFamily: 'ui-monospace, Consolas, monospace', fontWeight: 650 }}>{c}</li>)}</ul>
    <div className={glass.actions}><button className={glass.ghost} onClick={() => void navigator.clipboard?.writeText(codes.join('\n'))}>Copy codes</button><button className={glass.ghost} onClick={download}>Download codes</button></div>
    <label className={glass.switch}><span>I have saved my recovery codes.</span><input type="checkbox" checked={saved} onChange={e => setSaved(e.target.checked)} /></label>
    <div className={glass.actions}><button className={glass.button} disabled={!saved} onClick={onDone}>Continue</button></div>
  </div>;
}

/* Account-page card: shows MFA state; employees may turn it on or off, administrators cannot turn it off. */
export function MfaManager({ onChange }: { onChange?: () => void }) {
  const [state, setState] = useState<MfaState | null>(null), [setup, setSetup] = useState(false), [code, setCode] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const load = useCallback(() => call<MfaState>('/api/v1/account/security', { cache: 'no-store' }).then(setState).catch(e => setError(e.message)), []);
  useEffect(() => { void load(); }, [load]);
  async function disable(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError('');
    try { await call('/api/v1/account/mfa/disable', { method: 'POST', headers: json, body: JSON.stringify({ code: code.trim() }) }); setCode(''); await load(); onChange?.(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Could not turn two-factor off.'); }
    finally { setBusy(false); }
  }
  if (!state) return <p className={glass.lede}>{error || 'Loading security settings…'}</p>;
  if (!state.configured) return <p className={glass.lede}>Two-factor authentication is not available on this server yet.</p>;
  if (setup) return <MfaSetup required={state.required} onDone={() => { setSetup(false); void load(); onChange?.(); }} />;
  return <div style={{ display: 'grid', gap: 12 }}>
    <p className={glass.lede}>Authenticator app: <strong>{state.enabled ? 'on' : 'off'}</strong>{state.required ? ' (required for administrators)' : ' (optional for employees)'}{state.enabled ? ` · ${state.recoveryRemaining} recovery codes left` : ''}</p>
    {error && <p className={glass.alert} role="alert">{error}</p>}
    {!state.enabled && <div className={glass.actions}><button className={glass.button} onClick={() => setSetup(true)}>Set up authenticator app</button></div>}
    {state.enabled && !state.required && <form onSubmit={disable} style={{ display: 'grid', gap: 10 }}><label className={glass.field}>To turn it off, enter a current code or a recovery code<input value={code} onChange={e => setCode(e.target.value)} autoComplete="one-time-code" disabled={busy} /></label><div className={glass.actions}><button className={glass.ghost} disabled={busy || code.trim().length < 6}>Turn off two-factor</button></div></form>}
  </div>;
}
