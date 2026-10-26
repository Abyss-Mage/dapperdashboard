'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

/* Square application keys for the online payment gateway: one set for testing (sandbox) and one for live payments.
   The Application ID and secret come from the Square Developer dashboard. The secret is sent to the server once, sealed there,
   and never shown again. Which set a franchise uses is chosen below, on the franchise's card payments settings. */

type Env = 'sandbox' | 'production';
type Status = { environment: Env; configured: boolean; applicationId: string; source: 'saved' | 'server' | ''; updatedAt?: string };
const LABEL: Record<Env, { title: string; help: string }> = {
  sandbox: { title: 'Testing (sandbox)', help: 'Practice keys. No real card is ever charged. Use these while you are setting up.' },
  production: { title: 'Live (production)', help: 'Real keys. Cards are charged for real. Add these only when you are ready to take payments.' },
};
async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? (Object.values(b.errors ?? {}).join('. ') || 'The keys could not be saved.')); return b as T; }

export default function SquareKeys({ organizationId }: { organizationId: string }) {
  const url = `/api/v1/organizations/${organizationId}/square/app-credentials`;
  const [items, setItems] = useState<Status[] | null>(null);
  const [redirect, setRedirect] = useState('');
  const [draft, setDraft] = useState<Record<Env, { id: string; secret: string }>>({ sandbox: { id: '', secret: '' }, production: { id: '', secret: '' } });
  const [busy, setBusy] = useState<Env | ''>('');
  const [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const b = await read<{ items: Status[]; redirectUrl: string }>(await fetch(url, { cache: 'no-store' }));
    setItems(b.items); setRedirect(b.redirectUrl);
  }, [url]);
  useEffect(() => { setItems(null); setError(''); load().catch((e) => setError(e.message)); }, [load]);

  async function save(env: Env) {
    setBusy(env); setError(''); setNotice('');
    try {
      await read(await fetch(`${url}/${env}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ applicationId: draft[env].id, secret: draft[env].secret }) }));
      setDraft((d) => ({ ...d, [env]: { id: '', secret: '' } }));
      setNotice(`${LABEL[env].title} keys saved. They are in use now.`);
      await load();
    } catch (e) { setError((e as Error).message); } finally { setBusy(''); }
  }
  async function clear(env: Env) {
    setBusy(env); setError(''); setNotice('');
    try { await read(await fetch(`${url}/${env}`, { method: 'DELETE' })); setNotice(`${LABEL[env].title} keys removed.`); await load(); }
    catch (e) { setError((e as Error).message); } finally { setBusy(''); }
  }

  return (
    <section className={styles.card} style={{ padding: 20 }}>
      <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>Online payment keys (Square)</h2>
      <p className={styles.personSub}>Keep a testing set and a live set. Each franchise then connects its own Square account using whichever set you choose in its card payments settings below.</p>
      {redirect && <p className={styles.personSub}>In your Square Developer dashboard, set the OAuth redirect URL to <code style={{ userSelect: 'all' }}>{redirect}</code></p>}
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading keys" />}
      <div className={styles.franchiseGrid} style={{ marginTop: 12 }}>
        {(['sandbox', 'production'] as Env[]).map((env) => {
          const st = items?.find((i) => i.environment === env), d = draft[env];
          return (
            <form key={env} className={styles.resource} onSubmit={(e) => { e.preventDefault(); void save(env); }} autoComplete="off">
              <div className={styles.resourceHead}>
                <div><h3 style={{ margin: 0 }}>{LABEL[env].title}</h3><p className={styles.personSub}>{LABEL[env].help}</p></div>
                <span className={`${styles.badge} ${st?.configured ? styles.badge_complete : ''}`}>{st?.configured ? (st.source === 'server' ? 'Set on the server' : 'Saved') : 'Not set'}</span>
              </div>
              {st?.configured && <p className={styles.personSub}>Application ID: <strong>{st.applicationId}</strong>{st.updatedAt ? ` · saved ${new Date(st.updatedAt).toLocaleDateString('en-CA')}` : ''}</p>}
              <label className="form-field">Application ID<input value={d.id} onChange={(e) => setDraft({ ...draft, [env]: { ...d, id: e.target.value } })} disabled={!!busy} maxLength={200} autoComplete="off" placeholder={env === 'sandbox' ? 'sandbox-sq0idb-…' : 'sq0idp-…'} /></label>
              <label className="form-field">Application secret<input type="password" value={d.secret} onChange={(e) => setDraft({ ...draft, [env]: { ...d, secret: e.target.value } })} disabled={!!busy} maxLength={500} autoComplete="new-password" placeholder={st?.configured ? 'Enter a new secret to replace it' : 'Paste the secret'} /></label>
              <div className={styles.rowBetween}>
                <button className="primary" disabled={!!busy || d.id.trim().length < 6 || d.secret.trim().length < 8}>{busy === env ? 'Saving…' : st?.configured ? 'Replace keys' : 'Save keys'}</button>
                {st?.source === 'saved' && <button type="button" className="secondary" disabled={!!busy} onClick={() => void clear(env)}>Remove keys</button>}
              </div>
            </form>
          );
        })}
      </div>
    </section>
  );
}
