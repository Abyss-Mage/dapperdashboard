'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import MediaLibrary from './MediaLibrary';

type Site = { slug: string; name: string; published: boolean; version: number };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* The Website images page: the media library, once this franchise has a website entry to attach pictures to. */
export default function WebsiteImages({ base, franchiseName, franchiseCode }: { base: string; franchiseName: string; franchiseCode: string }) {
  const root = `${base}/content`;
  const [site, setSite] = useState<Site | null | undefined>(undefined);
  const [busy, setBusy] = useState(false), [error, setError] = useState('');

  const load = useCallback(async () => {
    const data = await readJson(await fetch(root, { cache: 'no-store' }));
    setSite(data.site?.slug ? data.site : null);
  }, [root]);
  useEffect(() => { setSite(undefined); setError(''); load().catch((e) => setError(e.message)); }, [load]);

  async function createSite() {
    setBusy(true); setError('');
    try {
      const slug = (franchiseCode || franchiseName).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40) || 'main';
      await readJson(await fetch(`${root}/site`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ slug, name: franchiseName || 'Website', published: true, version: 0 }) }));
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The website could not be set up.'); }
    finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      {error && <p role="alert" className="field-error">{error}</p>}
      {site === null && (
        <div className={styles.resource}>
          <p>This franchise does not have a website entry yet. Create it to start adding pictures.</p>
          <button type="button" className="primary" disabled={busy} onClick={() => void createSite()}>{busy ? 'Setting up…' : 'Set up the website entry'}</button>
        </div>
      )}
      {site && <MediaLibrary base={base} />}
    </div>
  );
}
