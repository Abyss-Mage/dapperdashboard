'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import { fitImage } from './fitImage';
import { partners } from '../../(site)/dapper';

type Dealer = { id: string; mediaId: string; name: string; sortOrder: number; published: boolean; version: number };
type Logo = { key: string; name: string; src: string; off: boolean; path?: string; replaced?: boolean; dealer?: Dealer };

// The share of the website's logo card each size fills: [width %, height %]. The same numbers as dealers.css, so the preview is what visitors see.
const SIZES = [['s', 'Small', 55, 44], ['m', 'Medium', 78, 62], ['l', 'Large', 90, 76], ['xl', 'Full', 100, 100]] as const;

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

/* Every dealership logo on the website in one list: the ones the site came with and the ones added here. Each card shows the logo the way the
   website's card does, with its size, Replace and Remove. */
export default function WebsiteDealers({ base }: { base: string }) {
  const root = `${base}/content`, assets = `${base}/site-assets`;
  const [dealers, setDealers] = useState<Dealer[] | null>(null);
  const [replaced, setReplaced] = useState<Set<string>>(new Set());
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [sizes, setSizes] = useState<Record<string, string>>({});
  const [name, setName] = useState('');
  const [rights, setRights] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [target, setTarget] = useState<Dealer | null>(null);
  const [bump, setBump] = useState(0);
  const pick = useRef<HTMLInputElement>(null);
  const replacing = useRef<Logo | null>(null);

  const load = useCallback(async () => {
    const [content, mine] = await Promise.all([fetch(root, { cache: 'no-store' }).then(readJson), fetch(assets, { cache: 'no-store' }).then(readJson)]);
    setDealers(content.dealers ?? []);
    setReplaced(new Set((mine.items ?? []).map((m: { path: string }) => m.path)));
    setHidden(new Set(mine.hidden ?? []));
    setSizes(mine.logoSizes ?? {});
  }, [root, assets]);
  useEffect(() => { setDealers(null); setError(''); setNotice(''); load().catch((e) => setError(e.message)); }, [load]);

  async function act(work: () => Promise<unknown>, said: string) {
    setBusy(true); setError(''); setNotice('');
    try { await work(); setNotice(said); setBump((n) => n + 1); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); await load().catch(() => undefined); }
    finally { setBusy(false); }
  }
  const upload = async (file: File) => {
    const logo = await fitImage(file, { logo: true });
    return (await readJson(await fetch(`${root}/media`, { method: 'POST', headers: { 'Content-Type': logo.type }, body: logo }))).id as string;
  };
  const saveDealer = (d: Dealer, change: Partial<Dealer>) => {
    const next = { ...d, ...change };
    return fetch(`${root}/dealers`, send('PUT', { id: d.id, mediaId: next.mediaId, name: next.name.trim(), sortOrder: next.sortOrder, published: next.published, version: d.version, rightsReviewed: next.published })).then(readJson);
  };

  function chooseAdd() {
    if (name.trim().length < 2) { setError('Type the dealership name first.'); return; }
    if (!rights) { setError('Tick the box to confirm you may show this logo.'); return; }
    replacing.current = null; pick.current?.click();
  }
  function onFile(files: FileList | null) {
    const file = files?.[0];
    if (pick.current) pick.current.value = '';
    if (!file) return;
    const logo = replacing.current;
    if (logo?.path) { const path = logo.path; void act(async () => { const body = await fitImage(file, { logo: true }); await readJson(await fetch(`${assets}?path=${encodeURIComponent(path)}`, { method: 'PUT', body })); }, `${logo.name} replaced.`); return; }
    if (logo?.dealer) { const d = logo.dealer; void act(async () => { await saveDealer(d, { mediaId: await upload(file) }); }, `${logo.name} replaced.`); return; }
    const clean = name.trim();
    void act(async () => {
      const mediaId = await upload(file);
      const order = (dealers ?? []).reduce((m, d) => Math.max(m, d.sortOrder), -10) + 10;
      await readJson(await fetch(`${root}/dealers`, send('PUT', { mediaId, name: clean, sortOrder: order, published: true, rightsReviewed: true })));
      setName('');
    }, `${clean} added to the website.`);
  }
  async function setSize(logo: Logo, size: string) {
    setSizes((s) => ({ ...s, [logo.key]: size })); setError(''); setNotice('');
    try { await readJson(await fetch(`${assets}/logo-size?logo=${encodeURIComponent(logo.key)}&size=${size}`, { method: 'PUT' })); }
    catch (e) { setError(e instanceof Error ? e.message : 'The size could not be saved.'); await load().catch(() => undefined); }
  }
  const setOff = (path: string, off: boolean) => act(() => fetch(`${assets}/hidden?path=${encodeURIComponent(path)}`, { method: off ? 'PUT' : 'DELETE' }).then(readJson), off ? 'Taken off the website.' : 'Back on the website.');
  function move(index: number, by: -1 | 1) {
    if (!dealers) return;
    const next = [...dealers]; const to = index + by;
    if (to < 0 || to >= next.length) return;
    [next[index], next[to]] = [next[to], next[index]];
    void act(() => fetch(`${root}/dealers-order`, send('PUT', { ids: next.map((d) => d.id) })).then(readJson), 'Order saved.');
  }
  function remove() {
    if (!target) return;
    const d = target; setTarget(null);
    void act(() => fetch(`${root}/dealers/${d.id}`, { method: 'DELETE' }).then(readJson), `${d.name} was removed.`);
  }

  if (!dealers) return error ? <p role="alert" className="field-error">{error}</p> : null;
  const logos: Logo[] = [
    ...partners.filter((p) => p.src).map((p) => { const path = p.src!.replace('/site/assets/', ''); return { key: path, name: p.name, src: `${p.src}?v=${bump}`, off: hidden.has(path), path, replaced: replaced.has(path) }; }),
    ...dealers.map((d) => ({ key: 'dealers/' + d.id, name: d.name, src: `${root}/media/${d.mediaId}`, off: !d.published, dealer: d })),
  ];
  return (
    <section className={styles.stack} aria-label="Dealership logos">
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.resource}>
        <div className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
          <label className="form-field" style={{ maxWidth: 320 }}>Add a dealership
            <input value={name} maxLength={120} disabled={busy} onChange={(e) => setName(e.target.value)} placeholder="Dealership name" />
          </label>
          <button type="button" className="primary" disabled={busy} onClick={chooseAdd}>{busy ? 'Working…' : 'Choose logo'}</button>
        </div>
        <label style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginTop: 12 }}>
          <input type="checkbox" checked={rights} onChange={(e) => setRights(e.target.checked)} />
          <span>We have permission to show this dealership&apos;s name and logo.</span>
        </label>
      </div>
      <input ref={pick} type="file" accept="image/*" hidden onChange={(e) => onFile(e.target.files)} />
      <ul className={styles.logoGrid}>
        {logos.map((l) => {
          const size = SIZES.find((s) => s[0] === sizes[l.key]) ?? SIZES[1];
          const index = l.dealer ? dealers.indexOf(l.dealer) : -1;
          return (
            <li key={l.key} className={styles.logoCard} data-off={l.off || undefined}>
              <div className={styles.logoTile}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={l.src} alt={l.name + ' logo'} loading="lazy" style={{ width: size[2] + '%', height: size[3] + '%' }} />
              </div>
              {l.dealer
                ? <input aria-label="Dealership name" className={styles.logoName} defaultValue={l.name} maxLength={120} disabled={busy} onBlur={(e) => { const v = e.target.value.trim(); if (v.length >= 2 && v !== l.name) void act(() => saveDealer(l.dealer!, { name: v }), 'Name saved.'); }} />
                : <strong className={styles.logoName}>{l.name}</strong>}
              <div className={styles.sizeRow} role="group" aria-label={`Size of the ${l.name} logo`}>
                {SIZES.map((s) => <button key={s[0]} type="button" className={styles.sizeBtn} aria-pressed={s[0] === size[0]} disabled={busy || l.off} onClick={() => void setSize(l, s[0])}>{s[1]}</button>)}
              </div>
              {l.off
                ? <button type="button" className="secondary" disabled={busy} onClick={() => { if (l.path) void setOff(l.path, false); else if (!rights) setError('Tick the permission box above first.'); else void act(() => saveDealer(l.dealer!, { published: true }), `${l.name} is on the website.`); }}>Show again</button>
                : <div className={styles.logoActions}>
                    <button type="button" className="secondary" disabled={busy} onClick={() => { replacing.current = l; pick.current?.click(); }}>Replace</button>
                    <button type="button" className="secondary" disabled={busy} onClick={() => { if (l.path) void setOff(l.path, true); else setTarget(l.dealer!); }}>Remove</button>
                  </div>}
              {l.replaced && !l.off && <button type="button" className={styles.logoUndo} disabled={busy} onClick={() => void act(() => fetch(`${assets}?path=${encodeURIComponent(l.path!)}`, { method: 'DELETE' }).then(readJson), 'The original is back.')}>Undo replace</button>}
              {index >= 0 && dealers.length > 1 && <div className={styles.logoActions}>
                <button type="button" className="secondary" disabled={busy || index === 0} onClick={() => move(index, -1)} aria-label={`Move ${l.name} earlier`}>←</button>
                <button type="button" className="secondary" disabled={busy || index === dealers.length - 1} onClick={() => move(index, 1)} aria-label={`Move ${l.name} later`}>→</button>
              </div>}
            </li>
          );
        })}
      </ul>
      <ConfirmDialog isOpen={target !== null} title="Remove dealership" entityName={target?.name ?? ''} entityType="Dealership" actionType="delete" isBusy={busy}
        explanation="The dealership is taken off the website and its logo is deleted. This cannot be undone." onConfirm={remove} onCancel={() => setTarget(null)} />
    </section>
  );
}
