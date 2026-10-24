'use client';
import { Camera, Check, ImagePlus, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import styles from './employee.module.css';

type Kind = 'dealership_tag' | 'customer_car';
type Photo = { id: string; ordinal: number; width: number; height: number; sizeBytes: number };
export type UploadPost = { id: string; kind: Kind; dealershipName: string; customerName?: string; inspectionType?: 'pre' | 'post'; bookingId?: string; serviceName?: string; bookingTime?: string; retentionUntil?: string; notes: string; latitude?: number; longitude?: number; accuracyMeters?: number; capturedAt: string; createdAt: string; photos: Photo[] };
type Fix = { latitude: number; longitude: number; accuracy: number };
type Draft = { id: string; blob: Blob; url: string };
const MAX_PHOTOS = 10, photoUrl = (id: string) => `/api/v1/employee/uploads/photos/${id}`;

async function readError(r: Response) { const b = await r.json().catch(() => ({})); return (b.error ?? b.errors?.[0] ?? 'The request could not be completed.') as string; }
function getFix(): Promise<Fix | null> {
  return new Promise(resolve => {
    if (!('geolocation' in navigator)) return resolve(null);
    navigator.geolocation.getCurrentPosition(p => resolve({ latitude: p.coords.latitude, longitude: p.coords.longitude, accuracy: p.coords.accuracy }), () => resolve(null), { enableHighAccuracy: true, timeout: 10_000, maximumAge: 60_000 });
  });
}
/* Burns date/time, employee and GPS into the pixels for convenience, then shrinks to a phone-friendly JPEG.
   The server keeps its own timestamp, identity and digest for every photo, so the audit trail never rests on pixels. */
async function stampPhoto(file: File, lines: string[]): Promise<Blob> {
  const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' }).catch(() => { throw new Error('That photo format is not supported. Use a JPEG or PNG photo.'); });
  const scale = Math.min(1, 2048 / Math.max(bitmap.width, bitmap.height)), w = Math.round(bitmap.width * scale), h = Math.round(bitmap.height * scale);
  const canvas = document.createElement('canvas'); canvas.width = w; canvas.height = h;
  const ctx = canvas.getContext('2d'); if (!ctx) throw new Error('Photos cannot be prepared in this browser.');
  ctx.drawImage(bitmap, 0, 0, w, h); bitmap.close();
  const size = Math.max(14, Math.round(w / 52)), pad = size * 0.6, line = size * 1.35, box = lines.length * line + pad * 2;
  ctx.fillStyle = 'rgba(0,0,0,.58)'; ctx.fillRect(0, h - box, w, box);
  ctx.fillStyle = '#fff'; ctx.font = `600 ${size}px system-ui, sans-serif`; ctx.textBaseline = 'alphabetic';
  lines.forEach((text, i) => ctx.fillText(text, pad, h - box + pad + size + i * line, w - pad * 2));
  for (const quality of [0.82, 0.7, 0.55]) {
    const blob = await new Promise<Blob | null>(res => canvas.toBlob(res, 'image/jpeg', quality));
    if (blob && blob.size <= 5 * 1024 * 1024) return blob;
  }
  throw new Error('That photo is too large even after shrinking.');
}

export type PickableBooking = { id: string; customerName: string; serviceName: string; bookingTime: string };
type DealershipChoice = { id: string; name: string };

export function UploadComposer({ kind, timezone, employee, bookings, onPosted }: { kind: Kind; timezone: string; employee: string; bookings: PickableBooking[]; onPosted: (post: UploadPost) => void }) {
  const isCar = kind === 'customer_car';
  const [drafts, setDrafts] = useState<Draft[]>([]), [bookingId, setBookingId] = useState(''), [dealerships, setDealerships] = useState<DealershipChoice[]>([]), [dealershipsLoaded, setDealershipsLoaded] = useState(false), [dealershipId, setDealershipId] = useState(''), [inspection, setInspection] = useState<'pre' | 'post'>('pre'), [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState(''), [fix, setFix] = useState<Fix | null>(null), [fixAsked, setFixAsked] = useState(false);
  const [recent, setRecent] = useState<UploadPost[] | null>(null);
  const key = useRef(`up-${crypto.randomUUID()}`), captureInput = useRef<HTMLInputElement>(null), addInput = useRef<HTMLInputElement>(null), errorRef = useRef<HTMLParagraphElement>(null);
  const loadRecent = useCallback(() => fetch(`/api/v1/employee/uploads?kind=${kind}`, { cache: 'no-store' }).then(r => r.ok ? r.json() : Promise.reject()).then(d => setRecent(d.items)).catch(() => setRecent([])), [kind]);
  useEffect(() => { loadRecent(); }, [loadRecent]);
  useEffect(() => {
    if (isCar) return; let live = true;
    fetch('/api/v1/employee/uploads/options', { cache: 'no-store' }).then(r => r.ok ? r.json() : Promise.reject()).then(d => { if (!live) return; setDealerships(d.dealerships); setDealershipsLoaded(true); }).catch(() => { if (live) setError('Dealerships could not be loaded. Refresh and try again.'); });
    return () => { live = false; };
  }, [isCar]);
  const chosenDealership = dealerships.find(d => d.id === dealershipId)?.name ?? '';
  const chosenBooking = bookings.find(b => b.id === bookingId);
  const needsChoice = isCar ? !bookingId : !dealershipId;
  const when = (v: string) => new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeStyle: 'short', timeZone: timezone }).format(new Date(v));
  const sortedBookings = [...bookings].sort((x, y) => Math.abs(new Date(x.bookingTime).getTime() - Date.now()) - Math.abs(new Date(y.bookingTime).getTime() - Date.now()));
  useEffect(() => () => drafts.forEach(d => URL.revokeObjectURL(d.url)), []); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (error) errorRef.current?.focus(); }, [error]);

  async function addFiles(list: FileList | null) {
    if (!list?.length) return; setError(''); setNotice('');
    if (needsChoice) { setError(isCar ? 'Choose the booking first so the photos are stamped for it.' : 'Choose the dealership first so the photos are stamped for it.'); return; }
    const room = MAX_PHOTOS - drafts.length; if (room <= 0) { setError('You can add up to 10 photos per post.'); return; }
    setBusy(true);
    try {
      let here = fix; if (!fixAsked) { setFixAsked(true); here = await getFix(); setFix(here); }
      const when = new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeStyle: 'medium', timeZone: timezone }).format(new Date());
      const lines = [`${when} · ${employee}`, `${here ? `GPS ${here.latitude.toFixed(5)}, ${here.longitude.toFixed(5)} ±${Math.round(here.accuracy)} m` : 'GPS unavailable'} · ${isCar ? `${chosenBooking?.customerName ?? ''} · ${chosenBooking?.serviceName ?? ''}` : chosenDealership}`];
      const next: Draft[] = [];
      for (const file of Array.from(list).slice(0, room)) { const blob = await stampPhoto(file, lines); next.push({ id: crypto.randomUUID(), blob, url: URL.createObjectURL(blob) }); }
      setDrafts(current => [...current, ...next]);
      if (list.length > room) setError(`Only ${room} more photo${room === 1 ? '' : 's'} fit in this post; the rest were skipped.`);
    } catch (e) { setError(e instanceof Error ? e.message : 'The photo could not be prepared.'); }
    finally { setBusy(false); if (captureInput.current) captureInput.current.value = ''; if (addInput.current) addInput.current.value = ''; }
  }
  function remove(id: string) { setDrafts(current => { const gone = current.find(d => d.id === id); if (gone) URL.revokeObjectURL(gone.url); return current.filter(d => d.id !== id); }); }
  async function post(e: React.FormEvent) {
    e.preventDefault(); setError(''); setNotice('');
    if (!drafts.length) { setError('Add at least one photo.'); return; }
    if (isCar && !bookingId) { setError('Choose the booking this inspection belongs to.'); return; }
    if (!isCar && !dealershipId) { setError('Choose a dealership.'); return; }
    setBusy(true);
    try {
      const form = new FormData();
      form.set('kind', kind); form.set('requestKey', key.current); form.set('capturedAt', new Date().toISOString()); form.set('notes', notes.trim());
      if (isCar) { form.set('bookingId', bookingId); form.set('inspectionType', inspection); } else form.set('dealershipId', dealershipId);
      if (fix) { form.set('latitude', String(fix.latitude)); form.set('longitude', String(fix.longitude)); form.set('accuracyMeters', String(fix.accuracy)); }
      drafts.forEach((d, i) => form.append('photos', d.blob, `photo-${i + 1}.jpg`));
      const r = await fetch('/api/v1/employee/uploads', { method: 'POST', body: form });
      if (!r.ok) throw new Error(await readError(r));
      const saved = await r.json() as UploadPost;
      drafts.forEach(d => URL.revokeObjectURL(d.url)); setDrafts([]); setBookingId(''); setNotes(''); setFix(null); setFixAsked(false); key.current = `up-${crypto.randomUUID()}`;
      setNotice(isCar ? 'Inspection photos posted.' : 'Key tags posted.'); onPosted(saved); void loadRecent();
    } catch (err) { setError(err instanceof Error ? err.message : 'The post could not be saved. Check your connection and try again; your photos are kept.'); }
    finally { setBusy(false); }
  }
  const title = isCar ? 'Upload inspection photos' : 'Post key tags';
  return <div className={styles.uploadGrid}>
    <section className={styles.uploadCard} aria-labelledby="upload-title">
      <div className={styles.uploadHead}><span className={styles.uploadIcon}><Camera size={20} aria-hidden="true" /></span><div><h2 id="upload-title">{title}</h2><p className={styles.help}>{isCar ? 'Linked to the booking. Multiple photos, stamped with date, time and your name.' : 'Add one or many photos. Date, time, GPS and your name are stamped on every image.'}</p></div></div>
      <form onSubmit={post} noValidate>
        {error && <p ref={errorRef} tabIndex={-1} role="alert" className={styles.error}>{error}</p>}{notice && <p role="status" className={styles.notice}><Check size={18} aria-hidden="true" />{notice}</p>}
        {isCar ? <div className={styles.fieldPair}>
          <label>Booking<select value={bookingId} onChange={e => setBookingId(e.target.value)} disabled={busy || !sortedBookings.length} required><option value="">{sortedBookings.length ? 'Choose a booking' : 'No bookings available'}</option>{sortedBookings.map(b => <option key={b.id} value={b.id}>{b.customerName} · {b.serviceName} · {when(b.bookingTime)}</option>)}</select>{!sortedBookings.length && <span className={styles.help}>Bookings linked to your franchise appear here. Ask your administrator if one is missing.</span>}</label>
          <fieldset><legend>Inspection type</legend><div className={styles.segment} role="group" aria-label="Inspection type">{(['pre', 'post'] as const).map(v => <button type="button" key={v} aria-pressed={inspection === v} onClick={() => setInspection(v)} disabled={busy}>{v === 'pre' ? 'Pre-detail' : 'Post-detail'}</button>)}</div></fieldset>
        </div> : <label>Dealership<select value={dealershipId} onChange={e => setDealershipId(e.target.value)} disabled={busy || !dealerships.length} required><option value="">{dealershipsLoaded ? 'Select a dealership' : 'Loading dealerships…'}</option>{dealerships.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}</select><span className={styles.help}>{dealershipsLoaded && !dealerships.length ? 'No partner dealerships have been added yet. An administrator adds them in Master under B2B Fleet Accounts.' : 'Partner dealerships (car dealers we work with) are added by an administrator. They are not your location.'}</span></label>}
        {isCar && <label>Notes <span className={styles.optional}>(optional)</span><textarea rows={3} maxLength={2000} value={notes} onChange={e => setNotes(e.target.value)} placeholder="Any visible damage, special instructions…" disabled={busy} /></label>}
        <div><span className={styles.eyebrow}>Photos · {drafts.length}/{MAX_PHOTOS}</span>{needsChoice && <span className={styles.help} style={{ display: 'block', marginTop: 6 }}>{isCar ? 'Choose a booking to start adding photos.' : 'Choose a dealership to start adding photos.'}</span>}
          <div className={styles.photoGrid}>
            {drafts.map((d, i) => <div className={styles.photoTile} key={d.id}><img src={d.url} alt={`Photo ${i + 1} of ${drafts.length}`} /><button type="button" className={styles.tileRemove} aria-label={`Remove photo ${i + 1}`} onClick={() => remove(d.id)} disabled={busy}><X size={16} aria-hidden="true" /></button></div>)}
            {drafts.length < MAX_PHOTOS && <>
              <button type="button" className={styles.addTile} onClick={() => captureInput.current?.click()} disabled={busy || needsChoice}><Camera size={20} aria-hidden="true" />Capture</button>
              <button type="button" className={styles.addTile} onClick={() => addInput.current?.click()} disabled={busy || needsChoice}><ImagePlus size={20} aria-hidden="true" />Add</button>
            </>}
          </div>
          <input ref={captureInput} type="file" accept="image/*" capture="environment" hidden onChange={e => void addFiles(e.target.files)} />
          <input ref={addInput} type="file" accept="image/*" multiple hidden onChange={e => void addFiles(e.target.files)} />
        </div>
        <p className={styles.help}>{fixAsked ? (fix ? 'Your location is stamped on these photos and saved with the post.' : 'Location was not shared, so photos say “GPS unavailable”. You can still post.') : 'When you add the first photo your browser asks to share your location for the stamp. You can decline and still post.'}</p>
        <div><button className={styles.primary} disabled={busy || !drafts.length}><Check size={18} aria-hidden="true" />{busy ? 'Working…' : 'Post'}</button></div>
        <p className={styles.help}>Each photo is stamped and its original upload time, your identity and a checksum are recorded by the server for audit. {isCar ? 'Inspection photos are kept permanently.' : 'Key tags are deleted automatically 30 days after posting.'}</p>
      </form>
    </section>
    <aside className={styles.recentCard} aria-labelledby="recent-title"><div className={styles.sectionTop}><h2 id="recent-title">{isCar ? 'Your recent uploads' : 'Your recent tags'}</h2><span className={styles.count}>{recent?.length ?? 0}</span></div>
      {recent === null ? <p className={styles.help}>Loading…</p> : !recent.length ? <p className={styles.emptyNote}>Nothing yet.</p> : <ul className={styles.recentList}>{recent.slice(0, 6).map(p => <li key={p.id}><strong>{p.kind === 'customer_car' ? `${p.customerName} · ${p.inspectionType === 'pre' ? 'Pre' : 'Post'}-detail` : p.dealershipName}</strong><span>{new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeStyle: 'short', timeZone: timezone }).format(new Date(p.createdAt))} · {p.photos.length} photo{p.photos.length === 1 ? '' : 's'}</span></li>)}</ul>}
    </aside>
  </div>;
}

export function MyUploads({ timezone }: { timezone: string }) {
  const [posts, setPosts] = useState<UploadPost[] | null>(null), [error, setError] = useState(''), [view, setView] = useState<Kind>('dealership_tag');
  useEffect(() => { let live = true; fetch('/api/v1/employee/uploads', { cache: 'no-store' }).then(async r => { if (!r.ok) throw new Error(await readError(r)); return r.json(); }).then(d => { if (live) setPosts(d.items); }).catch(e => { if (live) setError(e.message); }); return () => { live = false; }; }, []);
  const tags = posts?.filter(p => p.kind === 'dealership_tag') ?? [], cars = posts?.filter(p => p.kind === 'customer_car') ?? [], shown = view === 'dealership_tag' ? tags : cars;
  const fmt = (v: string) => new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeStyle: 'short', timeZone: timezone }).format(new Date(v));
  return <section className={styles.uploadsPage} aria-labelledby="uploads-title"><h2 id="uploads-title" className={styles.srOnly}>My uploads</h2>
    <div className={styles.segment} role="group" aria-label="Upload type"><button type="button" aria-pressed={view === 'dealership_tag'} onClick={() => setView('dealership_tag')}>Dealership tags ({tags.length})</button><button type="button" aria-pressed={view === 'customer_car'} onClick={() => setView('customer_car')}>Customer cars ({cars.length})</button></div>
    {error && <p role="alert" className={styles.error}>{error}</p>}
    {posts === null && !error ? <p className={styles.help}>Loading your uploads…</p> : !shown.length ? <div className={styles.uploadCard}><p className={styles.emptyNote}>No uploads yet.</p></div> :
      <ul className={styles.postList}>{shown.map(p => <li key={p.id} className={styles.postCard}>
        <div className={styles.postTop}><div><h3>{p.kind === 'customer_car' ? p.customerName : p.dealershipName}</h3><p className={styles.help}>{fmt(p.createdAt)}{p.kind === 'customer_car' && ` · ${p.inspectionType === 'pre' ? 'Pre' : 'Post'}-detail · ${p.serviceName ?? ''}${p.bookingTime ? ' · booking ' + fmt(p.bookingTime) : ''}`}{p.retentionUntil ? ` · deleted automatically ${fmt(p.retentionUntil)}` : ' · kept permanently'}</p></div><span className={styles.status}>{p.photos.length} photo{p.photos.length === 1 ? '' : 's'}</span></div>
        {p.notes && <p>{p.notes}</p>}
        <p className={styles.help}>{p.latitude != null && p.longitude != null ? `GPS ${p.latitude.toFixed(5)}, ${p.longitude.toFixed(5)}${p.accuracyMeters != null ? ` ±${Math.round(p.accuracyMeters)} m` : ''}` : 'GPS was not shared for this post.'}</p>
        <div className={styles.photoGrid}>{p.photos.map(ph => <a key={ph.id} className={styles.photoTile} href={photoUrl(ph.id)} target="_blank" rel="noreferrer" aria-label={`Open photo ${ph.ordinal} of ${p.photos.length}`}><img loading="lazy" src={photoUrl(ph.id)} alt={`Photo ${ph.ordinal}`} /></a>)}</div>
      </li>)}</ul>}
  </section>;
}

type Break = { start: string; end?: string; minutes: number };
export function BreakHistory({ timezone }: { timezone: string }) {
  const [items, setItems] = useState<Break[] | null>(null), [error, setError] = useState('');
  useEffect(() => { let live = true; fetch('/api/v1/employee/breaks', { cache: 'no-store' }).then(async r => { if (!r.ok) throw new Error(await readError(r)); return r.json(); }).then(d => { if (live) setItems(d.items); }).catch(e => { if (live) setError(e.message); }); return () => { live = false; }; }, []);
  const day = (v: string) => new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeZone: timezone }).format(new Date(v)), time = (v: string) => new Intl.DateTimeFormat('en-CA', { timeStyle: 'short', timeZone: timezone }).format(new Date(v));
  return <section className={styles.punches} aria-labelledby="breaks-title"><div className={styles.sectionTop}><h2 id="breaks-title">Recorded breaks</h2><span className={styles.count}>{items?.length ?? 0} shown</span></div>
    {error && <p role="alert" className={styles.error}>{error}</p>}
    {items === null && !error ? <p className={styles.help}>Loading…</p> : items && !items.length ? <p className={styles.emptyNote}>No breaks recorded yet.</p> : items && <div className={styles.tableScroll}><table className={styles.table}><thead><tr><th>Day</th><th>Started</th><th>Ended</th><th>Length</th></tr></thead><tbody>
      {items.map((b, i) => <tr key={i}><td>{day(b.start)}</td><td>{time(b.start)}</td><td>{b.end ? time(b.end) : <span className={styles.activePill}>In progress</span>}</td><td>{b.end ? `${b.minutes} min` : '—'}</td></tr>)}
    </tbody></table></div>}
  </section>;
}
