'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import styles from './owner.module.css';
import { fitImage } from './fitImage';

type Built = { path: string; kind: 'image' | 'video'; bytes: number };
type Mine = { path: string; contentType: string; bytes: number; updatedAt: string };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}
const mb = (n: number) => (n / 1048576 >= 1 ? (n / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB');
const brandMark = (path: string) => path === 'logo.webp' || path === 'favicon.webp';
const shapeOf = (url: string) => new Promise<number | undefined>((resolve) => { const i = new Image(); i.onload = () => resolve(i.naturalWidth / i.naturalHeight || undefined); i.onerror = () => resolve(undefined); i.src = url; });
const nice = (folder: string) => ({ services: 'Service and vehicle pictures', portfolio: 'Portfolio photos', partners: 'Dealership logos', transformations: 'Film clips (built in)', team: 'Team photos', about: 'About page', process: 'How we do it' } as Record<string, string>)[folder] ?? folder.charAt(0).toUpperCase() + folder.slice(1);

/* Every picture and film of the website. Replace any of them with your own file (the website shows yours at once, the original is kept
   and comes back with "Put the original back"), and add or remove the films shown on Home and Portfolio. */
export default function SiteMedia({ base }: { base: string }) {
  const root = `${base}/site-assets`;
  const [built, setBuilt] = useState<Built[] | null>(null);
  const [mine, setMine] = useState<Mine[]>([]);
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(''), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [bump, setBump] = useState(0);
  const target = useRef('');
  const pick = useRef<HTMLInputElement>(null);
  const [accept, setAccept] = useState('image/jpeg,image/png,image/webp');

  const load = useCallback(async () => {
    const [b, m] = await Promise.all([fetch('/site-assets', { cache: 'no-store' }).then(readJson), fetch(root, { cache: 'no-store' }).then(readJson)]);
    setBuilt(b.items ?? []); setMine(m.items ?? []); setHidden(new Set<string>(m.hidden ?? []));
  }, [root]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function upload(path: string, file: File) {
    setBusy(path); setError(''); setNotice('');
    try {
      // Pictures are made ready for their slot: logos lose their blank margins, everything else takes the shape of the picture it replaces.
      let body: Blob = file;
      if (!file.type.startsWith('video/')) {
        const logo = path.startsWith('partners/') || brandMark(path);
        body = await fitImage(file, logo ? { logo } : { ratio: await shapeOf(`/site/assets/${path}?v=${bump}`) });
      }
      await readJson(await fetch(`${root}?path=${encodeURIComponent(path)}`, { method: 'PUT', body }));
      setNotice('Saved. The website shows it now.'); setBump((n) => n + 1); await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(''); }
  }
  async function remove(path: string, said: string) {
    setBusy(path); setError(''); setNotice('');
    try { await readJson(await fetch(`${root}?path=${encodeURIComponent(path)}`, { method: 'DELETE' })); setNotice(said); setBump((n) => n + 1); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(''); }
  }
  // Any picture can be taken off the website except the brand's own logo and icon; the file stays and can come back.
  async function setShown(path: string, show: boolean) {
    setBusy(path); setError(''); setNotice('');
    try { await readJson(await fetch(`${root}/hidden?path=${encodeURIComponent(path)}`, { method: show ? 'DELETE' : 'PUT' })); setNotice(show ? 'It is back on the website.' : 'It is off the website.'); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(''); }
  }
  function open(path: string, kind: 'image' | 'video') { target.current = path; setAccept(kind === 'video' ? 'video/mp4,video/webm' : 'image/*'); setTimeout(() => pick.current?.click(), 0); }
  function onFile(files: FileList | null) {
    const f = files?.[0];
    if (pick.current) pick.current.value = '';
    if (!f) return;
    const ext = (f.name.split('.').pop() ?? '').toLowerCase().replace(/[^a-z0-9]/g, '');
    const path = target.current || `films/${crypto.randomUUID().slice(0, 8)}.${ext || 'mp4'}`;
    void upload(path, f);
  }

  const replaced = new Set(mine.map((m) => m.path));
  const films = mine.filter((m) => m.path.startsWith('films/') && m.contentType.startsWith('video/'));
  const groups = new Map<string, Built[]>();
  for (const b of built ?? []) { const g = b.path.includes('/') ? b.path.split('/')[0] : 'More'; groups.set(g, [...(groups.get(g) ?? []), b]); }
  const src = (p: string) => `/site/assets/${p}?v=${bump}`;

  return (
    <div className={styles.stack}>
      <div>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Pictures and films of the whole website</h2>
        <p className={styles.personSub}>Replace or remove any picture or film, and add films for Home and Portfolio. A new picture is trimmed evenly from its edges to the shape of the one it replaces, so it fits its place; logos are shown whole. Films are MP4 or WebM, up to 25 MB. Only upload files you have the right to show publicly.</p>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <input ref={pick} type="file" accept={accept} hidden onChange={(e) => onFile(e.target.files)} />

      <section className={styles.resource} aria-label="Films">
        <div className={styles.rowBetween}>
          <h3 style={{ margin: 0 }}>Films on Home and Portfolio</h3>
          <button type="button" className="primary" disabled={!!busy} onClick={() => open('', 'video')}>{busy.startsWith('films/') ? 'Uploading…' : 'Add a film'}</button>
        </div>
        <p className={styles.personSub}>{films.length ? `The website shows these ${films.length} film${films.length === 1 ? '' : 's'}, in this order.` : 'No films added yet, so the website shows its own five films. Add a film and your films are shown instead.'}</p>
        {films.length > 0 && (
          <ul className={styles.smGrid}>
            {films.map((f) => (
              <li key={f.path} className={styles.smCard}>
                <video src={src(f.path)} muted playsInline controls preload="metadata" className={styles.smFilm} />
                <span className={styles.personSub}>{mb(f.bytes)}</span>
                <button type="button" className="secondary" disabled={!!busy} onClick={() => void remove(f.path, 'Film removed.')}>Remove</button>
              </li>
            ))}
          </ul>
        )}
        {films.length > 0 && <button type="button" className="secondary" disabled={!!busy} onClick={async () => { for (const f of films) await remove(f.path, 'Back to the built-in films.'); }}>Use the built-in films again</button>}
      </section>

      {!built && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading pictures" />}
      {[...groups.entries()].map(([group, list]) => (
        <section key={group} className={styles.resource} aria-label={nice(group)}>
          <h3 style={{ margin: 0 }}>{nice(group)}</h3>
          <ul className={styles.smGrid}>
            {list.map((b) => (
              <li key={b.path} className={styles.smCard} data-off={hidden.has(b.path) || undefined}>
                {b.kind === 'video'
                  ? <video src={src(b.path)} muted playsInline controls preload="metadata" className={styles.smThumb} />
                  : /* eslint-disable-next-line @next/next/no-img-element */ <img src={src(b.path)} alt="" loading="lazy" className={styles.smThumb} />}
                <span className={styles.smName}>{b.path.split('/').pop()}{replaced.has(b.path) ? ' · replaced' : ''}{hidden.has(b.path) ? ' · off the website' : ''}</span>
                <div className={styles.svcActions}>
                  <button type="button" className="secondary" disabled={!!busy} onClick={() => open(b.path, b.kind)}>{busy === b.path ? 'Saving…' : 'Replace'}</button>
                  {replaced.has(b.path) && <button type="button" className="secondary" disabled={!!busy} onClick={() => void remove(b.path, 'The original is back.')}>Put the original back</button>}
                  {!brandMark(b.path) && (hidden.has(b.path)
                    ? <button type="button" className="secondary" disabled={!!busy} onClick={() => void setShown(b.path, true)}>Show again</button>
                    : <button type="button" className="secondary" disabled={!!busy} onClick={() => void setShown(b.path, false)}>Remove</button>)}
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
