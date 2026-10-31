'use client';
import { ImageIcon } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import glass from '../glass.module.css';
import Select from '../GlassSelect';
import { DatePicker } from '../Pickers';

type Photo = { id: string; ordinal: number };
type Post = { id: string; kind: 'dealership_tag' | 'customer_car'; employee: string; dealershipName: string; customerName?: string; inspectionType?: 'pre' | 'post'; serviceName?: string; bookingTime?: string; retentionUntil?: string; notes: string; latitude?: number; longitude?: number; accuracyMeters?: number; createdAt: string; photos: Photo[] };
type View = { viewedAt: string; viewer: string; employee: string; postId: string; photoId: string };
async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Employee uploads are unavailable.'); return b as T; }

/* Customer-car posts stay closed until a reviewer opens them; dealership tags show as a gallery of small previews (not logged). Opening a photo full size is logged with the reviewer identity.
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
  // Dealership tags show as a gallery of small previews. Previews are not logged; opening a photo shows it full size with
  // its details and writes the viewing log entry, because that is when the full photo is fetched.
  const [viewing, setViewing] = useState<{ post: Post; photo: Photo } | null>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = dialogRef.current;
    if (!d) return;
    if (viewing && !d.open) d.showModal();
    if (!viewing && d.open) d.close();
  }, [viewing]);
  const gallery = fixedKind === 'dealership_tag';
  const tiles = shown.flatMap((p) => p.photos.map((ph) => ({ post: p, photo: ph })));
  const when = (v: string) => new Date(v).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
  return <section className={glass.stage} aria-labelledby="upload-review-title"><div className={glass.card}>
    <div className={glass.head}><h2 id="upload-review-title" className={glass.title}><ImageIcon size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />{title ?? 'Employee uploads'}</h2><span className={glass.pill}>{posts ? `${shown.length}${filtered ? ` of ${posts.length}` : ''} post${posts.length === 1 ? '' : 's'}` : 'Loading'}</span></div>
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
    {gallery && tiles.length > 0 && <ul aria-label="Photos" style={{ listStyle: 'none', margin: '12px 0', padding: 0, display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))', gap: 10 }}>{tiles.map(({ post, photo }) => <li key={photo.id}>
      <button type="button" onClick={() => setViewing({ post, photo })} aria-label={`Open photo ${photo.ordinal} from ${post.dealershipName}, posted ${when(post.createdAt)}`} style={{ display: 'block', width: '100%', padding: 0, border: '1px solid var(--border)', borderRadius: 14, overflow: 'hidden', background: 'transparent', cursor: 'pointer' }}>
        <img src={`${root}/photos/${photo.id}/thumbnail`} alt="" loading="lazy" style={{ display: 'block', width: '100%', aspectRatio: '4 / 3', objectFit: 'cover' }} />
      </button>
    </li>)}</ul>}
    <dialog ref={dialogRef} onClose={() => setViewing(null)} onClick={(e) => { if (e.target === dialogRef.current) setViewing(null); }} aria-label="Photo details" style={{ maxWidth: 'min(920px, 94vw)', width: '100%', padding: 0, border: '1px solid var(--border)', borderRadius: 18, background: 'var(--surface, #fff)', color: 'var(--text, inherit)' }}>
      {viewing && <div style={{ display: 'grid', gap: 12, padding: 16 }}>
        <img src={`${root}/photos/${viewing.photo.id}`} alt={`Photo ${viewing.photo.ordinal} of ${viewing.post.photos.length}`} style={{ width: '100%', maxHeight: '68vh', objectFit: 'contain', borderRadius: 12, background: '#000' }} />
        <div style={{ display: 'grid', gap: 4 }}>
          <strong>{viewing.post.dealershipName}</strong>
          <span className={glass.meta}>{viewing.post.employee} · {when(viewing.post.createdAt)}</span>
          <span className={glass.meta}>{viewing.post.latitude != null && viewing.post.longitude != null ? `GPS ${viewing.post.latitude.toFixed(5)}, ${viewing.post.longitude.toFixed(5)}${viewing.post.accuracyMeters != null ? ` ±${Math.round(viewing.post.accuracyMeters)} m` : ''}` : 'GPS was not shared for this post'} · {viewing.post.retentionUntil ? `deleted automatically ${when(viewing.post.retentionUntil)}` : 'kept permanently'}</span>
          {viewing.post.notes && <p className={glass.lede} style={{ margin: 0 }}>{viewing.post.notes}</p>}
          <span className={glass.meta}>Opening this photo was recorded in the viewing log.</span>
        </div>
        {viewing.post.photos.length > 1 && <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>{viewing.post.photos.map((ph) => <button key={ph.id} type="button" aria-label={`Show photo ${ph.ordinal}`} aria-current={ph.id === viewing.photo.id} onClick={() => setViewing({ post: viewing.post, photo: ph })} style={{ width: 72, padding: 0, border: ph.id === viewing.photo.id ? '2px solid var(--brand, #fb1a2b)' : '1px solid var(--border)', borderRadius: 10, overflow: 'hidden', background: 'transparent', cursor: 'pointer' }}><img src={`${root}/photos/${ph.id}/thumbnail`} alt="" style={{ display: 'block', width: '100%', aspectRatio: '4 / 3', objectFit: 'cover' }} /></button>)}</div>}
        <div className={glass.actions}><button type="button" className={glass.button} onClick={() => setViewing(null)}>Close</button></div>
      </div>}
    </dialog>
    {!gallery && <ul className={glass.list}>{shown.map(p => <li key={p.id} className={glass.row} style={{ gridTemplateColumns: '1fr', alignItems: 'start' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}><span className={glass.name}>{p.kind === 'customer_car' ? `${p.customerName} · ${p.inspectionType === 'pre' ? 'Pre' : 'Post'}-detail` : `Key tags · ${p.dealershipName}`}</span><span className={glass.pill}>{p.photos.length} photo{p.photos.length === 1 ? '' : 's'}</span></div>
      <span className={glass.meta}>{p.employee} · {when(p.createdAt)}{p.kind === 'customer_car' && p.serviceName ? ` · ${p.serviceName}${p.bookingTime ? ` (booking ${when(p.bookingTime)})` : ''}` : ''}</span>
      <span className={glass.meta}>{p.latitude != null && p.longitude != null ? `GPS ${p.latitude.toFixed(5)}, ${p.longitude.toFixed(5)}${p.accuracyMeters != null ? ` ±${Math.round(p.accuracyMeters)} m` : ''}` : 'GPS was not shared for this post'} · {p.retentionUntil ? `deleted automatically ${when(p.retentionUntil)}` : 'kept permanently'}</span>
      {p.notes && <p className={glass.lede}>{p.notes}</p>}
      {opened[p.id] ? <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: 10 }}>{p.photos.map(ph => <a key={ph.id} href={`${root}/photos/${ph.id}`} target="_blank" rel="noreferrer" aria-label={`Open photo ${ph.ordinal} full size (logged)`}><img src={`${root}/photos/${ph.id}`} alt={`Photo ${ph.ordinal}`} style={{ width: '100%', aspectRatio: '4 / 3', objectFit: 'cover', borderRadius: 14, border: '1px solid var(--border)' }} /></a>)}</div>
        : <div className={glass.actions}><button className={glass.ghost} onClick={() => setOpened(o => ({ ...o, [p.id]: true }))}>Open photos (logged)</button></div>}
    </li>)}</ul>}
    {canSeeLog && <div><button className={glass.ghost} aria-expanded={showLog} onClick={() => { const next = !showLog; setShowLog(next); if (next && !views) void loadLog(); }}>{showLog ? 'Hide viewing log' : 'Show viewing log'}</button>
      {showLog && (views === null ? <p className={glass.lede}>Loading…</p> : !views.length ? <p className={glass.lede}>No photos have been opened yet.</p> : <ul className={glass.list} style={{ marginTop: 12 }}>{views.map((v, i) => <li key={i} className={glass.row}><span className={glass.name}>{v.viewer}</span><span className={glass.meta} style={{ gridColumn: 'auto' }}>{when(v.viewedAt)}</span><span className={glass.meta}>opened a photo posted by {v.employee}</span></li>)}</ul>)}</div>}
  </div></section>;
}
