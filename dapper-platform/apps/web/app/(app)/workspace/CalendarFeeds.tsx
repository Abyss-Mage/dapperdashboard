'use client';

import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';

type Feed = { id: string; label: string; createdAt: string; locationName: string };
type Made = { url: string; webcalUrl: string };

/* Private subscribe links for Apple Calendar, Google Calendar and Outlook. The secret is in the address, so it is shown
   once, right after it is made; a link can be revoked at any time and a new one made. */
export default function CalendarFeeds({ endpoint, title, hint }: { endpoint: string; title: string; hint: string }) {
  const [feeds, setFeeds] = useState<Feed[] | null>(null);
  const [made, setMade] = useState<Made | null>(null);
  const [label, setLabel] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    const r = await fetch(endpoint, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Calendar links are unavailable.');
    setFeeds(b.items ?? []);
  }, [endpoint]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function create() {
    setBusy(true); setError(''); setCopied(false);
    try {
      const r = await fetch(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ label }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? b.errors?.label ?? 'The link could not be made.');
      setMade({ url: b.url, webcalUrl: b.webcalUrl }); setLabel('');
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The link could not be made.'); }
    finally { setBusy(false); }
  }
  async function revoke(id: string) {
    setBusy(true); setError('');
    try {
      const r = await fetch(`${endpoint}/${id}`, { method: 'DELETE' });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The link could not be turned off.');
      setMade(null);
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : 'The link could not be turned off.'); }
    finally { setBusy(false); }
  }

  return (
    <section className={glass.card} style={{ padding: 20, display: 'grid', gap: 12 }} aria-label={title}>
      <h3 style={{ margin: 0 }}>{title}</h3>
      <p className={glass.lede}>{hint}</p>
      {error && <p role="alert" className="field-error">{error}</p>}
      {made && (
        <div style={{ display: 'grid', gap: 8 }}>
          <p role="status"><strong>Your link is ready. Copy it now; for safety it is not shown again.</strong></p>
          <input readOnly value={made.url} aria-label="Calendar link" onFocus={(e) => e.currentTarget.select()} />
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
            <button type="button" className="primary" onClick={() => { void navigator.clipboard?.writeText(made.url).then(() => setCopied(true)); }}>{copied ? 'Copied' : 'Copy link'}</button>
            <a className="secondary" href={made.webcalUrl} style={{ padding: '10px 16px', borderRadius: 12 }}>Open in Calendar</a>
          </div>
          <p className={glass.lede}>Apple Calendar: File &gt; New Calendar Subscription, paste the link. Google Calendar: Other calendars &gt; From URL. The link asks calendars to refresh every 5 minutes, but Google decides its own pace (often several hours). Apple Calendar on an iPhone or Mac will only subscribe once the website has a proper domain with a trusted certificate; it refuses the temporary address certificate.</p>
        </div>
      )}
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'end' }}>
        <label className={glass.field} style={{ flex: '1 1 220px' }}>Name (optional)<input value={label} maxLength={80} onChange={(e) => setLabel(e.target.value)} placeholder="e.g. Front desk phone" disabled={busy} /></label>
        <button type="button" className="primary" disabled={busy} onClick={() => void create()}>{busy ? 'Working…' : 'Make a new link'}</button>
      </div>
      {feeds && feeds.length > 0 && (
        <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: 8 }}>
          {feeds.map((f) => (
            <li key={f.id} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
              <span>{f.label || 'Calendar link'} · made {new Date(f.createdAt).toLocaleDateString('en-CA', { year: 'numeric', month: 'short', day: 'numeric' })}</span>
              <button type="button" className="secondary" disabled={busy} onClick={() => void revoke(f.id)}>Turn off</button>
            </li>
          ))}
        </ul>
      )}
      {feeds && feeds.length === 0 && <p className={glass.lede}>No active links yet.</p>}
    </section>
  );
}
