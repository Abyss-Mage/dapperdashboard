'use client';
import { ImageIcon } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';

type Photo = { id: string; ordinal: number };
type Post = { id: string; kind: 'dealership_tag' | 'customer_car'; employee: string; dealershipName: string; customerName?: string; inspectionType?: 'pre' | 'post'; serviceName?: string; bookingTime?: string; retentionUntil?: string; notes: string; latitude?: number; longitude?: number; accuracyMeters?: number; createdAt: string; photos: Photo[] };
type View = { viewedAt: string; viewer: string; employee: string; postId: string; photoId: string };
async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Employee uploads are unavailable.'); return b as T; }

/* Photos stay closed until a reviewer opens a post, and every photo opened is logged with the reviewer's identity.
   Self-contained (takes only the location API base) so it can move into any dashboard shell. */
export default function EmployeeUploadsReview({ base, canSeeLog }: { base: string; canSeeLog: boolean }) {
  const root = `${base}/employee-uploads`;
  const [kind, setKind] = useState<'' | 'dealership_tag' | 'customer_car'>(''), [posts, setPosts] = useState<Post[] | null>(null), [error, setError] = useState(''), [opened, setOpened] = useState<Record<string, boolean>>({});
  const [views, setViews] = useState<View[] | null>(null), [showLog, setShowLog] = useState(false);
  const load = useCallback(async () => { setPosts(null); setError(''); try { setPosts((await read<{ items: Post[] }>(await fetch(`${root}${kind ? `?kind=${kind}` : ''}`, { cache: 'no-store' }))).items); } catch (e) { setError((e as Error).message); } }, [root, kind]);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { setOpened({}); setViews(null); setShowLog(false); }, [root]);
  async function loadLog() { setError(''); try { setViews((await read<{ items: View[] }>(await fetch(`${root}/views`, { cache: 'no-store' }))).items); } catch (e) { setError((e as Error).message); } }
  const when = (v: string) => new Date(v).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
  return <section className={glass.stage} aria-labelledby="upload-review-title"><div className={glass.card}>
    <div className={glass.head}><h2 id="upload-review-title" className={glass.title}><ImageIcon size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Employee uploads</h2><span className={glass.pill}>{posts ? `${posts.length} post${posts.length === 1 ? '' : 's'}` : 'Loading'}</span></div>
    <p className={glass.lede}>Dealership key tags and customer-car inspections posted at this location. Photos stay closed until you open a post, and each photo you open is recorded with your name.</p>
    <div className={glass.actions} role="group" aria-label="Filter uploads">{([['', 'All'], ['dealership_tag', 'Dealership tags'], ['customer_car', 'Customer cars']] as const).map(([value, label]) => <button key={value} className={kind === value ? glass.button : glass.ghost} aria-pressed={kind === value} onClick={() => setKind(value)}>{label}</button>)}</div>
    {error && <p className={glass.alert} role="alert">{error}</p>}
    {posts && !posts.length && <p className={glass.lede}>No uploads yet.</p>}
    <ul className={glass.list}>{posts?.map(p => <li key={p.id} className={glass.row} style={{ gridTemplateColumns: '1fr', alignItems: 'start' }}>
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
