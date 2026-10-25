'use client';
import { MapPin } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import glass from '../glass.module.css';
import NumberField from '../NumberField';

type Settings = { enabled: boolean; sampleIntervalSeconds: number; retentionDays: number; version: number };
type Entry = { attendanceId: string; employee: string; shiftStatus: string; state: string; stale: boolean; lastSampleAt?: string; latitude?: number; longitude?: number; accuracyMeters?: number };
type Live = { settings: Settings; entries: Entry[] };
const json = { 'Content-Type': 'application/json' };
async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Location settings are unavailable.'); return b as T; }

const stateLabel: Record<string, string> = { active: 'Sharing', none: 'Not started', permission_denied: 'Permission denied', permission_lost: 'Permission lost', unsupported: 'Unsupported device', stopped: 'Stopped by employee' };

export default function GpsPanel({ base, canManage }: { base: string; canManage: boolean }) {
  const [live, setLive] = useState<Live | null>(null), [form, setForm] = useState<Settings | null>(null), [error, setError] = useState(''), [notice, setNotice] = useState(''), [busy, setBusy] = useState(false);
  const load = useCallback(async (resetForm: boolean) => {
    const data = await read<Live>(await fetch(`${base}/gps/live`, { cache: 'no-store' }));
    setLive(data); if (resetForm) setForm(data.settings);
  }, [base]);
  useEffect(() => {
    let alive = true; setLive(null); setForm(null); setError(''); setNotice('');
    load(true).catch(e => { if (alive) setError(e.message); });
    const id = window.setInterval(() => load(false).catch(() => undefined), 60_000);
    return () => { alive = false; window.clearInterval(id); };
  }, [load]);

  async function save(e: React.FormEvent) {
    e.preventDefault(); if (!form) return; setBusy(true); setError(''); setNotice('');
    try {
      const next = await read<Settings>(await fetch(`${base}/gps/settings`, { method: 'PUT', headers: json, body: JSON.stringify({ enabled: form.enabled, sampleIntervalSeconds: form.sampleIntervalSeconds, retentionDays: form.retentionDays, version: form.version }) }));
      setForm(next); await load(false);
      setNotice(next.enabled ? 'Saved. Employees can now choose to share location while clocked in.' : 'Saved. Location collection is off.');
    } catch (err) { setError((err as Error).message); } finally { setBusy(false); }
  }
  const minutes = form ? Math.round(form.sampleIntervalSeconds / 60) : 5;
  const time = (iso?: string) => iso ? new Date(iso).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' }) : 'no update yet';

  return <section className={glass.stage} aria-labelledby="gps-admin-title"><div className={glass.card}>
    <div className={glass.head}><h2 id="gps-admin-title" className={glass.title}><MapPin size={18} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />On-shift location</h2>
      <span className={`${glass.pill} ${live?.settings.enabled ? glass.pillOk : ''}`}>{live ? (live.settings.enabled ? 'Collection on' : 'Collection off') : 'Loading'}</span></div>
    <p className={glass.lede}>Samples are accepted only while an employee is clocked in and not on break, after they choose to share. People with attendance access at this location can view them. Nothing is collected off shift.</p>
    {error && <p className={glass.alert} role="alert">{error}</p>}{notice && <p className={glass.notice} role="status">{notice}</p>}
    {form && (canManage ? <form onSubmit={save} style={{ display: 'grid', gap: 14 }}>
      <label className={glass.switch}><span>Allow on-shift location at this location</span><input type="checkbox" role="switch" checked={form.enabled} disabled={busy} onChange={e => setForm({ ...form, enabled: e.target.checked })} /></label>
      <div className={glass.fields}>
        <label className={glass.field}>Sample every (seconds)<NumberField min={60} max={3600} step={30} required disabled={busy} value={form.sampleIntervalSeconds} onChange={e => setForm({ ...form, sampleIntervalSeconds: Number(e.target.value) })} /></label>
        <label className={glass.field}>Keep for (days)<NumberField min={1} max={365} required disabled={busy} value={form.retentionDays} onChange={e => setForm({ ...form, retentionDays: Number(e.target.value) })} /></label>
      </div>
      <p className={glass.lede}>Currently about every {minutes} minute{minutes === 1 ? '' : 's'}, deleted after {form.retentionDays} days. Starting values are 5 minutes and 30 days.</p>
      <div className={glass.actions}><button className={glass.button} disabled={busy}>{busy ? 'Saving…' : 'Save location settings'}</button></div>
    </form> : <p className={glass.notice}>Only people who can manage attendance can change these settings.</p>)}
    <h3 className={glass.title}>Open shifts</h3>
    {live && !live.entries.length && <p className={glass.lede}>No one is clocked in at this location.</p>}
    <ul className={glass.list}>{live?.entries.map(e => {
      const onBreak = e.shiftStatus === 'on_break', bad = e.state === 'permission_denied' || e.state === 'permission_lost' || e.state === 'unsupported';
      const tone = !live.settings.enabled || onBreak ? '' : bad ? glass.pillBad : e.stale || e.state === 'stopped' || e.state === 'none' ? glass.pillWarn : glass.pillOk;
      const label = !live.settings.enabled ? 'Collection off' : onBreak ? 'Paused on break' : e.stale && e.state === 'active' ? 'No recent update' : stateLabel[e.state] ?? e.state;
      return <li key={e.attendanceId} className={glass.row}><span className={glass.name}>{e.employee}</span><span className={`${glass.pill} ${tone}`}>{label}</span>
        <span className={glass.meta}>{e.latitude != null && e.longitude != null ? <>Last seen {time(e.lastSampleAt)} · {e.latitude.toFixed(4)}, {e.longitude.toFixed(4)}{e.accuracyMeters != null ? ` · ±${Math.round(e.accuracyMeters)} m` : ''}</> : onBreak ? 'Location hidden during break' : `Last update: ${time(e.lastSampleAt)}`}</span></li>;
    })}</ul>
  </div></section>;
}
