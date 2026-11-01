'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import NumberField from '../NumberField';

type Resource = { id: string; name: string };
type Rule = { id: string; serviceId: string; addonIds: string[]; durationMinutes: number; beforeMinutes: number; afterMinutes: number; units: number; slots: number; workingTime?: boolean; version: number };
type Svc = { id: string; name: string; durationMinutes?: number | null };

/* Slot settings for one service at one franchise: which calendar (resource) it is booked on, how long it holds the slot,
   the clean-up time before and after, and how many can run at once. This is the service's own scheduling rule
   (the one with no add-ons); a calendar's weekly open hours decide which days and times it can be booked. */
export default function ServiceSlots({ base, service, onClose }: { base: string; service: Svc; onClose: () => void }) {
  const [resources, setResources] = useState<Resource[] | null>(null);
  const [resource, setResource] = useState('');
  const [rule, setRule] = useState<Rule | null>(null);
  const [duration, setDuration] = useState(service.durationMinutes ? String(service.durationMinutes) : '');
  const [before, setBefore] = useState('0');
  const [after, setAfter] = useState('0');
  const [units, setUnits] = useState('1');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  useEffect(() => {
    let live = true;
    fetch(`${base}/scheduling/resources`, { cache: 'no-store' })
      .then(async (r) => { if (!r.ok) throw new Error('Calendars could not be loaded.'); return (await r.json()).items as Resource[]; })
      .then((items) => { if (live) { setResources(items); setResource(items[0]?.id ?? ''); } })
      .catch((e) => live && setError(e.message));
    return () => { live = false; };
  }, [base]);

  const loadRule = useCallback(async () => {
    if (!resource) { setRule(null); return; }
    const r = await fetch(`${base}/scheduling/resources/${resource}/rules`, { cache: 'no-store' });
    if (!r.ok) throw new Error('Slot settings could not be loaded.');
    const found = ((await r.json()).items as Rule[]).find((x) => x.serviceId === service.id && x.addonIds.length === 0) ?? null;
    setRule(found);
    if (found) { setDuration(String(found.durationMinutes)); setBefore(String(found.beforeMinutes)); setAfter(String(found.afterMinutes)); setUnits(String(found.slots || 1)); }
  }, [base, resource, service.id]);
  useEffect(() => { setError(''); setNotice(''); loadRule().catch((e) => setError(e.message)); }, [loadRule]);

  const valid = Number(duration) >= 1 && Number(duration) <= 10080 && Number(before) >= 0 && Number(after) >= 0 && Number(units) >= 1;
  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(`${base}/scheduling/resources/${resource}/rules${rule ? '/' + rule.id : ''}`, {
        method: rule ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ serviceId: service.id, addonIds: [], durationMinutes: Math.round(Number(duration)), beforeMinutes: Math.round(Number(before)), afterMinutes: Math.round(Number(after)), units: rule ? rule.units : 1, slots: Math.round(Number(units)), workingTime: rule?.workingTime ?? false, ...(rule ? { version: rule.version } : {}) }),
      });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The slot settings could not be saved.'));
      await loadRule();
      setNotice('Slot settings saved.');
    } catch (e2) { setError(e2 instanceof Error ? e2.message : 'The slot settings could not be saved.'); }
    finally { setBusy(false); }
  }

  return (
    <form onSubmit={save} className={styles.resource}>
      <h3>Slots for “{service.name}”</h3>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {resources && resources.length === 0 && <p className={styles.status}>This franchise has no calendar yet. Add one under Scheduling first.</p>}
      {resources && resources.length > 0 && (
        <>
          <div className={styles.slotGrid}>
            <label className="form-field">Calendar
              <Select value={resource} onChange={(e) => setResource(e.target.value)} disabled={busy}>
                {resources.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
            </label>
            <label className="form-field">Slot length (minutes)
              <NumberField inputMode="numeric" min="1" max="10080" step="5" value={duration} onChange={(e) => setDuration(e.target.value)} disabled={busy} />
            </label>
            <label className="form-field">Gap before (minutes)
              <NumberField inputMode="numeric" min="0" step="5" value={before} onChange={(e) => setBefore(e.target.value)} disabled={busy} />
            </label>
            <label className="form-field">Gap after (minutes)
              <NumberField inputMode="numeric" min="0" step="5" value={after} onChange={(e) => setAfter(e.target.value)} disabled={busy} />
            </label>
            <label className="form-field">Slots (bookings of this service at the same time)
              <NumberField inputMode="numeric" min="1" max="1000" step="1" value={units} onChange={(e) => setUnits(e.target.value)} disabled={busy} />
            </label>
          </div>
          <p className={styles.footnote}>Slots belong to this service alone: other services do not use them up. The days and hours this service can be booked come from the calendar’s weekly hours. To give a service its own days and hours, put it on its own calendar.</p>
        </>
      )}
      <div className={styles.rowBetween}>
        {resources && resources.length > 0 ? <button className="primary" disabled={busy || !valid || !resource}>{busy ? 'Saving…' : rule ? 'Save slot settings' : 'Add slot settings'}</button> : <span />}
        <button type="button" className="secondary" disabled={busy} onClick={onClose}>Close</button>
      </div>
    </form>
  );
}
