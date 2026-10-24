'use client';
import { Landmark } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';

type Info = { configured: boolean; credentialsConfigured?: boolean; applicationId?: string; binding?: { version: number; environment: 'sandbox' | 'production'; merchantId: string; squareLocationId: string; reviewedAt: string } };
const json = { 'Content-Type': 'application/json' };
const referenceFor = (env: string) => (env === 'production' ? 'production-default' : 'sandbox-default');

export default function SquareMerchantPanel({ organizationId, locationId, canManage }: { organizationId: string; locationId: string; canManage: boolean }) {
  const base = `/api/v1/organizations/${organizationId}/locations/${locationId}/square/binding`;
  const [info, setInfo] = useState<Info | null>(null), [env, setEnv] = useState<'sandbox' | 'production'>('sandbox'), [merchant, setMerchant] = useState(''), [squareLocation, setSquareLocation] = useState('');
  const [reviewed, setReviewed] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const load = useCallback(async () => {
    const r = await fetch(base, { cache: 'no-store' }), b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Square settings are unavailable or access has changed.');
    setInfo(b as Info);
  }, [base]);
  useEffect(() => { setInfo(null); setError(''); setNotice(''); setReviewed(false); load().catch(e => setError(e.message)); }, [load]);

  async function save(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(base, { method: 'PUT', headers: json, body: JSON.stringify({ environment: env, merchantId: merchant.trim(), squareLocationId: squareLocation.trim(), credentialReference: referenceFor(env), expectedVersion: info?.binding?.version ?? 0 }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The Square merchant could not be saved.');
      setReviewed(false); setMerchant(''); setSquareLocation(''); await load();
      setNotice('Saved. Square confirmed this merchant and location for CAD card processing.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The Square merchant could not be saved.'); }
    finally { setBusy(false); }
  }
  const b = info?.binding;
  return <section className={glass.stage} aria-labelledby="sq-merchant-title"><div className={glass.card}>
    <div className={glass.head}><h2 id="sq-merchant-title" className={glass.title}><Landmark size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Square merchant</h2>
      <span className={`${glass.pill} ${info?.configured ? glass.pillOk : ''}`}>{info ? (info.configured ? 'Connected' : 'Not set up') : 'Loading'}</span></div>
    <p className={glass.lede}>Card payments for this location are charged to exactly one reviewed Square merchant and location. Dapper asks Square to confirm them before saving. Every change is kept as a new version.</p>
    {error && <p className={glass.alert} role="alert">{error}</p>}{notice && <p className={glass.notice} role="status">{notice}</p>}
    {b && <ul className={glass.list}><li className={glass.row}><span className={glass.name}>{b.environment === 'sandbox' ? 'Sandbox (test cards only)' : 'Production (real cards)'}</span><span className={`${glass.pill} ${b.environment === 'production' ? glass.pillWarn : ''}`}>Version {b.version}</span>
      <span className={glass.meta}>Merchant {b.merchantId} · Location {b.squareLocationId} · reviewed {new Date(b.reviewedAt).toLocaleDateString()}</span></li></ul>}
    {info && !info.credentialsConfigured && <p className={`${glass.notice} ${glass.warn}`}>The server has no Square credentials loaded. Run the sandbox setup script and restart the application before saving a merchant.</p>}
    {info?.configured && info.credentialsConfigured && !info.applicationId && <p className={`${glass.notice} ${glass.warn}`}>The Square application ID is missing on the server, so the browser card form cannot start.</p>}
    {canManage ? <form onSubmit={save} style={{ display: 'grid', gap: 14 }}>
      <div className={glass.fields}>
        <label className={glass.field}>Environment<select disabled={busy} value={env} onChange={e => { setEnv(e.target.value as 'sandbox' | 'production'); setReviewed(false); }} style={{ minHeight: 46, borderRadius: 14, padding: '0 12px', font: 'inherit' }}><option value="sandbox">Sandbox (test)</option><option value="production">Production (real cards)</option></select></label>
        <label className={glass.field}>Square merchant ID<input required maxLength={128} disabled={busy} value={merchant} onChange={e => setMerchant(e.target.value)} autoComplete="off" spellCheck={false} /></label>
        <label className={glass.field}>Square location ID<input required maxLength={128} disabled={busy} value={squareLocation} onChange={e => setSquareLocation(e.target.value)} autoComplete="off" spellCheck={false} /></label>
      </div>
      {env === 'production' && <p className={`${glass.notice} ${glass.warn}`}>Production charges real cards. Only save this once the client&rsquo;s merchant and the production credential are approved.</p>}
      <label className={glass.switch}><span>I confirmed these belong to the Square account for this location.</span><input type="checkbox" checked={reviewed} disabled={busy} onChange={e => setReviewed(e.target.checked)} /></label>
      <div className={glass.actions}><button className={glass.button} disabled={busy || !reviewed || !merchant.trim() || !squareLocation.trim()}>{busy ? 'Checking with Square…' : b ? 'Save new version' : 'Save Square merchant'}</button></div>
    </form> : <p className={glass.notice}>Only people who can configure invoices can change the Square merchant.</p>}
  </div></section>;
}
