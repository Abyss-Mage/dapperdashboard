'use client';
import { useEffect, useState } from 'react';
import { CalendarDays, Clock, DollarSign, History, Mail, Pencil, Phone, Sparkles, X } from 'lucide-react';
import { readableDay, worksiteDate } from '../workspace/employee_helpers';
import BookingRescheduler from '../workspace/BookingRescheduler';
import FormPopup from '../FormPopup';
import Select from '../GlassSelect';
import { DateTimePicker } from '../Pickers';
import owner from './owner.module.css';

export type CalendarBooking = { id: string; customerName: string; customerEmail: string; customerPhone: string; serviceName: string; serviceId?: string | null; bookingTime: string; status: string; channel: string; addons: { serviceId: string; name: string }[]; reservationId?: string | null; reservationVersion?: number | null };
type Bill = { totalCents: number; paidCents: number };
type Event = { id: string; fromStatus?: string | null; toStatus: string; notes: string; createdAt: string };

const money = (cents: number) => (cents / 100).toLocaleString('en-CA', { style: 'currency', currency: 'CAD' });
const LABEL: Record<string, string> = { pending: 'Pending', confirmed: 'Approved', completed: 'Completed', cancelled: 'Cancelled' };
const words = (s: string) => LABEL[s] ?? s.replaceAll('_', ' ');

/* The booking card that opens from the calendar: who, what, when and what it costs, with the status, reschedule and history actions the
   Appointments page has. Pickers open above the page, so moving a booking swaps this card for its own window rather than sitting on top of one. */
export default function BookingCard({ base, booking: b, timezone, canManage, onClose, onChanged }: { base: string; booking: CalendarBooking; timezone: string; canManage: boolean; onClose: () => void; onChanged: (notice: string) => void }) {
  const [bill, setBill] = useState<Bill | null>(null);
  const [ask, setAsk] = useState<'completed' | 'cancelled' | null>(null);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const [events, setEvents] = useState<Event[] | null>(null), [showHistory, setShowHistory] = useState(false);
  const [moving, setMoving] = useState(false), [moveTime, setMoveTime] = useState(''), [rescheduling, setRescheduling] = useState(false);
  const final = b.status === 'completed' || b.status === 'cancelled';
  const owed = bill ? bill.totalCents - bill.paidCents : 0;
  const time = new Intl.DateTimeFormat('en-CA', { timeZone: timezone, hour: 'numeric', minute: '2-digit' }).format(new Date(b.bookingTime));

  useEffect(() => {
    let live = true;
    const list = (url: string) => fetch(url, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : { items: [] })).then((body) => (body.items ?? []) as (Bill & { bookingId?: string; status: string })[]).catch(() => []);
    void (async () => {
      const found = (await list(`${base}/invoices`)).find((i) => i.bookingId === b.id && i.status !== 'void') ?? (await list(`${base}/legacy-billing`)).find((i) => i.bookingId === b.id && i.status !== 'void');
      if (live && found) setBill(found);
    })();
    return () => { live = false; };
  }, [base, b.id]);

  async function setStatus(toStatus: string) {
    setBusy(true); setError('');
    try {
      const r = await fetch(`${base}/bookings/${b.id}/status`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ toStatus, cancellationReason: toStatus === 'cancelled' ? reason : '', notes: 'Operator status update', ...(toStatus === 'cancelled' ? { reservationVersion: b.reservationVersion ?? 0 } : {}) }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? 'The status could not be changed.');
      onChanged(`Booking marked ${words(toStatus)}.${body.paymentNotice ? ' ' + body.paymentNotice : ''}`);
    } catch (e) { setError(e instanceof Error ? e.message : 'The status could not be changed.'); setBusy(false); }
  }

  async function saveMove() {
    setBusy(true); setError('');
    try {
      const r = await fetch(`${base}/bookings/${b.id}/move`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ bookingTime: new Date(moveTime).toISOString() }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? 'The time could not be changed.');
      onChanged('The appointment was moved.');
    } catch (e) { setError(e instanceof Error ? e.message : 'The time could not be changed.'); setBusy(false); }
  }

  function toggleHistory() {
    setShowHistory((on) => !on);
    if (events) return;
    fetch(`${base}/bookings/${b.id}`, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : { events: [] })).then((body) => setEvents(body.events ?? [])).catch(() => setEvents([]));
  }

  if (rescheduling) return <BookingRescheduler base={base} booking={b} onSaved={async () => onChanged('The appointment was moved.')} onClose={() => setRescheduling(false)} />;
  if (moving) return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Reschedule appointment">
      <div className="modal" style={{ padding: 24, display: 'grid', gap: 14 }}>
        <h3 style={{ margin: 0 }}>Reschedule “{b.serviceName}”</h3>
        <p>This appointment holds no calendar slot, so choose the new date and time yourself.</p>
        <label className="form-field">New date and time<DateTimePicker value={moveTime} onChange={(e) => setMoveTime(e.target.value)} /></label>
        {error && <p role="alert" className="field-error">{error}</p>}
        <div className="form-actions" style={{ justifyContent: 'flex-start' }}>
          <button className="primary" disabled={busy || !moveTime} onClick={() => void saveMove()}>{busy ? 'Saving…' : 'Move appointment'}</button>
          <button className="secondary" disabled={busy} onClick={() => setMoving(false)}>Back</button>
        </div>
      </div>
    </div>
  );

  return (
    <FormPopup label="Selected booking" width={380} onClose={onClose}>
      <div className={owner.apptCard}>
        <div className={owner.apptTop}>
          <h2>{b.customerName}</h2>
          <button type="button" aria-label="Close booking details" onClick={onClose}><X size={18} /></button>
        </div>
        <p className={owner.apptRow}><Sparkles size={16} aria-hidden="true" /><span>{b.serviceName}{b.addons?.length ? ` · ${b.addons.map((a) => a.name).join(', ')}` : ''}</span></p>
        <p className={owner.apptRow}><CalendarDays size={16} aria-hidden="true" /><span>{readableDay(worksiteDate(b.bookingTime, timezone), { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' })}</span></p>
        <p className={owner.apptRow}><Clock size={16} aria-hidden="true" /><span>{time}</span></p>
        {b.customerEmail && <p className={owner.apptRow}><Mail size={16} aria-hidden="true" /><a href={`mailto:${b.customerEmail}`}>{b.customerEmail}</a></p>}
        {b.customerPhone && <p className={owner.apptRow}><Phone size={16} aria-hidden="true" /><a href={`tel:${b.customerPhone}`}>{b.customerPhone}</a></p>}
        <p className={owner.apptRow}><DollarSign size={16} aria-hidden="true" /><span>{bill ? `${money(bill.totalCents)}${owed > 0 ? ` · ${money(owed)} still owed` : ' · paid'}` : 'No invoice yet'}</span></p>

        {ask && (
          <div className={owner.apptAsk}>
            {ask === 'completed'
              ? <p>{owed > 0 ? `${money(owed)} is still owed. Record the payment on the Appointments page, or complete it anyway.` : 'Mark this booking as completed?'}</p>
              : <label className="form-field">Why is it being cancelled? (optional)<textarea maxLength={2000} rows={2} value={reason} disabled={busy} onChange={(e) => setReason(e.target.value)} /></label>}
            <div>
              <button type="button" className="primary" disabled={busy} onClick={() => void setStatus(ask)}>{busy ? 'Saving…' : ask === 'completed' ? 'Complete' : 'Cancel booking'}</button>
              <button type="button" className="secondary" disabled={busy} onClick={() => { setAsk(null); setError(''); }}>Back</button>
            </div>
          </div>
        )}
        {error && <p role="alert" className="field-error">{error}</p>}

        <div className={owner.apptFoot}>
          {canManage
            ? <span className={owner.apptSelect}><Select aria-label="Change status" disabled={busy} value={b.status === 'pending' ? 'pending' : final ? b.status : 'confirmed'} onChange={(e) => { const v = e.target.value; if (v === 'completed' || v === 'cancelled') { setError(''); setReason(''); setAsk(v); } else if (v === 'confirmed' && b.status !== 'confirmed') void setStatus('confirmed'); }}>
                {b.status === 'pending' && <option value="pending">Pending</option>}
                <option value="confirmed">Approved</option><option value="completed">Completed</option><option value="cancelled">Cancelled</option>
              </Select></span>
            : <span className={owner.apptStatus}>{words(b.status)}</span>}
          <span className={owner.apptTools}>
            {canManage && !final && <button type="button" aria-label="Reschedule" title="Reschedule" disabled={busy} onClick={() => { setError(''); if (b.reservationId) setRescheduling(true); else { setMoveTime(''); setMoving(true); } }}><Pencil size={16} aria-hidden="true" /></button>}
            <button type="button" aria-label="History" title="History" aria-pressed={showHistory} onClick={toggleHistory}><History size={16} aria-hidden="true" /></button>
          </span>
        </div>

        {showHistory && (
          <div className={owner.apptHistory}>
            {events === null ? <p>Loading…</p> : events.length === 0 ? <p>No history yet.</p> : events.map((ev) => (
              <p key={ev.id}><strong>{ev.fromStatus ? `${words(ev.fromStatus)} → ${words(ev.toStatus)}` : words(ev.toStatus)}</strong><small>{new Date(ev.createdAt).toLocaleString('en-CA', { timeZone: timezone, dateStyle: 'medium', timeStyle: 'short' })}</small></p>
            ))}
          </div>
        )}
      </div>
    </FormPopup>
  );
}
