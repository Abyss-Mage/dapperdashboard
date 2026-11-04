'use client';
import { MapPin } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import glass from '../glass.module.css';

type Status = { onShift: boolean; shiftStatus?: string; settings: { enabled: boolean; sampleIntervalSeconds: number; retentionDays: number }; state?: string; lastSampleAt?: string; trackingActive: boolean };
type Fix = { latitude: number; longitude: number; accuracy: number; at: number };
type Mode = 'idle' | 'starting' | 'sharing' | 'denied' | 'unsupported';

const json = { 'Content-Type': 'application/json' };

/* Device sharing is explicit (a button press) and only runs in this open page. Browsers can pause
   location when the screen locks, so supervisors see a "stale" state rather than a promise of
   background tracking. Break, clock-out, disabled settings and permission loss all stop sampling. */
// It is mounted once for the whole workspace so sharing keeps running on every tab; its card is drawn into the #gps-slot the home
// tab's shift panel leaves for it (slotKey changes whenever the visible tab does).
export default function GpsTracker({ shiftStatus, slotKey }: { shiftStatus?: 'clocked_in' | 'on_break' | 'clocked_out'; slotKey?: string }) {
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  useEffect(() => { setSlot(document.getElementById('gps-slot')); }, [slotKey]);
  const [status, setStatus] = useState<Status | null>(null), [mode, setMode] = useState<Mode>('idle'), [error, setError] = useState(''), [lastSent, setLastSent] = useState<Date | null>(null);
  const watch = useRef<number | null>(null), fix = useRef<Fix | null>(null), timer = useRef<number | null>(null), first = useRef(false), autoTried = useRef(false);

  const report = useCallback((state: string) => { fetch('/api/v1/employee/gps/state', { method: 'POST', headers: json, body: JSON.stringify({ state }) }).catch(() => undefined); }, []);
  const halt = useCallback(() => {
    if (watch.current !== null) navigator.geolocation.clearWatch(watch.current);
    if (timer.current !== null) window.clearInterval(timer.current);
    watch.current = timer.current = null; fix.current = null; first.current = false;
  }, []);
  const load = useCallback(async () => {
    try { const r = await fetch('/api/v1/employee/gps', { cache: 'no-store' }); if (!r.ok) throw new Error(); setStatus(await r.json()); setError(''); }
    catch { setError('Location status is unavailable right now.'); }
  }, []);

  useEffect(() => { load(); }, [load, shiftStatus]);
  const active = !!status?.trackingActive && shiftStatus === 'clocked_in';
  const interval = status?.settings.sampleIntervalSeconds ?? 300;

  const send = useCallback(async () => {
    const f = fix.current; if (!f || Date.now() - f.at > interval * 1000 + 60_000) return;
    try {
      const r = await fetch('/api/v1/employee/gps/samples', { method: 'POST', headers: json, body: JSON.stringify({ latitude: f.latitude, longitude: f.longitude, accuracyMeters: f.accuracy, capturedAt: new Date(f.at).toISOString() }) });
      if (r.status === 201) { setLastSent(new Date()); setError(''); }
      else if (r.status === 409) { halt(); setMode('idle'); load(); }
      else if (r.status !== 429) setError('A location update could not be saved. It will retry at the next interval.');
    } catch { setError('Offline — location updates resume when you reconnect.'); }
  }, [halt, interval, load]);

  // Stop everything the moment the shift is no longer actively tracked (break, clock-out, disabled).
  useEffect(() => { if (!active && mode !== 'idle' && mode !== 'denied' && mode !== 'unsupported') { halt(); setMode('idle'); } }, [active, mode, halt]);
  useEffect(() => halt, [halt]);
  // Clocking in asks for browser location permission once (the browser prompt is the consent step).
  // Sharing is required while clocked in: there is no way to stop it. A permission already denied is reported, not re-prompted.
  useEffect(() => {
    if (!active) { autoTried.current = false; return; }
    if (mode !== 'idle' || autoTried.current) return;
    autoTried.current = true;
    let live = true;
    (async () => {
      try {
        const p = await navigator.permissions?.query({ name: 'geolocation' as PermissionName });
        if (live && p?.state === 'denied') { setMode('denied'); report('permission_denied'); return; }
      } catch { /* permission query unsupported: fall through to the browser prompt */ }
      if (live) start();
    })();
    return () => { live = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, mode, shiftStatus]);

  function start() {
    if (!('geolocation' in navigator)) { setMode('unsupported'); report('unsupported'); return; }
    setMode('starting'); setError('');
    watch.current = navigator.geolocation.watchPosition(p => {
      fix.current = { latitude: p.coords.latitude, longitude: p.coords.longitude, accuracy: p.coords.accuracy, at: p.timestamp };
      if (!first.current) { first.current = true; report('active'); void send(); timer.current = window.setInterval(() => void send(), interval * 1000); }
      setMode('sharing');
    }, e => {
      const hadFix = first.current; halt();
      if (e.code === e.PERMISSION_DENIED) { setMode('denied'); report(hadFix ? 'permission_lost' : 'permission_denied'); }
      else { autoTried.current = false; setMode('idle'); setError('Your device could not find a location yet. Retrying — move to a clearer signal if this continues.'); }
    }, { enableHighAccuracy: false, maximumAge: 30_000, timeout: 30_000 });
  }

  const minutes = Math.round(interval / 60), days = status?.settings.retentionDays ?? 30;
  let pill = <span className={glass.pill}>Not sharing</span>, body: React.ReactNode;
  if (!status) body = <p className={glass.lede}>Checking location settings…</p>;
  else if (!status.onShift || shiftStatus === 'clocked_out') body = <p className={glass.lede}>Location is only shared while you are clocked in. Nothing is recorded off shift.</p>;
  else if (!status.settings.enabled) body = <p className={glass.lede}>On-shift location is not turned on for this worksite, so you will not be asked to share it. Nothing is being recorded. An administrator can turn it on in Master under On-shift location.</p>;
  else if (shiftStatus === 'on_break') { pill = <span className={`${glass.pill} ${glass.pillWarn}`}>Paused on break</span>; body = <p className={glass.lede}>Location is paused during your break and resumes when you end it.</p>; }
  else {
    if (mode === 'sharing') pill = <span className={`${glass.pill} ${glass.pillOk}`}>Sharing</span>;
    if (mode === 'denied') pill = <span className={`${glass.pill} ${glass.pillBad}`}>Permission off</span>;
    body = <>
      <p className={glass.lede}>Sharing is required while you are clocked in. It pauses on breaks. This device sends your location about every {minutes} minute{minutes === 1 ? '' : 's'}. People with attendance access at this worksite can see it, and it is deleted after {days} days.</p>
      {mode === 'denied' && <p className={`${glass.alert}`} role="alert">Sharing your location is required while you are clocked in, and permission is off. Allow location for this site in your browser settings, then press Try again. Your supervisor can see that sharing is off.</p>}
      {mode === 'unsupported' && <p className={`${glass.alert}`} role="alert">This browser cannot share location. Use a supported phone browser.</p>}
      {mode === 'sharing' && <p className={glass.notice} role="status">{lastSent ? `Last update sent ${lastSent.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}.` : 'Finding your location…'} Keep this page open — browsers can pause location when the screen is locked.</p>}
      {mode === 'sharing' || mode === 'starting' ? null : <div className={glass.actions}><button className={glass.button} onClick={() => { autoTried.current = true; start(); }}>{mode === 'denied' || mode === 'unsupported' ? 'Try again' : 'Share location now'}</button></div>}
    </>;
  }
  if (!slot) return null;
  return createPortal(<section className={`${glass.stage} ${glass.flatStage}`} aria-labelledby="gps-title"><div className={`${glass.card} ${glass.flatCard}`}>
    <div className={glass.head}><h3 id="gps-title" className={glass.title}><MapPin size={17} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Location on shift</h3>{pill}</div>
    {body}{error && <p className={glass.alert} role="alert">{error}</p>}
  </div></section>, slot);
}
