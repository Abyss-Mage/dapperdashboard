'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

type Dealer = { id: string; mediaId: string; name: string; sortOrder: number; published: boolean; version: number };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

/* The dealerships shown in the "Trusted by the dealers" section of the website, for the selected franchise: a name and a logo each.
   Nothing is shown until it is published, and publishing needs the box confirming the dealer's logo may be shown. Once at least one
   dealer is published here, the website shows these instead of its built-in list. */
export default function WebsiteDealers({ base }: { base: string }) {
  const root = `${base}/content`;
  const [dealers, setDealers] = useState<Dealer[] | null>(null);
  const [name, setName] = useState('');
  const [rights, setRights] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [target, setTarget] = useState<Dealer | null>(null);
  const pick = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    const data = await readJson(await fetch(root, { cache: 'no-store' }));
    setDealers(data.dealers ?? []);
  }, [root]);
  useEffect(() => { setDealers(null); setError(''); setNotice(''); load().catch((e) => setError(e.message)); }, [load]);

  async function add(files: FileList | null) {
    const file = files?.[0];
    if (!file) return;
    const clean = name.trim();
    if (clean.length < 2) { setError('Type the dealership name first.'); if (pick.current) pick.current.value = ''; return; }
    if (!/^image\/(jpeg|png)$/.test(file.type)) { setError(`"${file.name}" is not a JPEG or PNG logo.`); return; }
    if (file.size > 6 * 1024 * 1024) { setError(`"${file.name}" is over 6 MB. Choose a smaller copy.`); return; }
    setBusy(true); setError(''); setNotice('');
    try {
      const media = await readJson(await fetch(`${root}/media`, { method: 'POST', headers: { 'Content-Type': file.type }, body: file }));
      const order = (dealers ?? []).reduce((m, d) => Math.max(m, d.sortOrder), -10) + 10;
      await readJson(await fetch(`${root}/dealers`, send('PUT', { mediaId: media.id, name: clean, sortOrder: order, published: rights, rightsReviewed: rights })));
      setNotice(`${clean} added${rights ? ' and shown on the website.' : ' but hidden. Tick the box and switch it on when you are ready.'}`);
      setName('');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The dealership could not be added.'); }
    finally { setBusy(false); if (pick.current) pick.current.value = ''; }
  }

  async function save(d: Dealer, change: Partial<Dealer>) {
    setBusy(true); setError(''); setNotice('');
    const next = { ...d, ...change };
    try {
      await readJson(await fetch(`${root}/dealers`, send('PUT', { id: d.id, mediaId: d.mediaId, name: next.name.trim(), sortOrder: next.sortOrder, published: next.published, version: d.version, rightsReviewed: next.published })));
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); await load().catch(() => undefined); }
    finally { setBusy(false); }
  }

  async function move(index: number, by: -1 | 1) {
    if (!dealers) return;
    const next = [...dealers]; const to = index + by;
    if (to < 0 || to >= next.length) return;
    [next[index], next[to]] = [next[to], next[index]];
    setDealers(next); setBusy(true); setError('');
    try { await readJson(await fetch(`${root}/dealers-order`, send('PUT', { ids: next.map((d) => d.id) }))); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The order could not be saved.'); await load().catch(() => undefined); }
    finally { setBusy(false); }
  }

  async function remove() {
    if (!target) return;
    setBusy(true); setError(''); setNotice('');
    try { await readJson(await fetch(`${root}/dealers/${target.id}`, { method: 'DELETE' })); setNotice(`${target.name} was removed.`); setTarget(null); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The dealership could not be removed.'); setTarget(null); }
    finally { setBusy(false); }
  }

  if (!dealers) return error ? <p role="alert" className="field-error">{error}</p> : null;
  const live = dealers.filter((d) => d.published).length;
  return (
    <section className={styles.stack} aria-labelledby="site-dealers-title" style={{ marginTop: 28 }}>
      <div>
        <h2 id="site-dealers-title" className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Dealerships on the website</h2>
        <p className={styles.personSub}>The logos in &ldquo;Trusted by the dealers&rdquo;. {live > 0 ? `${live} shown now, in this order.` : 'Until you publish one, the website shows its built-in list.'}</p>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.resource}>
        <div className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
          <label className="form-field" style={{ maxWidth: 320 }}>Dealership name
            <input value={name} maxLength={120} disabled={busy} onChange={(e) => setName(e.target.value)} placeholder="For example, Holiday Ford" />
          </label>
          <div>
            <input ref={pick} type="file" accept="image/jpeg,image/png" hidden onChange={(e) => void add(e.target.files)} />
            <button type="button" className="primary" disabled={busy} onClick={() => pick.current?.click()}>{busy ? 'Working…' : 'Choose logo and add'}</button>
          </div>
        </div>
        <label style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginTop: 12 }}>
          <input type="checkbox" checked={rights} onChange={(e) => setRights(e.target.checked)} />
          <span>We have permission to show this dealership&apos;s name and logo publicly. Dealerships added with this ticked are shown right away.</span>
        </label>
        <p className={styles.footnote}>JPEG or PNG, up to 6 MB. A logo on a white or transparent background looks best, because the website sets each one on a white diamond.</p>
      </div>
      {dealers.length === 0 && <p className={styles.status}>No dealerships added yet.</p>}
      <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 14 }}>
        {dealers.map((d, i) => (
          <li key={d.id} className={styles.resource} style={{ display: 'grid', gap: 10, opacity: d.published ? 1 : 0.7 }}>
            <div style={{ background: '#fff', borderRadius: 12, aspectRatio: '16 / 10', display: 'grid', placeItems: 'center', padding: 12 }}>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={`${root}/media/${d.mediaId}`} alt={d.name + ' logo'} loading="lazy" style={{ maxWidth: '100%', maxHeight: '100%', objectFit: 'contain' }} />
            </div>
            <input aria-label="Dealership name" defaultValue={d.name} maxLength={120} disabled={busy} onBlur={(e) => { const v = e.target.value.trim(); if (v.length >= 2 && v !== d.name) void save(d, { name: v }); }} style={{ width: '100%' }} />
            <div className={styles.rowBetween}>
              <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <input type="checkbox" checked={d.published} disabled={busy || (!d.published && !rights)} onChange={(e) => void save(d, { published: e.target.checked })} />
                <span>{d.published ? 'On the website' : 'Hidden'}</span>
              </label>
              <span className={styles.svcActions}>
                <button type="button" className="secondary" disabled={busy || i === 0} onClick={() => void move(i, -1)} aria-label="Move earlier">↑</button>
                <button type="button" className="secondary" disabled={busy || i === dealers.length - 1} onClick={() => void move(i, 1)} aria-label="Move later">↓</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => setTarget(d)}>Delete</button>
              </span>
            </div>
            {!d.published && !rights && <span className={styles.personSub}>Tick the permission box above to show this one.</span>}
          </li>
        ))}
      </ul>
      <ConfirmDialog isOpen={target !== null} title="Remove dealership" entityName={target?.name ?? ''} entityType="Dealership" actionType="delete" isBusy={busy}
        explanation="The dealership is taken off the website and its logo is deleted. This cannot be undone." onConfirm={() => void remove()} onCancel={() => setTarget(null)} />
    </section>
  );
}
