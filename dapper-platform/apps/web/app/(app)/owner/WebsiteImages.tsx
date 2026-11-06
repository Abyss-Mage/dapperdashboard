'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import ConfirmDialog from '../workspace/ConfirmDialog';
import WebsiteDealers from './WebsiteDealers';
import SiteMedia from './SiteMedia';

type Photo = { id: string; mediaId: string; title: string; alt: string; category: string; sortOrder: number; published: boolean; version: number; width?: number; height?: number };
type Site = { slug: string; name: string; published: boolean; version: number };
const CATEGORIES: [string, string][] = [['detailing', 'Detailing'], ['ceramic', 'Ceramic coating'], ['ppf', 'Paint protection film'], ['other', 'Other']];

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
const nameFromFile = (file: string) => file.replace(/\.[^.]+$/, '').replace(/[-_]+/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 100);

/* The photos on the website's Portfolio page for the selected franchise. Add photos (they appear on the website right away when
   published), put them in order, hide or delete them. Once at least one photo is published here, the website shows these instead
   of its built-in photos. */
export default function WebsiteImages({ base, franchiseName, franchiseCode }: { base: string; franchiseName: string; franchiseCode: string }) {
  const root = `${base}/content`;
  const [site, setSite] = useState<Site | null>(null);
  const [photos, setPhotos] = useState<Photo[] | null>(null);
  const [category, setCategory] = useState('detailing');
  const [rights, setRights] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [target, setTarget] = useState<Photo | null>(null);
  const pick = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    const data = await readJson(await fetch(root, { cache: 'no-store' }));
    setSite(data.site?.slug ? data.site : null);
    setPhotos(data.portfolio ?? []);
  }, [root]);
  useEffect(() => { setPhotos(null); setError(''); setNotice(''); load().catch((e) => setError(e.message)); }, [load]);

  async function createSite() {
    setBusy(true); setError('');
    try {
      const slug = (franchiseCode || franchiseName).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40) || 'main';
      await readJson(await fetch(`${root}/site`, send('PUT', { slug, name: franchiseName || 'Website', published: true, version: 0 })));
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The website could not be set up.'); }
    finally { setBusy(false); }
  }

  async function upload(files: FileList | null) {
    if (!files || !files.length) return;
    setBusy(true); setError(''); setNotice('');
    let added = 0;
    try {
      let order = (photos ?? []).reduce((m, p) => Math.max(m, p.sortOrder), -10) + 10;
      for (const file of Array.from(files)) {
        if (!/^image\/(jpeg|png)$/.test(file.type)) throw new Error(`"${file.name}" is not a JPEG or PNG photo.`);
        if (file.size > 6 * 1024 * 1024) throw new Error(`"${file.name}" is over 6 MB. Choose a smaller copy.`);
        const media = await readJson(await fetch(`${root}/media`, { method: 'POST', headers: { 'Content-Type': file.type }, body: file }));
        const title = nameFromFile(file.name) || 'Portfolio photo';
        await readJson(await fetch(`${root}/portfolio`, send('PUT', { mediaId: media.id, title: title.length < 2 ? 'Portfolio photo' : title, alt: title.length < 2 ? 'Portfolio photo' : title, category, sortOrder: order, published: rights, rightsReviewed: rights })));
        order += 10; added += 1;
      }
      setNotice(`${added} photo${added === 1 ? '' : 's'} added${rights ? ' and published to the website.' : ' but not published yet. Tick the box above and switch each one on when you are ready.'}`);
    } catch (e) { setError((e instanceof Error ? e.message : 'The photos could not be added.') + (added ? ` ${added} were added before this.` : '')); }
    finally { setBusy(false); if (pick.current) pick.current.value = ''; await load().catch(() => undefined); }
  }

  async function save(p: Photo, change: Partial<Photo>) {
    setBusy(true); setError(''); setNotice('');
    const next = { ...p, ...change };
    try {
      await readJson(await fetch(`${root}/portfolio`, send('PUT', { id: p.id, mediaId: p.mediaId, title: next.title.trim(), alt: next.alt.trim() || next.title.trim(), category: next.category, sortOrder: next.sortOrder, published: next.published, version: p.version, rightsReviewed: next.published })));
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); await load().catch(() => undefined); }
    finally { setBusy(false); }
  }

  async function move(index: number, by: -1 | 1) {
    if (!photos) return;
    const next = [...photos]; const to = index + by;
    if (to < 0 || to >= next.length) return;
    [next[index], next[to]] = [next[to], next[index]];
    setPhotos(next); setBusy(true); setError('');
    try { await readJson(await fetch(`${root}/portfolio-order`, send('PUT', { ids: next.map((p) => p.id) }))); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The order could not be saved.'); await load().catch(() => undefined); }
    finally { setBusy(false); }
  }

  async function remove() {
    if (!target) return;
    setBusy(true); setError(''); setNotice('');
    try { await readJson(await fetch(`${root}/portfolio/${target.id}`, { method: 'DELETE' })); setNotice(`"${target.title}" was deleted.`); setTarget(null); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The photo could not be deleted.'); setTarget(null); }
    finally { setBusy(false); }
  }

  const live = (photos ?? []).filter((p) => p.published).length;
  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Website images</h2>
          <p className={styles.personSub}>The photos on the website&apos;s Portfolio page for {franchiseName || 'this franchise'}. {live > 0 ? `${live} published now, shown in this order.` : 'Until you publish one, the website shows its built-in photos.'}</p>
        </div>
        <a className="secondary" href="/portfolio" target="_blank" rel="noreferrer" style={{ padding: '0.55rem 1rem', borderRadius: 999, border: '1px solid var(--glass-line)' }}>View on website</a>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {photos && !site && (
        <div className={styles.resource}>
          <p>This franchise does not have a website entry yet. Create it to start adding portfolio photos.</p>
          <button type="button" className="primary" disabled={busy} onClick={() => void createSite()}>{busy ? 'Setting up…' : 'Set up the website entry'}</button>
        </div>
      )}
      {photos && site && (
        <>
          <div className={styles.resource}>
            <div className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
              <label className="form-field" style={{ maxWidth: 260 }}>Section
                <Select value={category} onChange={(e) => setCategory(e.target.value)} disabled={busy}>{CATEGORIES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</Select>
              </label>
              <div>
                <input ref={pick} type="file" accept="image/jpeg,image/png" multiple hidden onChange={(e) => void upload(e.target.files)} />
                <button type="button" className="primary" disabled={busy} onClick={() => pick.current?.click()}>{busy ? 'Working…' : 'Add photos'}</button>
              </div>
            </div>
            <label style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginTop: 12 }}>
              <input type="checkbox" checked={rights} onChange={(e) => setRights(e.target.checked)} />
              <span>These photos are ours, or we have permission to show them publicly. Photos added with this ticked are published right away.</span>
            </label>
            <p className={styles.footnote}>JPEG or PNG, up to 6 MB each. You can choose several at once. Edit the title afterwards; it is also the text read to people who cannot see the photo.</p>
          </div>
          {photos.length === 0 && <p className={styles.status}>No photos yet.</p>}
          <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 14 }}>
            {photos.map((p, i) => (
              <li key={p.id} className={styles.resource} style={{ display: 'grid', gap: 10, opacity: p.published ? 1 : 0.7 }}>
                <img src={`${root}/media/${p.mediaId}`} alt={p.alt} loading="lazy" style={{ width: '100%', aspectRatio: '4 / 3', objectFit: 'cover', borderRadius: 12, background: 'rgba(255,255,255,.04)' }} />
                <input aria-label="Photo title" defaultValue={p.title} maxLength={120} disabled={busy} onBlur={(e) => { const v = e.target.value.trim(); if (v.length >= 2 && v !== p.title) void save(p, { title: v, alt: v }); }} style={{ width: '100%' }} />
                <Select value={p.category} disabled={busy} onChange={(e) => void save(p, { category: e.target.value })} aria-label="Section">{CATEGORIES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</Select>
                <div className={styles.rowBetween}>
                  <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <input type="checkbox" checked={p.published} disabled={busy || (!p.published && !rights)} onChange={(e) => void save(p, { published: e.target.checked })} />
                    <span>{p.published ? 'On the website' : 'Hidden'}</span>
                  </label>
                  <span className={styles.svcActions}>
                    <button type="button" className="secondary" disabled={busy || i === 0} onClick={() => void move(i, -1)} aria-label="Move earlier">↑</button>
                    <button type="button" className="secondary" disabled={busy || i === photos.length - 1} onClick={() => void move(i, 1)} aria-label="Move later">↓</button>
                    <button type="button" className="secondary" disabled={busy} onClick={() => setTarget(p)}>Delete</button>
                  </span>
                </div>
                {!p.published && !rights && <span className={styles.personSub}>Tick the permission box above to publish this one.</span>}
              </li>
            ))}
          </ul>
        </>
      )}
      {photos && site && <WebsiteDealers base={base} />}
      {photos && site && <SiteMedia base={base} />}
      <ConfirmDialog isOpen={target !== null} title="Delete photo" entityName={target?.title ?? ''} entityType="Photo" actionType="delete" isBusy={busy}
        explanation="The photo is taken off the website and deleted. This cannot be undone." onConfirm={() => void remove()} onCancel={() => setTarget(null)} />
    </div>
  );
}
