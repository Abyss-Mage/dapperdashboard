'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

/* Owner settings for the public website's tags. Only identifiers are stored; the site builds the tags itself. */
export default function WebsiteTrackingEditor({ organizationId, revision }: { organizationId: string; revision: number }) {
  const url = `/api/v1/organizations/${organizationId}/website-tracking`;
  const [gtm, setGtm] = useState(''), [pixel, setPixel] = useState(''), [mailchimp, setMailchimp] = useState('');
  const [version, setVersion] = useState(0), [loaded, setLoaded] = useState(false), [busy, setBusy] = useState(false);
  const [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const r = await fetch(url, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Website tracking is unavailable or access has changed.');
    setGtm(b.gtmId ?? ''); setPixel(b.metaPixelId ?? ''); setMailchimp(b.mailchimpScriptUrl ?? ''); setVersion(b.version ?? 0); setLoaded(true);
  }, [url]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load, revision]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ gtmId: gtm, metaPixelId: pixel, mailchimpScriptUrl: mailchimp, version }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The settings could not be saved.');
      await load();
      setNotice('Saved. The public website loads these on its next page view.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The settings could not be saved.'); }
    finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Website tracking</h2>
        <p className={styles.personSub}>The tags the public website loads, as they were in WordPress. Leave a field empty to switch it off. Paste only the identifier, never a script.</p>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {loaded && (
        <form onSubmit={save} className={styles.resource}>
          <div className={styles.inlineForm}>
            <label className="form-field">Google Tag Manager container
              <input value={gtm} onChange={(e) => setGtm(e.target.value)} placeholder="GTM-XXXXXXX" maxLength={20} autoComplete="off" disabled={busy} />
            </label>
            <label className="form-field">Meta Pixel ID
              <input value={pixel} onChange={(e) => setPixel(e.target.value)} placeholder="1234567890123456" inputMode="numeric" maxLength={20} autoComplete="off" disabled={busy} />
            </label>
            <label className="form-field">Mailchimp popup script address
              <input value={mailchimp} onChange={(e) => setMailchimp(e.target.value)} placeholder="https://chimpstatic.com/mcjs-connected/js/users/…" maxLength={600} autoComplete="off" disabled={busy} />
            </label>
          </div>
          <p className={styles.footnote}>The Mailchimp address is the one inside the script tag Mailchimp gives you for the popup (it starts with https://chimpstatic.com/ or a list-manage.com address). Other addresses are refused.</p>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
          </div>
        </form>
      )}
    </div>
  );
}
