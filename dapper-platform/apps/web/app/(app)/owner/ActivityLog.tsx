'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import { DatePicker } from '../Pickers';

type Entry = { id: number; at: string; person: string; email: string; kind: 'sign_in' | 'sign_in_failed' | 'write'; what: string; method: string; path: string; status: number; ip: string; userAgent: string };

// A short device description from the browser's own text: "Chrome on Windows".
function device(ua: string) {
  const browser = /Edg\//.test(ua) ? 'Edge' : /OPR\/|Opera/.test(ua) ? 'Opera' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : ua ? 'Browser' : '';
  const os = /Windows/.test(ua) ? 'Windows' : /iPhone|iPad/.test(ua) ? 'iOS' : /Android/.test(ua) ? 'Android' : /Mac OS X/.test(ua) ? 'macOS' : /Linux/.test(ua) ? 'Linux' : '';
  return [browser, os].filter(Boolean).join(' on ') || 'Unknown device';
}
const when = (iso: string) => new Date(iso).toLocaleString('en-CA', { dateStyle: 'medium', timeStyle: 'medium' });

/* Who signed in (or failed to), and every change a person made: when, from which address, on which device. */
export default function ActivityLog({ organizationId }: { organizationId: string }) {
  const [kind, setKind] = useState('');
  const [q, setQ] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [items, setItems] = useState<Entry[] | null>(null);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    const query = new URLSearchParams({ limit: '300' });
    if (kind) query.set('kind', kind);
    if (q.trim()) query.set('q', q.trim());
    if (from) query.set('from', from);
    if (to) query.set('to', to);
    const r = await fetch(`/api/v1/organizations/${organizationId}/activity-log?${query}`, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'The activity log is unavailable or access has changed.');
    setItems(b.items ?? []); setError('');
  }, [organizationId, kind, q, from, to]);
  useEffect(() => { const id = setTimeout(() => { load().catch((e) => { setItems(null); setError(e.message); }); }, 250); return () => clearTimeout(id); }, [load]);

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Activity log</h2>
        <button type="button" className="secondary" onClick={() => void load().catch((e) => setError(e.message))}>Refresh</button>
      </div>
      <div className={styles.custFilters}>
        <input type="search" className={styles.searchBox} placeholder="Person, email, address or action" aria-label="Search the activity log" value={q} onChange={(e) => setQ(e.target.value)} />
        <Select value={kind} onChange={(e) => setKind(e.target.value)} aria-label="Type"><option value="">Everything</option><option value="sign_in">Sign-ins</option><option value="sign_in_failed">Failed sign-ins</option><option value="write">Changes made</option></Select>
        <DatePicker value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} aria-label="From date" />
        <DatePicker value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} aria-label="To date" />
        {(q || kind || from || to) && <button type="button" className="secondary" onClick={() => { setQ(''); setKind(''); setFrom(''); setTo(''); }}>Clear filters</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading activity" />}
      {items && items.length === 0 && <p className={styles.status}>Nothing recorded for these filters yet. Sign-ins and changes are logged from now on.</p>}
      {items && items.length > 0 && (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead><tr style={{ textAlign: 'left', color: 'var(--text-muted)', fontSize: 12, textTransform: 'uppercase', letterSpacing: '.05em' }}><th style={{ padding: '8px 10px' }}>When</th><th>Who</th><th>What</th><th>From</th><th>Device</th><th>Result</th></tr></thead>
            <tbody>
              {items.map((e) => (
                <tr key={e.id} style={{ borderTop: '1px solid var(--glass-line)' }}>
                  <td style={{ padding: '9px 10px', whiteSpace: 'nowrap' }}>{when(e.at)}</td>
                  <td><strong>{e.person || e.email || 'Unknown'}</strong>{e.person && e.email ? <><br /><span className={styles.personSub}>{e.email}</span></> : null}</td>
                  <td>{e.what}{e.kind === 'write' && <><br /><span className={styles.personSub}>{e.method} {e.path.replace(/^\/api\/v1\//, '')}</span></>}</td>
                  <td style={{ whiteSpace: 'nowrap' }}>{e.ip || '—'}</td>
                  <td>{device(e.userAgent)}</td>
                  <td>{e.kind === 'sign_in_failed' ? 'Refused' : e.status >= 400 ? `Failed (${e.status})` : 'OK'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
