'use client';
import { Check } from 'lucide-react';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import glass from '../glass.module.css';
import { MfaManager, ProfileForm, type MfaState, type Profile } from './AccountSecurity';
import DocumentsCard from './EmployeeDocuments';

export type Onboarding = { profile: Profile; mfa: MfaState; assigned: boolean; documentsTotal: number; documentsPending: number; documentsUnavailable: number; complete: boolean };

function Step({ n, ok, title, children }: { n: number; ok: boolean; title: string; children: ReactNode }) {
  return <section className={glass.card} style={{ background: 'var(--surface)', WebkitBackdropFilter: 'none', backdropFilter: 'none' }}>
    <div className={glass.head} style={{ justifyContent: 'flex-start' }}>
      <span aria-hidden="true" style={{ flex: 'none', width: 30, height: 30, borderRadius: '50%', display: 'grid', placeItems: 'center', fontSize: '.875rem', fontWeight: 700, background: ok ? 'var(--success)' : 'var(--surface-alt)', color: ok ? '#fff' : 'var(--text-muted)' }}>{ok ? <Check size={17} /> : n}</span>
      <h2 className={glass.title}>{title}</h2>
    </div>
    {children}
  </section>;
}

/* First-run checklist shown before the workspace until: details saved, MFA on (administrators; optional for
   employees), and every published document read and acknowledged. */
export default function OnboardingGate({ timezone, onChanged }: { timezone: string; onChanged: () => void }) {
  const [status, setStatus] = useState<Onboarding | null>(null), [error, setError] = useState('');
  const load = useCallback(async () => {
    try { const r = await fetch('/api/v1/account/onboarding', { cache: 'no-store' }); const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Your setup status could not be loaded.'); setStatus(b); setError(''); }
    catch (e) { setError(e instanceof Error ? e.message : 'Your setup status could not be loaded.'); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  if (!status) return <p className={glass.lede} role="status">{error || 'Preparing your setup…'}</p>;
  const mfaDone = status.mfa.enabled || (!status.mfa.required && status.mfa.configured === false);
  const docsDone = status.assigned && status.documentsPending === 0;
  return <div style={{ display: 'grid', gap: 18, width: '100%', maxWidth: 640, margin: '0 auto' }}>
    <div style={{ display: 'grid', gap: 8, marginBottom: 6 }}><h1 style={{ margin: 0 }}>Welcome. Let&rsquo;s finish setting up.</h1><p className={glass.lede}>A few quick steps before you start. You can come back to your details any time from your account.</p></div>
    {error && <p className={glass.alert} role="alert">{error}</p>}
    <Step n={1} ok={status.profile.complete} title="Your details">
      <ProfileForm initial={status.profile} onSaved={() => void load()} />
    </Step>
    <Step n={2} ok={mfaDone} title={`Two-factor authentication ${status.mfa.required ? '(required)' : '(optional)'}`}>
      {status.mfa.enabled ? <p className={glass.lede}>Your authenticator app is on.</p> : <MfaManager onChange={() => void load()} />}
    </Step>
    <Step n={3} ok={docsDone} title="Read and acknowledge your documents">
      {!status.assigned ? <p className={glass.lede}>SOP & Compliance documents appear here once your administrator assigns you to a franchise. You can finish steps 1 and 2 now and come back.</p> : <>
        <p className={glass.lede}>{status.documentsPending === 0 ? 'All documents are acknowledged.' : `${status.documentsPending} of ${status.documentsTotal} still need your acknowledgment. Open each PDF, read it, then acknowledge.`}{status.documentsUnavailable > 0 ? ` ${status.documentsUnavailable} could not be loaded; ask your administrator.` : ''}</p>
        <DocumentsCard timezone={timezone} onChanged={() => void load()} />
      </>}
    </Step>
    <div className={glass.actions}><button className={glass.button} disabled={!status.complete} onClick={onChanged}>{status.complete ? 'Continue to your workspace' : 'Complete the steps above to continue'}</button></div>
  </div>;
}
