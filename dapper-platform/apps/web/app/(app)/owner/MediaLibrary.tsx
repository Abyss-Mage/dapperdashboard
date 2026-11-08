'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Search, Trash2, Upload, X } from 'lucide-react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import ConfirmDialog from '../workspace/ConfirmDialog';
import { fitImage } from './fitImage';
import { partners } from '../../(site)/dapper';

type Photo = { id: string; mediaId: string; title: string; alt: string; category: string; sortOrder: number; published: boolean; version: number; binnedAt?: string };
type Dealer = { id: string; mediaId: string; name: string; sortOrder: number; published: boolean; version: number; binnedAt?: string };
type Built = { path: string; kind: 'image' | 'video'; bytes: number };
type Mine = { path: string; contentType: string };
type Item = { key: string; group: string; name: string; src: string; kind: 'image' | 'video'; draft?: boolean; replaced?: boolean; path?: string; photo?: Photo; dealer?: Dealer; removedAt?: string };

// The share of the website's logo card each size fills: [width %, height %]. The same numbers as dealers.css, so the preview is what visitors see.
const SIZES = [['s', 'Small', 55, 44], ['m', 'Medium', 78, 62], ['l', 'Large', 90, 76], ['xl', 'Full', 100, 100]] as const;
const SECTIONS: [string, string][] = [['detailing', 'Detailing'], ['ceramic', 'Ceramic coating'], ['ppf', 'Paint protection film'], ['other', 'Other']];
const GROUP_NAMES: Record<string, string> = { logos: 'Dealership logos', portfolio: 'Portfolio', films: 'Films', services: 'Service pictures', team: 'Team', about: 'About page', process: 'How we do it', More: 'Other' };
const groupOf = (path: string) => { if (path.startsWith('partners/')) return 'logos'; if (path.startsWith('transformations/') || path.startsWith('films/')) return 'films'; return path.includes('/') ? path.split('/')[0] : 'More'; };
const groupName = (g: string) => GROUP_NAMES[g] ?? g.charAt(0).toUpperCase() + g.slice(1);
const pretty = (file: string) => file.replace(/\.[^.]+$/, '').replace(/[-_]+/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 100);
const brandMark = (path?: string) => path === 'logo.webp' || path === 'favicon.webp';
const logoNames = new Map(partners.filter((p) => p.src).map((p) => [p.src!.replace('/site/assets/', ''), p.name]));
const shapeOf = (url: string) => new Promise<number | undefined>((resolve) => { const i = new Image(); i.onload = () => resolve(i.naturalWidth / i.naturalHeight || undefined); i.onerror = () => resolve(undefined); i.src = url; });
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* Everything the website shows, in one place, the way a content system works: pick a category, drop files in, click a picture to change it.
   Removing moves things to the Bin, sorted by category, where they can be restored or deleted forever. */
export default function MediaLibrary({ base }: { base: string }) {
  const root = `${base}/content`, assets = `${base}/site-assets`;
  const [built, setBuilt] = useState<Built[] | null>(null);
  const [mine, setMine] = useState<Mine[]>([]);
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [bin, setBin] = useState<{ path: string; removedAt: string }[]>([]);
  const [sizes, setSizes] = useState<Record<string, string>>({});
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [dealers, setDealers] = useState<Dealer[]>([]);
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [bump, setBump] = useState(0);
  const [queue, setQueue] = useState<File[]>([]);
  const [addTo, setAddTo] = useState('portfolio');
  const [section, setSection] = useState('detailing');
  const [rights, setRights] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [doomed, setDoomed] = useState<Item[] | null>(null);
  const addInput = useRef<HTMLInputElement>(null), replaceInput = useRef<HTMLInputElement>(null);
  const sheet = useRef<HTMLDialogElement>(null), uploader = useRef<HTMLDialogElement>(null);

  const load = useCallback(async () => {
    const [b, m, c] = await Promise.all([fetch('/site-assets', { cache: 'no-store' }).then(readJson), fetch(assets, { cache: 'no-store' }).then(readJson), fetch(root, { cache: 'no-store' }).then(readJson)]);
    setBuilt(b.items ?? []); setMine(m.items ?? []); setHidden(new Set<string>(m.hidden ?? [])); setBin(m.bin ?? []); setSizes(m.logoSizes ?? {});
    setPhotos(c.portfolio ?? []); setDealers(c.dealers ?? []);
  }, [assets, root]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  const photoItem = useCallback((p: Photo): Item => ({ key: 'photo:' + p.id, group: 'portfolio', name: p.title, src: `${root}/media/${p.mediaId}`, kind: 'image', draft: !p.published, photo: p, removedAt: p.binnedAt }), [root]);
  const dealerItem = useCallback((d: Dealer): Item => ({ key: 'dealers/' + d.id, group: 'logos', name: d.name, src: `${root}/media/${d.mediaId}`, kind: 'image', draft: !d.published, dealer: d, removedAt: d.binnedAt }), [root]);
  const fileItem = useCallback((path: string, kind: 'image' | 'video', removedAt?: string): Item => ({ key: path, group: groupOf(path), name: logoNames.get(path) ?? pretty(path.split('/').pop() ?? path), src: `/site/assets/${path}?v=${bump}`, kind, path, removedAt, replaced: mine.some((m) => m.path === path) && !path.startsWith('films/') }), [bump, mine]);

  const items = useMemo(() => {
    const films = mine.filter((m) => m.path.startsWith('films/') && m.contentType.startsWith('video/') && !hidden.has(m.path)).map((m) => fileItem(m.path, 'video'));
    const files = (built ?? []).filter((b) => !hidden.has(b.path)).map((b) => fileItem(b.path, b.kind));
    return [...photos.filter((p) => !p.binnedAt).map(photoItem), ...dealers.filter((d) => !d.binnedAt).map(dealerItem), ...films, ...files];
  }, [built, mine, hidden, photos, dealers, photoItem, dealerItem, fileItem]);
  const binned = useMemo(() => {
    const kindOf = (path: string) => (built ?? []).find((b) => b.path === path)?.kind ?? (path.startsWith('films/') || path.startsWith('transformations/') ? 'video' : 'image');
    return [...photos.filter((p) => p.binnedAt).map(photoItem), ...dealers.filter((d) => d.binnedAt).map(dealerItem), ...bin.map((b) => fileItem(b.path, kindOf(b.path), b.removedAt))];
  }, [built, bin, photos, dealers, photoItem, dealerItem, fileItem]);

  const order = ['logos', 'portfolio', 'films'];
  const groups = [...new Set(items.map((i) => i.group))].sort((a, b) => ((order.indexOf(a) + 1 || 99) - (order.indexOf(b) + 1 || 99)) || groupName(a).localeCompare(groupName(b)));
  const needle = query.trim().toLowerCase();
  const shown = items.filter((i) => (filter === 'all' || i.group === filter) && (!needle || i.name.toLowerCase().includes(needle)));
  const open = items.find((i) => i.key === selected) ?? null;
  const binGroups = [...new Set(binned.map((i) => i.group))];

  useEffect(() => { const d = sheet.current; if (!d) return; if (open && !d.open) d.showModal(); if (!open && d.open) d.close(); }, [open]);
  useEffect(() => { const d = uploader.current; if (!d) return; if (queue.length && !d.open) d.showModal(); if (!queue.length && d.open) d.close(); }, [queue.length]);

  async function act(work: () => Promise<unknown>, said: string) {
    setBusy(true); setError(''); setNotice('');
    try { await work(); setNotice(said); setBump((n) => n + 1); await load(); return true; }
    catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); await load().catch(() => undefined); return false; }
    finally { setBusy(false); }
  }
  const putMedia = async (blob: Blob) => (await readJson(await fetch(`${root}/media`, { method: 'POST', headers: { 'Content-Type': blob.type }, body: blob }))).id as string;
  const savePhoto = (p: Photo, change: Partial<Photo>) => { const n = { ...p, ...change }; return fetch(`${root}/portfolio`, send('PUT', { id: p.id, mediaId: n.mediaId, title: n.title.trim(), alt: n.alt.trim() || n.title.trim(), category: n.category, sortOrder: n.sortOrder, published: n.published, version: p.version, rightsReviewed: n.published })).then(readJson); };
  const saveDealer = (d: Dealer, change: Partial<Dealer>) => { const n = { ...d, ...change }; return fetch(`${root}/dealers`, send('PUT', { id: d.id, mediaId: n.mediaId, name: n.name.trim(), sortOrder: n.sortOrder, published: n.published, version: d.version, rightsReviewed: n.published })).then(readJson); };

  function chooseFiles(files: FileList | File[] | null) {
    const list = Array.from(files ?? []).filter((f) => f.type.startsWith('image/') || f.type.startsWith('video/') || /\.(png|jpe?g|webp|avif|gif|svg)$/i.test(f.name));
    if (addInput.current) addInput.current.value = '';
    if (!list.length) return;
    setAddTo(filter === 'logos' ? 'logos' : 'portfolio'); setRights(false); setError(''); setQueue(list);
  }
  async function addQueued() {
    const files = queue; let done = 0;
    const ok = await act(async () => {
      let photoOrder = photos.reduce((m, p) => Math.max(m, p.sortOrder), -10) + 10, dealerOrder = dealers.reduce((m, d) => Math.max(m, d.sortOrder), -10) + 10;
      for (const file of files) {
        const title = pretty(file.name);
        const name = title.length < 2 ? 'Untitled' : title;
        if (file.type.startsWith('video/')) {
          const ext = (file.name.split('.').pop() ?? 'mp4').toLowerCase().replace(/[^a-z0-9]/g, '') || 'mp4';
          await readJson(await fetch(`${assets}?path=${encodeURIComponent(`films/${crypto.randomUUID().slice(0, 8)}.${ext}`)}`, { method: 'PUT', body: file }));
        } else if (addTo === 'logos') {
          const mediaId = await putMedia(await fitImage(file, { logo: true }));
          await readJson(await fetch(`${root}/dealers`, send('PUT', { mediaId, name, sortOrder: dealerOrder, published: true, rightsReviewed: true })));
          dealerOrder += 10;
        } else {
          const mediaId = await putMedia(await fitImage(file));
          await readJson(await fetch(`${root}/portfolio`, send('PUT', { mediaId, title: name, alt: name, category: section, sortOrder: photoOrder, published: true, rightsReviewed: true })));
          photoOrder += 10;
        }
        done += 1;
      }
    }, `${files.length} added to the website.`);
    if (ok) setQueue([]);
    else if (done) setQueue(files.slice(done));
  }
  function onReplace(files: FileList | null) {
    const file = files?.[0], item = open;
    if (replaceInput.current) replaceInput.current.value = '';
    if (!file || !item) return;
    void act(async () => {
      if (item.photo) await savePhoto(item.photo, { mediaId: await putMedia(await fitImage(file)) });
      else if (item.dealer) await saveDealer(item.dealer, { mediaId: await putMedia(await fitImage(file, { logo: true })) });
      else if (item.kind === 'video') await readJson(await fetch(`${assets}?path=${encodeURIComponent(item.path!)}`, { method: 'PUT', body: file }));
      else {
        const logo = item.group === 'logos' || brandMark(item.path);
        const body = await fitImage(file, logo ? { logo } : { ratio: await shapeOf(item.src) });
        await readJson(await fetch(`${assets}?path=${encodeURIComponent(item.path!)}`, { method: 'PUT', body }));
      }
    }, `${item.name} replaced.`);
  }
  const moveToBin = (i: Item) => { setSelected(''); return act(() => (i.photo || i.dealer ? fetch(`${root}/bin/${i.photo ? 'portfolio' : 'dealers'}/${i.photo?.id ?? i.dealer!.id}`, { method: 'PUT' }) : fetch(`${assets}/hidden?path=${encodeURIComponent(i.path!)}`, { method: 'PUT' })).then(readJson), `${i.name} moved to the Bin.`); };
  const restore = (i: Item) => act(() => (i.photo || i.dealer ? fetch(`${root}/bin/${i.photo ? 'portfolio' : 'dealers'}/${i.photo?.id ?? i.dealer!.id}`, { method: 'DELETE' }) : fetch(`${assets}/hidden?path=${encodeURIComponent(i.path!)}`, { method: 'DELETE' })).then(readJson), `${i.name} is back on the website.`);
  const erase = (i: Item) => fetch(i.photo ? `${root}/portfolio/${i.photo.id}` : i.dealer ? `${root}/dealers/${i.dealer.id}` : `${assets}/bin?path=${encodeURIComponent(i.path!)}`, { method: 'DELETE' }).then(readJson);
  function eraseDoomed() {
    const list = doomed ?? []; setDoomed(null);
    void act(async () => { for (const i of list) await erase(i); }, list.length === 1 ? `${list[0].name} was deleted forever.` : `${list.length} items were deleted forever.`);
  }
  async function setSize(key: string, size: string) {
    setSizes((s) => ({ ...s, [key]: size })); setError('');
    try { await readJson(await fetch(`${assets}/logo-size?logo=${encodeURIComponent(key)}&size=${size}`, { method: 'PUT' })); }
    catch (e) { setError(e instanceof Error ? e.message : 'The size could not be saved.'); await load().catch(() => undefined); }
  }
  function move(item: Item, by: -1 | 1) {
    const ids = item.photo ? photos.filter((p) => !p.binnedAt).map((p) => p.id) : dealers.filter((d) => !d.binnedAt).map((d) => d.id);
    const id = item.photo?.id ?? item.dealer!.id, at = ids.indexOf(id), to = at + by;
    if (to < 0 || to >= ids.length) return;
    [ids[at], ids[to]] = [ids[to], ids[at]];
    void act(() => fetch(`${root}/${item.photo ? 'portfolio-order' : 'dealers-order'}`, send('PUT', { ids })).then(readJson), 'Order saved.');
  }

  const tile = (i: Item) => (i.kind === 'video'
    ? <video src={i.src} muted playsInline preload="metadata" />
    /* eslint-disable-next-line @next/next/no-img-element */
    : <img src={i.src} alt="" loading="lazy" />);
  const logoSize = open ? SIZES.find((s) => s[0] === sizes[open.key]) ?? SIZES[1] : SIZES[1];
  const siblings = open?.photo ? photos.filter((p) => !p.binnedAt).length : open?.dealer ? dealers.filter((d) => !d.binnedAt).length : 0;

  return (
    <section className={styles.mlRoot} aria-label="Media library" data-dragging={dragging || undefined}
      onDragOver={(e) => { if (e.dataTransfer.types.includes('Files')) { e.preventDefault(); setDragging(true); } }}
      onDragLeave={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false); }}
      onDrop={(e) => { e.preventDefault(); setDragging(false); chooseFiles(e.dataTransfer.files); }}>
      <div className={styles.mlBar}>
        <label className={styles.mlSearch}><Search size={16} aria-hidden="true" /><input type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search" aria-label="Search pictures and films" /></label>
        <button type="button" className="primary" disabled={busy} onClick={() => addInput.current?.click()}><Upload size={16} aria-hidden="true" /> {busy ? 'Working…' : 'Upload'}</button>
      </div>
      <input ref={addInput} type="file" accept="image/*,video/mp4,video/webm" multiple hidden onChange={(e) => chooseFiles(e.target.files)} />
      <input ref={replaceInput} type="file" accept={open?.kind === 'video' ? 'video/mp4,video/webm' : 'image/*'} hidden onChange={(e) => onReplace(e.target.files)} />

      <nav className={styles.mlChips} aria-label="Categories">
        <button type="button" aria-pressed={filter === 'all'} onClick={() => setFilter('all')}>All <span>{items.length}</span></button>
        {groups.map((g) => <button key={g} type="button" aria-pressed={filter === g} onClick={() => setFilter(g)}>{groupName(g)} <span>{items.filter((i) => i.group === g).length}</span></button>)}
        <button type="button" className={styles.mlBinChip} aria-pressed={filter === 'bin'} onClick={() => setFilter('bin')}><Trash2 size={14} aria-hidden="true" /> Bin <span>{binned.length}</span></button>
      </nav>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!built && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading pictures" />}

      {filter !== 'bin' && built && (shown.length
        ? <ul className={styles.mlGrid}>
            {shown.map((i) => (
              <li key={i.key}>
                <button type="button" className={styles.mlItem} onClick={() => setSelected(i.key)}>
                  <span className={styles.mlThumb}>{tile(i)}</span>
                  <span className={styles.mlName}>{i.name}</span>
                  <span className={styles.mlMeta}>{groupName(i.group)}{i.draft ? ' · Not on the website' : ''}</span>
                </button>
              </li>
            ))}
          </ul>
        : <p className={styles.status}>{needle ? 'Nothing matches that search.' : 'Nothing here yet. Drop files anywhere on this page to add them.'}</p>)}

      {filter === 'bin' && (binned.length === 0
        ? <p className={styles.status}>The Bin is empty. Anything you remove waits here until you restore it or delete it forever.</p>
        : <>
            <div className={styles.rowBetween}>
              <p className={styles.personSub}>Nothing here is on the website. Items stay until you delete them.</p>
              <button type="button" className="secondary" disabled={busy} onClick={() => setDoomed(binned)}>Empty the Bin</button>
            </div>
            {binGroups.map((g) => (
              <div key={g} className={styles.stack}>
                <h3 className={styles.mlHeading}>{groupName(g)}</h3>
                <ul className={styles.mlGrid}>
                  {binned.filter((i) => i.group === g).map((i) => (
                    <li key={i.key} className={styles.mlBinItem}>
                      <span className={styles.mlThumb}>{tile(i)}</span>
                      <span className={styles.mlName}>{i.name}</span>
                      <div className={styles.logoActions}>
                        <button type="button" className="secondary" disabled={busy} onClick={() => void restore(i)}>Restore</button>
                        <button type="button" className="secondary" disabled={busy} onClick={() => setDoomed([i])}>Delete</button>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </>)}

      <dialog ref={sheet} className={styles.mlSheet} aria-label={open?.name ?? 'Picture details'} onClose={() => setSelected('')}>
        {open && <div className={styles.stack}>
          <div className={styles.rowBetween}><h3 className={styles.mlHeading}>{groupName(open.group)}</h3><button type="button" className={styles.mlClose} aria-label="Close" onClick={() => setSelected('')}><X size={20} aria-hidden="true" /></button></div>
          {open.group === 'logos'
            ? <div className={styles.logoTile}>{/* eslint-disable-next-line @next/next/no-img-element */}<img src={open.src} alt="" style={{ width: logoSize[2] + '%', height: logoSize[3] + '%' }} /></div>
            : <span className={styles.mlPreview}>{open.kind === 'video' ? <video src={open.src} muted playsInline controls preload="metadata" /> : /* eslint-disable-next-line @next/next/no-img-element */ <img src={open.src} alt="" />}</span>}
          {open.group === 'logos' && <div className={styles.sizeRow} role="group" aria-label="Logo size">{SIZES.map((s) => <button key={s[0]} type="button" className={styles.sizeBtn} aria-pressed={s[0] === logoSize[0]} onClick={() => void setSize(open.key, s[0])}>{s[1]}</button>)}</div>}
          {open.photo || open.dealer
            ? <label className="form-field">Name<input key={open.key + open.name} defaultValue={open.name} maxLength={120} disabled={busy} onBlur={(e) => { const v = e.target.value.trim(); if (v.length < 2 || v === open.name) return; void act(() => (open.photo ? savePhoto(open.photo, { title: v, alt: v }) : saveDealer(open.dealer!, { name: v })), 'Name saved.'); }} /></label>
            : <strong className={styles.logoName}>{open.name}</strong>}
          {open.photo && <label className="form-field">Section<Select value={open.photo.category} disabled={busy} onChange={(e) => void act(() => savePhoto(open.photo!, { category: e.target.value }), 'Section saved.')}>{SECTIONS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</Select></label>}
          {open.draft && <button type="button" className="primary" disabled={busy} onClick={() => void act(() => (open.photo ? savePhoto(open.photo, { published: true }) : saveDealer(open.dealer!, { published: true })), `${open.name} is on the website.`)}>We have permission: show on the website</button>}
          <div className={styles.logoActions}>
            <button type="button" className="secondary" disabled={busy} onClick={() => replaceInput.current?.click()}>Replace</button>
            {!brandMark(open.path) && <button type="button" className="secondary" disabled={busy} onClick={() => void moveToBin(open)}><Trash2 size={15} aria-hidden="true" /> Move to Bin</button>}
          </div>
          {open.replaced && <button type="button" className={styles.logoUndo} disabled={busy} onClick={() => void act(() => fetch(`${assets}?path=${encodeURIComponent(open.path!)}`, { method: 'DELETE' }).then(readJson), 'The original is back.')}>Put the original back</button>}
          {siblings > 1 && <div className={styles.logoActions}>
            <button type="button" className="secondary" disabled={busy} onClick={() => move(open, -1)}>Move earlier</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => move(open, 1)}>Move later</button>
          </div>}
        </div>}
      </dialog>

      <dialog ref={uploader} className={styles.mlUpload} aria-label="Add files" onClose={() => setQueue([])}>
        <div className={styles.stack}>
          <h3 className={styles.mlHeading}>Add {queue.length} {queue.length === 1 ? 'file' : 'files'}</h3>
          <p className={styles.mlFiles}>{queue.map((f) => f.name).join(', ')}</p>
          {queue.some((f) => !f.type.startsWith('video/')) && <label className="form-field">Add pictures to<Select value={addTo} disabled={busy} onChange={(e) => setAddTo(e.target.value)}><option value="portfolio">Portfolio</option><option value="logos">Dealership logos</option></Select></label>}
          {addTo === 'portfolio' && queue.some((f) => !f.type.startsWith('video/')) && <label className="form-field">Section<Select value={section} disabled={busy} onChange={(e) => setSection(e.target.value)}>{SECTIONS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</Select></label>}
          <label style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}><input type="checkbox" checked={rights} onChange={(e) => setRights(e.target.checked)} /><span>We have the right to show these publicly.</span></label>
          {error && <p role="alert" className="field-error">{error}</p>}
          <div className={styles.logoActions}>
            <button type="button" className="secondary" disabled={busy} onClick={() => setQueue([])}>Cancel</button>
            <button type="button" className="primary" disabled={busy || !rights} onClick={() => void addQueued()}>{busy ? 'Adding…' : 'Add to website'}</button>
          </div>
        </div>
      </dialog>

      <ConfirmDialog isOpen={doomed !== null} title={doomed && doomed.length > 1 ? 'Empty the Bin' : 'Delete forever'} entityName={doomed && doomed.length > 1 ? `${doomed.length} items` : doomed?.[0]?.name ?? ''} entityType="Media" actionType="delete" isBusy={busy}
        explanation="This cannot be undone. Built-in pictures stay off the website; uploaded files are erased." onConfirm={eraseDoomed} onCancel={() => setDoomed(null)} />
    </section>
  );
}
