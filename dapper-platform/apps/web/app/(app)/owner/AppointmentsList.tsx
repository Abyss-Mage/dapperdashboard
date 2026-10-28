'use client';

import { useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import RangeBar, { resolveRange, type Custom, type RangePresetId } from './DateRange';
import Select from '../GlassSelect';
import Modal from '../Modal';
import CustomerBookingRequestList from '../account/CustomerBookingRequestList';

/* Appointments: every appointment for the franchise, whether it was booked on the website, by phone or by a staff member.
   Online requests that still need an answer sit on top. Click a row for the details. */

type Booking = { id: string; customerName: string; customerEmail: string; customerPhone: string; serviceName: string; bookingTime: string; status: string; channel: string; addons: { name: string }[] };

export default function AppointmentsList({ base, timezone }: { base: string; timezone: string }) {
  const [preset, setPreset] = useState<RangePresetId>('next30');
  const [custom, setCustom] = useState<Custom>({ from: '', to: '' });
  const [status, setStatus] = useState('');
  const [channel, setChannel] = useState('');
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<Booking[] | null>(null);
  const [error, setError] = useState('');
  const [open, setOpen] = useState<Booking | null>(null);
  const range = resolveRange(preset, custom);

  useEffect(() => {
    if (!range.from || !range.to) return;
    let live = true;
    setItems(null); setError('');
    const start = `${range.from}T00:00:00Z`, end = new Date(Date.parse(`${range.to}T00:00:00Z`) + 2 * 86400000).toISOString();
    fetch(`${base}/bookings?from=${encodeURIComponent(start)}&to=${encodeURIComponent(end)}&limit=1000`, { cache: 'no-store' })
      .then(async (r) => { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Appointments could not be loaded.'); return b; })
      .then((b) => live && setItems(b.items ?? []))
      .catch((e) => live && setError(e.message));
    return () => { live = false; };
  }, [base, range.from, range.to]);

  const fmt = (iso: string) => new Intl.DateTimeFormat('en-CA', { timeZone: timezone, weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }).format(new Date(iso));
  const statuses = useMemo(() => [...new Set((items ?? []).map((b) => b.status))].sort(), [items]);
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (items ?? []).filter((b) => (!status || b.status === status) && (!channel || b.channel === channel) && (!q || [b.customerName, b.customerEmail, b.customerPhone, b.serviceName].some((v) => (v ?? '').toLowerCase().includes(q)))).sort((a, b) => a.bookingTime.localeCompare(b.bookingTime));
  }, [items, status, channel, query]);

  return (
    <div className={styles.stack}>
      <CustomerBookingRequestList endpoint={`${base}/customer-booking-requests`} staff />
      <section className={styles.card} style={{ padding: 20 }}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>All appointments</h2>
        <p className={styles.personSub}>Online, phone and walk-in appointments in one list. Times are in {timezone}.</p>
        <div className={styles.stack} style={{ gap: 10, marginTop: 12 }}>
          <RangeBar presets={['next7', 'next30', 'last7', 'last30', 'month']} preset={preset} custom={custom} onPreset={setPreset} onCustom={setCustom} />
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
            <label className={styles.filterSelect}><span>Status</span><Select value={status} onChange={(e) => setStatus(e.target.value)}><option value="">Any</option>{statuses.map((s) => <option key={s} value={s}>{s.replaceAll('_', ' ')}</option>)}</Select></label>
            <label className={styles.filterSelect}><span>Booked</span><Select value={channel} onChange={(e) => setChannel(e.target.value)}><option value="">Any way</option><option value="online">Online</option><option value="walk_in">Walk-in / phone</option></Select></label>
            <input type="search" className={styles.searchBox} placeholder="Search customer or service" aria-label="Search appointments" value={query} onChange={(e) => setQuery(e.target.value)} />
          </div>
        </div>
        {error && <p role="alert" className="field-error">{error}</p>}
        {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading appointments" style={{ marginTop: 12 }} />}
        {items && shown.length === 0 && <p className={styles.status} style={{ marginTop: 12 }}>No appointments match.</p>}
        {items && shown.length > 0 && (
          <div style={{ overflowX: 'auto', marginTop: 12 }}>
            <table className={styles.table}>
              <caption className={styles.personSub} style={{ textAlign: 'left', marginBottom: 6 }}>{shown.length} appointment{shown.length === 1 ? '' : 's'}</caption>
              <thead><tr><th scope="col">When</th><th scope="col">Customer</th><th scope="col">Service</th><th scope="col">Status</th><th scope="col">Booked</th></tr></thead>
              <tbody>
                {shown.map((b) => (
                  <tr key={b.id} tabIndex={0} style={{ cursor: 'pointer' }} onClick={() => setOpen(b)} onKeyDown={(e) => { if (e.key === 'Enter') setOpen(b); }}>
                    <td>{fmt(b.bookingTime)}</td><td>{b.customerName}</td><td>{b.serviceName}{b.addons?.length ? ` + ${b.addons.length}` : ''}</td>
                    <td>{b.status.replaceAll('_', ' ')}</td><td>{b.channel === 'online' ? 'Online' : 'Walk-in / phone'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <Modal open={!!open} title={open?.customerName ?? ''} subtitle={open ? fmt(open.bookingTime) : ''} onClose={() => setOpen(null)}>
        {open && (
          <div className={styles.stack} style={{ gap: 8 }}>
            <p><strong>{open.serviceName}</strong>{open.addons?.length ? <><br /><span className={styles.personSub}>Extras: {open.addons.map((a) => a.name).join(', ')}</span></> : null}</p>
            <p className={styles.personSub}>Status: {open.status.replaceAll('_', ' ')} · Booked {open.channel === 'online' ? 'online' : 'by phone or walk-in'}</p>
            <p>{open.customerEmail && <a href={`mailto:${open.customerEmail}`}>{open.customerEmail}</a>}{open.customerEmail && open.customerPhone ? ' · ' : ''}{open.customerPhone && <a href={`tel:${open.customerPhone}`}>{open.customerPhone}</a>}</p>
          </div>
        )}
      </Modal>
    </div>
  );
}
