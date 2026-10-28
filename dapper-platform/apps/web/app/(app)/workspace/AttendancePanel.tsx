'use client';
import {getClockInFix} from './clockLocation';
import {useCallback,useEffect,useRef,useState} from 'react';

type AttendanceRecord = {
  id: string;
  status: 'clocked_in' | 'on_break' | 'clocked_out';
  clockInAt: string;
  clockOutAt?: string;
  currentBreakStart?: string;
  totalBreakMinutes: number;
  totalWorkMinutes: number;
  notes: string;
  createdAt: string;
};

export default function AttendancePanel({base, canRecord}: {base: string; canRecord: boolean}) {
  const [active, setActive] = useState<AttendanceRecord | null>(null);
  const [history, setHistory] = useState<AttendanceRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const reqIdRef = useRef(0);

  const load = useCallback(async () => {
    const reqId = ++reqIdRef.current;
    if (!base) {
      setActive(null);
      setHistory([]);
      setLoading(false);
      setError('');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const [activeRes, historyRes] = await Promise.all([
        fetch(base + '/attendance/active'),
        fetch(base + '/attendance/history'),
      ]);
      if (reqId !== reqIdRef.current) return;
      if (activeRes.status === 403 || historyRes.status === 403) {
        setActive(null);
        setHistory([]);
        setError('Access needed (attendance.read or attendance.record)');
        return;
      }
      if (!activeRes.ok || !historyRes.ok) {
        setActive(null);
        setHistory([]);
        setError('Could not load attendance records for this location.');
        return;
      }
      const activeBody = await activeRes.json();
      const historyBody = await historyRes.json();
      if (reqId !== reqIdRef.current) return;
      setActive(activeBody.item ?? null);
      setHistory(historyBody.items ?? []);
      setError('');
    } catch (err) {
      if (reqId !== reqIdRef.current) return;
      setActive(null);
      setHistory([]);
      setError(err instanceof Error ? err.message : 'Could not load attendance records. Refresh to retry.');
    } finally {
      if (reqId === reqIdRef.current) {
        setLoading(false);
      }
    }
  }, [base]);

  useEffect(() => {
    load();
  }, [load]);

  async function performAction(endpoint: string, actionName: string) {
    setBusy(true);
    const location = endpoint === 'clock-in' ? await getClockInFix() : null;
    setError('');
    setNotice('');
    try {
      const r = await fetch(`${base}/attendance/${endpoint}`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(location ? {notes, location} : {notes}),
      });
      const body = await r.json();
      if (!r.ok) {
        setError(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? `Could not ${actionName}`);
        return;
      }
      setNotes('');
      setNotice(`Successfully ${actionName}.`);
      await load();
    } catch {
      setError(`Could not complete ${actionName}. Please check network and retry.`);
    } finally {
      setBusy(false);
    }
  }

  function formatTime(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleTimeString('en-CA', {hour: '2-digit', minute: '2-digit'});
  }

  function formatDate(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleDateString('en-CA', {month: 'short', day: 'numeric', year: 'numeric'});
  }

  return (
    <section className="panel detail-card">
      <div className="section-head">
        <div>
          <h2>Time & attendance</h2>
          <p>
            Track daily work shifts, meal and rest breaks, and recorded time in this location. All time punches
            are immutably logged with actor audit records.
          </p>
        </div>
        <button type="button" className="secondary" disabled={busy} onClick={load}>
          Refresh status
        </button>
      </div>

      {loading && <p style={{color: 'var(--text-muted, var(--text-muted, #6b5558))', margin: '0.75rem 0'}}>Loading attendance records…</p>}
      {error && !loading && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}

      <div className="attendance-status-box" style={{marginBottom: '1.25rem', padding: '1rem', border: '1px solid var(--border)', borderRadius: '8px', background: 'var(--panel-subtle)'}}>
        <div style={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '0.75rem'}}>
          <div>
            <span style={{fontSize: '0.85rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-secondary, var(--text-muted, #6b5558))'}}>Current shift status</span>
            <div style={{display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '0.25rem'}}>
              <span
                className={`status ${
                  !active || active.status === 'clocked_out'
                    ? 'quote'
                    : active.status === 'on_break'
                    ? 'warning'
                    : 'progress'
                }`}
              >
                {!active || active.status === 'clocked_out'
                  ? 'Clocked Out'
                  : active.status === 'on_break'
                  ? 'On Break'
                  : 'Clocked In'}
              </span>
              {active && active.status !== 'clocked_out' && (
                <small>
                  Started at {formatTime(active.clockInAt)} · {active.totalWorkMinutes} min worked
                  {active.totalBreakMinutes > 0 && ` · ${active.totalBreakMinutes} min break`}
                </small>
              )}
            </div>
          </div>
        </div>

        {canRecord && (
          <div style={{marginTop: '1rem'}}>
            <label className="form-field" style={{marginBottom: '0.75rem'}}>
              Shift notes / punch comment (optional)
              <input
                type="text"
                disabled={busy}
                maxLength={1000}
                placeholder="e.g. Bay 2 maintenance, tire rotation support"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </label>
            <div style={{display: 'flex', gap: '0.75rem', flexWrap: 'wrap'}}>
              {(!active || active.status === 'clocked_out') && (
                <button
                  className="primary"
                  disabled={busy}
                  onClick={() => performAction('clock-in', 'clocked in')}
                >
                  {busy ? 'Recording…' : 'Clock in'}
                </button>
              )}
              {active && active.status === 'clocked_in' && (
                <>
                  <button
                    className="secondary"
                    disabled={busy}
                    onClick={() => performAction('start-break', 'started break')}
                  >
                    Start break
                  </button>
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() => performAction('clock-out', 'clocked out')}
                  >
                    Clock out
                  </button>
                </>
              )}
              {active && active.status === 'on_break' && (
                <>
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() => performAction('end-break', 'ended break')}
                  >
                    End break
                  </button>
                  <button
                    className="secondary"
                    disabled={busy}
                    onClick={() => performAction('clock-out', 'clocked out')}
                  >
                    Clock out
                  </button>
                </>
              )}
            </div>
          </div>
        )}
      </div>

      <h3>Shift history</h3>
      <div className="management-list">
        {history.map((record) => (
          <article key={record.id}>
            <div>
              <strong>
                {formatDate(record.clockInAt)} · {formatTime(record.clockInAt)} – {formatTime(record.clockOutAt)}
              </strong>
              <small>
                Total work: {record.totalWorkMinutes} min ({Math.floor(record.totalWorkMinutes / 60)}h{' '}
                {record.totalWorkMinutes % 60}m)
                {record.totalBreakMinutes > 0 ? ` · Break: ${record.totalBreakMinutes} min` : ''} · Status:{' '}
                {record.status}
              </small>
              {record.notes && <small>Notes: {record.notes}</small>}
            </div>
          </article>
        ))}
        {!history.length && <p>No recorded attendance shifts in this location.</p>}
      </div>
    </section>
  );
}
