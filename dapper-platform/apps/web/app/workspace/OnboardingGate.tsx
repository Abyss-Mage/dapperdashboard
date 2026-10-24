'use client';
import { Check, Circle } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';
import { MfaManager, ProfileForm, type MfaState, type Profile } from './AccountSecurity';
import DocumentsCard from './EmployeeDocuments';

export type Onboarding = { profile: Profile; mfa: MfaState; assigned: boolean; documentsTotal: number; documentsPending: number; documentsUnavailable: number; complete: boolean };

/* First-run checklist shown before the workspace until: details saved, MFA on (administrators; optional for
   employees), and every published document read and acknowledged. */
export default function OnboardingGate({ timezone, onChanged }: { timezone: string; onChanged: () => void }) {
  const [status, setStatus] = useState<Onboarding | null>(null), [error, setError] = useState('');
  const load = useCallback(async () => {
    try { const r = await fetch('/api/v1/account/onboarding', { cache: 'no-store' }); const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Your setup status could not be loaded.'); setStatus(b); setError(''); }
    catch (e) { setError(e instanceof Error ? e.message : 'Your setup status could not be loaded.'); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const done = (ok: boolean) => ok ? <Check size={18} aria-hidden="true" style={{ color: 'var(--success)' }} /> : <Circle size={18} aria-hidden="true" style={{ color: 'var(--text-muted)' }} />;
  if (!status) return <p className={glass.lede} role="status">{error || 'Preparing your setup…'}</p>;
  const mfaDone = status.mfa.enabled || (!status.mfa.required && status.mfa.configured === false);
  const docsDone = status.assigned && status.documentsPending === 0;
  return <div style={{ display: 'grid', gap: 22, maxWidth: 760 }}>
    <div><h1 style={{ margin: 0 }}>Welcome. Let&rsquo;s finish setting up.</h1><p className={glass.lede}>A few quick steps before you start. You can come back to your details any time from your account.</p></div>
    {error && <p className={glass.alert} role="alert">{error}</p>}
    <section className={glass.stage}><div className={glass.card}>
      <div className={glass.head}><h2 className={glass.title}>{done(status.profile.complete)} 1. Your details</h2></div>
      <ProfileForm initial={status.profile} onSaved={() => void load()} />
    </div></section>
    <section className={glass.stage}><div className={glass.card}>
      <div className={glass.head}><h2 className={glass.title}>{done(mfaDone)} 2. Two-factor authentication {status.mfa.required ? '(required)' : '(optional)'}</h2></div>
      {status.mfa.enabled ? <p className={glass.lede}>Your authenticator app is on.</p> : <MfaManager onChange={() => void load()} />}
    </div></section>
    <section className={glass.stage}><div className={glass.card}>
      <div className={glass.head}><h2 className={glass.title}>{done(docsDone)} 3. Read and acknowledge your documents</h2></div>
      {!status.assigned ? <p className={glass.lede}>Documents appear here once your administrator assigns you to a franchise. You can finish steps 1 and 2 now and come back.</p> : <>
        <p className={glass.lede}>{status.documentsPending === 0 ? 'All documents are acknowledged.' : `${status.documentsPending} of ${status.documentsTotal} still need your acknowledgment. Open each PDF, read it, then acknowledge.`}{status.documentsUnavailable > 0 ? ` ${status.documentsUnavailable} could not be loaded; ask your administrator.` : ''}</p>
        <DocumentsCard timezone={timezone} onChanged={() => void load()} />
      </>}
    </div></section>
    <div className={glass.actions}><button className={glass.button} disabled={!status.complete} onClick={onChanged}>{status.complete ? 'Continue to your workspace' : 'Complete the steps above to continue'}</button></div>
  </div>;
}
