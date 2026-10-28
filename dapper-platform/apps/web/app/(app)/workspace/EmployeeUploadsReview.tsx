'use client';
import { ImageIcon } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';
import Select from '../GlassSelect';
import { DatePicker } from '../Pickers';

type Photo = { id: string; ordinal: number };
type Post = { id: string; kind: 'dealership_tag' | 'customer_car'; employee: string; dealershipName: string; customerName?: string; inspectionType?: 'pre' | 'post'; serviceName?: string; bookingTime?: string; retentionUntil?: string; notes: string; latitude?: number; longitude?: number; accuracyMeters?: number; createdAt: string; photos: Photo[] };
type View = { viewedAt: string; viewer: string; employee: string; postId: string; photoId: string };
async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Employee uploads are unavailable.'); return b as T; }

/* Photos stay closed until a reviewer opens a post, and every photo opened is logged with the reviewer's identity.
   Self-contained (takes only the location API base) so it can move into any dashboard shell. */
export default function EmployeeUploadsReview({ base, canSeeLog, fixedKind, title }: { base: string; canSeeLog: boolean; fixedKind?: 'dealership_tag' | 'customer_car'; title?: string }) {
  const root = `${base}/employee-uploads`;
  const [kind, setKind] = useState<'' | 'dealership_tag' | 'customer_car'>(fixedKind ?? ''), [query, setQuery] = useState(''), [who, setWho] = useState(''), [place, setPlace] = useState(''), [from, setFrom] = useState(''), [to, setTo] = useState(''), [posts, setPosts] = useState<Post[] | null>(null), [error, setError] = useState(''), [opened, setOpened] = useState<Record<string, boolean>>({});
  const [views, setViews] = useState<View[] | null>(null), [showLog, setShowLog] = useState(false);
  const load = useCallback(async () => { setPosts(null); setError(''); try { setPosts((await read<{ items: Post[] }>(await fetch(`${root}${kind ? `?kind=${kind}` : ''}`, { cache: 'no-store' }))).items); } catch (e) { setError((e as Error).message); } }, [root, kind]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { setOpened({}); setViews(null); setShowLog(false); }, [root]);
  async function loadLog() { setError(''); try { setViews((await read<{ items: View[] }>(await fetch(`${root}/views`, { cache: 'no-store' }))).items); } catch (e) { setError((e as Error).message); } }
  // Filters on the loaded posts: free text, who posted, the dealership (tags) or pre/post-detail (cars), and a date range.
  const people = [...new Set((posts ?? []).map((p) => p.employee))].sort();
  const places = [...new Set((posts ?? []).map((p) => p.dealershipName).filter(Boolean))].sort();
  const shown = (posts ?? []).filter((p) => {
    const day = p.createdAt.slice(0, 10), q = query.trim().toLowerCase();
    return (!who || p.employee === who) && (!place || (fixedKind === 'customer_car' ? p.inspectionType === place : p.dealershipName === place)) && (!from || day >= from) && (!to || day <= to)
      && (!q || [p.employee, p.dealershipName, p.customerName, p.serviceName, p.notes].some((v) => (v ?? '').toLowerCase().includes(q)));
  });
  const filtered = !!(query || who || place || from || to);
  const when = (v: string) => new Date(v).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
  return <section className={glass.stage} aria-labelledby="upload-review-title"><div className={glass.card}>
    <div className={glass.head}><h2 id="upload-review-title" className={glass.title}><ImageIcon size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />{title ?? 'Employee uploads'}</h2><span className={glass.pill}>{posts ? `${shown.length}${filtered ? ` of ${posts.length}` : ''} post${posts.length === 1 ? '' : 's'}` : 'Loading'}</span></div>
    <p className={glass.lede}>Dealership key tags and customer-car inspections posted at this location. Photos stay closed until you open a post, and each photo you open is recorded with your name.</p>
    {fixedKind && <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'flex-end', margin: '8px 0 14px' }}>
      <label className={glass.field} style={{ flex: '1 1 200px' }}>Search<input type="search" placeholder={fixedKind === 'customer_car' ? 'Customer, service or note' : 'Dealership, employee or note'} value={query} onChange={(e) => setQuery(e.target.value)} /></label>
      <label className={glass.field}>Posted by<Select value={who} onChange={(e) => setWho(e.target.value)}><option value="">Anyone</option>{people.map((n) => <option key={n} value={n}>{n}</option>)}</Select></label>
      {fixedKind === 'dealership_tag' ? <label className={glass.field}>Dealership<Select value={place} onChange={(e) => setPlace(e.target.value)}><option value="">All dealerships</option>{places.map((n) => <option key={n} value={n}>{n}</option>)}</Select></label>
        : <label className={glass.field}>Inspection<Select value={place} onChange={(e) => setPlace(e.target.value)}><option value="">Pre and post</option><option value="pre">Pre-detail</option><option value="post">Post-detail</option></Select></label>}
      <label className={glass.field}>From<DatePicker value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} /></label>
      <label className={glass.field}>To<DatePicker value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} /></label>
      {filtered && <button type="button" className={glass.ghost} onClick={() => { setQuery(''); setWho(''); setPlace(''); setFrom(''); setTo(''); }}>Clear filters</button>}
    </div>}
    {!fixedKind && <div className={glass.actions} role="group" aria-label="Filter uploads">{([['', 'All'], ['dealership_tag', 'Dealership tags'], ['customer_car', 'Customer cars']] as const).map(([value, label]) => <button key={value} className={kind === value ? glass.button : glass.ghost} aria-pressed={kind === value} onClick={() => setKind(value)}>{label}</button>)}</div>}
    {error && <p className={glass.alert} role="alert">{error}</p>}
    {posts && !posts.length && <p className={glass.lede}>No uploads yet.</p>}
    {posts && posts.length > 0 && !shown.length && <p className={glass.lede}>No uploads match these filters.</p>}
    <ul className={glass.list}>{shown.map(p => <li key={p.id} className={glass.row} style={{ gridTemplateColumns: '1fr', alignItems: 'start' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}><span className={glass.name}>{p.kind === 'customer_car' ? `${p.customerName} · ${p.inspectionType === 'pre' ? 'Pre' : 'Post'}-detail` : `Key tags · ${p.dealershipName}`}</span><span className={glass.pill}>{p.photos.length} photo{p.photos.length === 1 ? '' : 's'}</span></div>
      <span className={glass.meta}>{p.employee} · {when(p.createdAt)}{p.kind === 'customer_car' && p.serviceName ? ` · ${p.serviceName}${p.bookingTime ? ` (booking ${when(p.bookingTime)})` : ''}` : ''}</span>
      <span className={glass.meta}>{p.latitude != null && p.longitude != null ? `GPS ${p.latitude.toFixed(5)}, ${p.longitude.toFixed(5)}${p.accuracyMeters != null ? ` ±${Math.round(p.accuracyMeters)} m` : ''}` : 'GPS was not shared for this post'} · {p.retentionUntil ? `deleted automatically ${when(p.retentionUntil)}` : 'kept permanently'}</span>
      {p.notes && <p className={glass.lede}>{p.notes}</p>}
      {opened[p.id] ? <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: 10 }}>{p.photos.map(ph => <a key={ph.id} href={`${root}/photos/${ph.id}`} target="_blank" rel="noreferrer" aria-label={`Open photo ${ph.ordinal} full size (logged)`}><img src={`${root}/photos/${ph.id}`} alt={`Photo ${ph.ordinal}`} style={{ width: '100%', aspectRatio: '4 / 3', objectFit: 'cover', borderRadius: 14, border: '1px solid var(--border)' }} /></a>)}</div>
        : <div className={glass.actions}><button className={glass.ghost} onClick={() => setOpened(o => ({ ...o, [p.id]: true }))}>Open photos (logged)</button></div>}
    </li>)}</ul>
    {canSeeLog && <div><button className={glass.ghost} aria-expanded={showLog} onClick={() => { const next = !showLog; setShowLog(next); if (next && !views) void loadLog(); }}>{showLog ? 'Hide viewing log' : 'Show viewing log'}</button>
      {showLog && (views === null ? <p className={glass.lede}>Loading…</p> : !views.length ? <p className={glass.lede}>No photos have been opened yet.</p> : <ul className={glass.list} style={{ marginTop: 12 }}>{views.map((v, i) => <li key={i} className={glass.row}><span className={glass.name}>{v.viewer}</span><span className={glass.meta} style={{ gridColumn: 'auto' }}>{when(v.viewedAt)}</span><span className={glass.meta}>opened a photo posted by {v.employee}</span></li>)}</ul>)}</div>}
  </div></section>;
}
