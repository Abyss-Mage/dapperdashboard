'use client';
import { Landmark } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';
import Select from '../GlassSelect';

/* Card payments for one franchise. Each franchise connects its OWN Square account with two keys from its Square app (Application
   ID and Access token, test or live; owner 2026-11-13: no Square sign-in page, no shared keys), then picks which of that account's
   Square locations this franchise charges to. Square confirms the location
   (active, CAD, card processing) before it is saved, and every change is kept as a new version. */

type Env = 'sandbox' | 'production';
type Info = { configured: boolean; credentialsConfigured?: boolean; applicationId?: string; binding?: { version: number; environment: Env; merchantId: string; squareLocationId: string; reviewedAt: string } };
type Connection = { environment: Env; merchantId: string; connectedAt: string; expiresAt: string; refreshedAt: string };
type SquareLocation = { id: string; name: string; currency: string; status: string; cards: boolean };
const json = { 'Content-Type': 'application/json' };
const ENV_LABEL: Record<Env, string> = { sandbox: 'Test (sandbox)', production: 'Live (real cards)' };

async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Square is unavailable right now.'); return b as T; }

export default function SquareMerchantPanel({ organizationId, locationId, canManage }: { organizationId: string; locationId: string; canManage: boolean }) {
  const scope = `/api/v1/organizations/${organizationId}/locations/${locationId}/square`;
  const [info, setInfo] = useState<Info | null>(null);
  const [connections, setConnections] = useState<Connection[]>([]);
  const [env, setEnv] = useState<Env>('sandbox');
  const [places, setPlaces] = useState<{ merchantId: string; locations: SquareLocation[] } | null>(null);
  const [pick, setPick] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    setInfo(await read<Info>(await fetch(`${scope}/binding`, { cache: 'no-store' })));
    if (canManage) {
      const c = await read<{ connections: Connection[] }>(await fetch(`${scope}/connection`, { cache: 'no-store' }));
      setConnections(c.connections);
    }
  }, [scope, canManage]);
  useEffect(() => {
    setInfo(null); setPlaces(null); setPick(''); setError(''); setNotice('');
    load().catch((e) => setError(e.message));
  }, [load]);

  const connection = connections.find((c) => c.environment === env);
  useEffect(() => {
    setPlaces(null); setPick('');
    if (!connection) return;
    let live = true;
    fetch(`${scope}/connection/locations?environment=${env}`, { cache: 'no-store' }).then((r) => read<{ merchantId: string; locations: SquareLocation[] }>(r)).then((b) => { if (live) setPlaces(b); }).catch((e) => { if (live) setError(e.message); });
    return () => { live = false; };
  }, [connection, env, scope]);

  const [keys, setKeys] = useState({ applicationId: '', accessToken: '' });
  const [editing, setEditing] = useState(false);
  async function saveKeys() {
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(`${scope}/connection/keys`, { method: 'POST', headers: json, body: JSON.stringify({ environment: env, ...keys }) });
      if (!r.ok && r.status !== 204) await read(r);
      setKeys({ applicationId: '', accessToken: '' }); setEditing(false);
      setNotice('Square accepted the keys. Now choose the Square location this franchise charges to.'); await load();
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function disconnect() {
    setBusy(true); setError(''); setNotice('');
    try { const r = await fetch(`${scope}/connection/disconnect?environment=${env}`, { method: 'POST', headers: json, body: '{}' }); if (!r.ok && r.status !== 204) await read(r); setNotice('Square account disconnected. Card payments for this franchise stop until an account is connected again.'); await load(); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function useLocation() {
    if (!places || !pick) return;
    setBusy(true); setError(''); setNotice('');
    try {
      await read(await fetch(`${scope}/binding`, { method: 'PUT', headers: json, body: JSON.stringify({ environment: env, merchantId: places.merchantId, squareLocationId: pick, credentialReference: `oauth:${locationId}`, expectedVersion: info?.binding?.version ?? 0 }) }));
      setNotice('Saved. Square confirmed this location for CAD card payments.'); await load();
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }

  const b = info?.binding;
  return <section className={glass.stage} aria-labelledby="sq-merchant-title"><div className={glass.card}>
    <div className={glass.head}><h2 id="sq-merchant-title" className={glass.title}><Landmark size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Card payments (Square)</h2>
      <span className={`${glass.pill} ${info?.configured ? glass.pillOk : ''}`}>{info ? (info.configured ? 'Taking cards' : 'Not set up') : 'Loading'}</span></div>
    <p className={glass.lede}>Each franchise uses its own Square account. Paste its Square keys once, then choose which Square location this franchise charges to.</p>
    {error && <p className={glass.alert} role="alert">{error}</p>}{notice && <p className={glass.notice} role="status">{notice}</p>}
    {b && <ul className={glass.list}><li className={glass.row}><span className={glass.name}>{ENV_LABEL[b.environment]}</span><span className={`${glass.pill} ${b.environment === 'production' ? glass.pillWarn : ''}`}>Version {b.version}</span>
      <span className={glass.meta}>Square account {b.merchantId} · location {b.squareLocationId} · saved {new Date(b.reviewedAt).toLocaleDateString()}</span></li></ul>}
    {canManage ? <div style={{ display: 'grid', gap: 14 }}>
      <div className={glass.fields}>
        <label className={glass.field}>Square account type<Select disabled={busy} value={env} onChange={(e) => setEnv(e.target.value as Env)} style={{ minHeight: 46, borderRadius: 14, padding: '0 12px', font: 'inherit' }}><option value="sandbox">{ENV_LABEL.sandbox}</option><option value="production">{ENV_LABEL.production}</option></Select></label>
      </div>
      {(!connection || editing) && <form className={glass.notice} style={{ display: 'grid', gap: 10 }} onSubmit={(e) => { e.preventDefault(); void saveKeys(); }}>
        <p style={{ margin: 0 }}>In the <strong>Square Developer Console</strong> open this franchise&rsquo;s app, go to <strong>Credentials</strong>, set the switch at the top to <strong>{env === 'production' ? 'Production' : 'Sandbox'}</strong>, and copy the two keys.</p>
        <label className={glass.field}>Application ID<input value={keys.applicationId} onChange={(e) => setKeys({ ...keys, applicationId: e.target.value })} autoComplete="off" required disabled={busy} placeholder={env === 'production' ? 'sq0idp-…' : 'sandbox-sq0idb-…'} style={{ minHeight: 46, borderRadius: 14, padding: '0 12px', font: 'inherit' }} /></label>
        <label className={glass.field}>Access token<input type="password" value={keys.accessToken} onChange={(e) => setKeys({ ...keys, accessToken: e.target.value })} autoComplete="off" required disabled={busy} placeholder="EAAA…" style={{ minHeight: 46, borderRadius: 14, padding: '0 12px', font: 'inherit' }} /></label>
        {env === 'production' && <p className={`${glass.notice} ${glass.warn}`} style={{ margin: 0 }}>Live keys charge real cards.</p>}
        <div className={glass.actions} style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className={glass.button} disabled={busy || !keys.applicationId.trim() || keys.accessToken.trim().length < 20}>{busy ? 'Checking with Square…' : connection ? 'Replace the keys' : 'Save keys'}</button>
          {editing && <button type="button" className={glass.button} disabled={busy} onClick={() => setEditing(false)}>Cancel</button>}
        </div>
      </form>}
      {connection ? <>
        <p className={glass.notice}>Connected to Square account {connection.merchantId} on {new Date(connection.connectedAt).toLocaleDateString()}.</p>
        {!places ? <p role="status" className={glass.lede}>Loading this account&rsquo;s Square locations…</p> : <>
          <label className={glass.field}>Square location for this franchise<Select disabled={busy} value={pick} onChange={(e) => setPick(e.target.value)} style={{ minHeight: 46, borderRadius: 14, padding: '0 12px', font: 'inherit' }}><option value="">Choose a location</option>{places.locations.map((l) => <option key={l.id} value={l.id} disabled={!l.cards || l.status !== 'ACTIVE' || l.currency !== 'CAD'}>{l.name} · {l.currency}{!l.cards ? ' · cannot take cards' : l.status !== 'ACTIVE' ? ' · inactive' : ''}</option>)}</Select></label>
          {env === 'production' && <p className={`${glass.notice} ${glass.warn}`}>Live mode charges real cards.</p>}
        </>}
        <div className={glass.actions} style={{ gap: 8, display: 'flex', flexWrap: 'wrap' }}>
          <button className={glass.button} disabled={busy || !pick} onClick={() => void useLocation()}>{busy ? 'Checking with Square…' : b ? 'Use this location (new version)' : 'Use this location'}</button>
          {!editing && <button type="button" className={glass.button} disabled={busy} onClick={() => { setEditing(true); setNotice(''); }}>Change the keys</button>}
          <button type="button" className={glass.button} disabled={busy} onClick={() => void disconnect()}>Disconnect Square</button>
        </div>
      </> : null}
    </div> : <p className={glass.notice}>Only people who can configure invoices can change card payments.</p>}
  </div></section>;
}
