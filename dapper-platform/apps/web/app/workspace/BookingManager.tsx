'use client';
import {useCallback,useEffect,useRef,useState} from 'react';
import BookingRescheduler from './BookingRescheduler';

type Customer = {id: string; name: string; email: string; phone: string};
type Vehicle = {id: string; customerId: string; description: string; identifier: string};
type Service = {id: string; name: string; kind: string; archived: boolean; parentId: string | null};
type Reservation = {id: string; start: string; durationMinutes: number; cancelledAt: string | null};

type Booking = {
  id: string;
  organizationId: string;
  locationId: string;
  customerId: string;
  vehicleId?: string | null;
  serviceId?: string | null;
  serviceName: string;
  addons: {serviceId: string; name: string}[];
  reservationId?: string | null;
  reservationVersion?: number | null;
  channel: 'online' | 'internal' | 'phone' | 'walk_in' | 'dealership';
  status: 'pending' | 'awaiting_payment' | 'confirmed' | 'scheduled' | 'in_progress' | 'completed' | 'cancelled';
  bookingTime: string;
  notes: string;
  cancellationReason: string;
  createdAt: string;
  customerName: string;
  customerEmail: string;
  customerPhone: string;
};

type BookingEvent = {
  id: string;
  bookingId: string;
  actorId?: string;
  fromStatus?: string;
  toStatus: string;
  notes: string;
  createdAt: string;
};

export default function BookingManager({
  base,
  canCreate,
  canManage,
  canCreateWorkOrders,
  onJobSpawned,
  revision = 0,
}: {
  base: string;
  canCreate: boolean;
  canManage: boolean;
  canCreateWorkOrders: boolean;
  onJobSpawned?: () => Promise<void>;
  revision?: number;
}) {
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [bookingsLoading, setBookingsLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState('');
  const [selectedBooking, setSelectedBooking] = useState<Booking | null>(null);
  const [events, setEvents] = useState<BookingEvent[]>([]);
  const [showCreateForm, setShowCreateForm] = useState(false);

  const bookingsReqIdRef = useRef(0);
  const choicesReqIdRef = useRef(0);

  // Form states
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [vehicleId, setVehicleId] = useState('');
  const [services, setServices] = useState<Service[]>([]);
  const [reservations, setReservations] = useState<Reservation[]>([]);
  const [customerId, setCustomerId] = useState('');
  const [serviceId, setServiceId] = useState('');
  const [addonIds, setAddonIds] = useState<string[]>([]);
  const [reservationId, setReservationId] = useState('');
  const [channel, setChannel] = useState<'internal' | 'phone' | 'walk_in' | 'dealership'>('internal');
  const [bookingTime, setBookingTime] = useState('');
  const [notes, setNotes] = useState('');
  const [cancellationReason, setCancellationReason] = useState('');
  const [cancellingBooking, setCancellingBooking] = useState<Booking | null>(null);
  const [reschedulingBooking, setReschedulingBooking] = useState<Booking | null>(null);
  const rescheduleTrigger = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {if (!reschedulingBooking) rescheduleTrigger.current?.focus();}, [reschedulingBooking]);
  const cancellationDialog = useRef<HTMLDialogElement>(null);
  const cancellationError = useRef<HTMLParagraphElement>(null);
  const cancellationTrigger = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    const dialog = cancellationDialog.current;
    if (cancellingBooking && dialog && !dialog.open) dialog.showModal();
  }, [cancellingBooking]);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [workOrderRequests, setWorkOrderRequests] = useState<Record<string, string>>({});
  useEffect(() => {if (cancellingBooking && error && !busy) cancellationError.current?.focus();}, [cancellingBooking, error, busy]);
  function closeCancellation() {
    cancellationDialog.current?.close();
    setCancellingBooking(null);
    cancellationTrigger.current?.focus();
  }

  const loadBookings = useCallback(async () => {
    const reqId = ++bookingsReqIdRef.current;
    if (!base) {
      setBookings([]);
      setBookingsLoading(false);
      setError('');
      return;
    }
    setBookingsLoading(true);
    setError('');
    try {
      const url = `${base}/bookings${statusFilter ? `?status=${statusFilter}` : ''}`;
      const r = await fetch(url);
      if (reqId !== bookingsReqIdRef.current) return;
      if (r.status === 403) {
        setBookings([]);
        setError('Access needed (bookings.read)');
        return;
      }
      if (!r.ok) {
        setBookings([]);
        const body = await r.json().catch(() => ({}));
        throw new Error(body.error ?? 'Could not load bookings for this location.');
      }
      const data = await r.json();
      if (reqId !== bookingsReqIdRef.current) return;
      setBookings(data.items ?? []);
      setError('');
    } catch (err) {
      if (reqId !== bookingsReqIdRef.current) return;
      setBookings([]);
      setError(err instanceof Error ? err.message : 'Could not load bookings for this location.');
    } finally {
      if (reqId === bookingsReqIdRef.current) {
        setBookingsLoading(false);
      }
    }
  }, [base, statusFilter]);

  const loadChoices = useCallback(async () => {
    const reqId = ++choicesReqIdRef.current;
    if (!base) {
      setCustomers([]);
      setServices([]);
      setReservations([]);
      setVehicles([]);
      return;
    }
    try {
      const now = new Date();
      const past = new Date(now.getTime() - 24 * 3600 * 1000).toISOString();
      const future = new Date(now.getTime() + 60 * 24 * 3600 * 1000).toISOString();

      const [cRes, sRes, rRes, vRes] = await Promise.all([
        fetch(base + '/customers'),
        fetch(base + '/services'),
        fetch(`${base}/scheduling/reservations?from=${encodeURIComponent(past)}&to=${encodeURIComponent(future)}`).catch(
          () => null
        ),
        fetch(base + '/vehicles').catch(() => null),
      ]);
      if (reqId !== choicesReqIdRef.current) return;
      if (cRes.ok) setCustomers((await cRes.json()).items ?? []);
      else setCustomers([]);
      if (sRes.ok) setServices((await sRes.json()).items.filter((s: Service) => !s.archived) ?? []);
      else setServices([]);
      if (rRes && rRes.ok) setReservations((await rRes.json()).items ?? []);
      else setReservations([]);
      setVehicles(vRes?.ok ? (await vRes.json()).items ?? [] : []);
    } catch {
      if (reqId !== choicesReqIdRef.current) return;
      // choices loading fallback
    }
  }, [base]);

  useEffect(() => {
    loadBookings();
  }, [loadBookings, revision]);

  useEffect(() => {
    if (showCreateForm) loadChoices();
  }, [showCreateForm, loadChoices]);

  async function viewEvents(booking: Booking) {
    setSelectedBooking(booking);
    setEvents([]);
    try {
      const r = await fetch(`${base}/bookings/${booking.id}`);
      if (r.ok) {
        const body = await r.json();
        setEvents(body.events ?? []);
      }
    } catch {
      // ignore
    }
  }

  async function updateStatus(bookingId: string, toStatus: string, reason = '') {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(`${base}/bookings/${bookingId}/status`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({toStatus, cancellationReason: reason, notes: 'Operator status update',...(toStatus==='cancelled'?{reservationVersion:cancellingBooking?.reservationVersion??0}:{})}),
      });
      const body = await r.json();
      if (!r.ok) {
        setError(body.error ?? 'Status update failed');
        return;
      }
      setNotice(`Booking updated to ${toStatus}.`);
      if (toStatus === 'cancelled') closeCancellation();
      if (selectedBooking?.id === bookingId) {
        setSelectedBooking(body);
        const evR = await fetch(`${base}/bookings/${bookingId}`);
        if (evR.ok) {const detail=await evR.json();setEvents(detail.events??[]);setSelectedBooking(detail.booking);}
      }
      await loadBookings();
    } catch {
      setError('Could not update booking status. Refresh before retrying.');
    } finally {
      setBusy(false);
    }
  }

  async function createWorkOrder(booking: Booking) {
    setBusy(true);
    setError('');
    setNotice('');
	const storageKey = `dapper-booking-work-order:${base}:${booking.id}`;
	let priorKey: string | undefined = workOrderRequests[booking.id];
	try { priorKey = priorKey ?? sessionStorage.getItem(storageKey) ?? undefined; } catch { /* In-memory retry receipt remains available. */ }
	const requestKey = priorKey ?? crypto.randomUUID();
	try { sessionStorage.setItem(storageKey, requestKey); } catch { /* Storage is optional; no credentials are stored. */ }
	setWorkOrderRequests((current) => ({...current, [booking.id]: requestKey}));
    try {
      const r = await fetch(`${base}/bookings/${booking.id}/work-orders`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          requestKey,
          description: `Internal work order from booking #${booking.id.slice(0, 8)}`,
        }),
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not create work order from booking');
        return;
      }
      const created = await r.json();
      setNotice(`Work order #${created.id.slice(0, 8)} is linked to booking #${booking.id.slice(0, 8)}.`);
      if (onJobSpawned) await onJobSpawned();
    } catch {
      setError('Could not spawn work order.');
    } finally {
      setBusy(false);
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const parsedTime = new Date(bookingTime).toISOString();
      const r = await fetch(base + '/bookings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          customerId,
          vehicleId: vehicleId || null,
          serviceId: serviceId || null,
          addonIds,
          reservationId: reservationId || null,
          channel,
          status: 'pending',
          bookingTime: parsedTime,
          notes,
        }),
      });
      const body = await r.json();
      if (!r.ok) {
        setError(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'Could not create booking');
        return;
      }
      setShowCreateForm(false);
      setNotice(`Booking #${body.id.slice(0, 8)} created successfully.`);
      setCustomerId('');
      setVehicleId('');
      setServiceId('');
      setAddonIds([]);
      setReservationId('');
      setBookingTime('');
      setNotes('');
      await loadBookings();
    } catch {
      setError('Could not save booking. Input preserved.');
    } finally {
      setBusy(false);
    }
  }

  function formatTime(iso: string) {
    return new Date(iso).toLocaleString('en-CA', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  return (
    <section className="panel detail-card">
      <div className="section-head">
        <div>
          <h2>Bookings & Appointments</h2>
          <p>
            Customer appointments, online bookings, dealership requests, and walk-in schedules. All status transitions
            are validated and logged in an immutable audit ledger.
          </p>
        </div>
        <div style={{display: 'flex', gap: '0.5rem', flexWrap: 'wrap'}}>
          {canCreate && (
            <button
              className="primary"
              disabled={busy}
              onClick={() => setShowCreateForm((prev) => !prev)}
            >
              {showCreateForm ? 'Cancel new booking' : '+ New appointment'}
            </button>
          )}
          <button type="button" className="secondary" disabled={busy} onClick={loadBookings}>
            Refresh bookings
          </button>
        </div>
      </div>

      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}

      {showCreateForm && (
        <form onSubmit={handleCreate} style={{marginBottom: '1.5rem', padding: '1.25rem', border: '1px solid var(--border)', borderRadius: '8px', background: 'var(--panel-subtle)'}}>
          <h3>Create internal appointment</h3>
          <div className="management-form">
            <label className="form-field">
              Customer
              <select required disabled={busy} value={customerId} onChange={(e) => {setCustomerId(e.target.value); setVehicleId('');}}>
                <option value="">Select customer</option>
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.phone || c.email})
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              Customer vehicle or equipment (optional)
              <select disabled={busy || !customerId} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)}>
                <option value="">No equipment selected</option>
                {vehicles.filter((v) => v.customerId === customerId).map((v) => (
                  <option key={v.id} value={v.id}>{v.description}{v.identifier ? ` · ${v.identifier}` : ''}</option>
                ))}
              </select>
              <small>Only equipment owned by the selected customer and visible with your permissions is listed.</small>
            </label>
            <label className="form-field">
              Main service
              <select
                required
                disabled={busy}
                value={serviceId}
                onChange={(e) => {
                  setServiceId(e.target.value);
                  setAddonIds([]);
                }}
              >
                <option value="">Select service</option>
                {services
                  .filter((s) => s.kind === 'main')
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
              </select>
            </label>
          </div>

          <div className="management-form">
            <label className="form-field">
              Intake channel
              <select disabled={busy} value={channel} onChange={(e) => setChannel(e.target.value as any)}>
                <option value="internal">Internal / Shop</option>
                <option value="phone">Phone call</option>
                <option value="walk_in">Walk-in</option>
                <option value="dealership">Dealership / Fleet</option>
              </select>
            </label>
            <label className="form-field">
              Appointment date & time
              <input
                type="datetime-local"
                required
                disabled={busy}
                value={bookingTime}
                onChange={(e) => setBookingTime(e.target.value)}
              />
            </label>
            {reservations.length > 0 && (
              <label className="form-field">
                Capacity slot (optional)
                <select disabled={busy} value={reservationId} onChange={(e) => setReservationId(e.target.value)}>
                  <option value="">None</option>
                  {reservations
                    .filter((r) => !r.cancelledAt)
                    .map((r) => (
                      <option key={r.id} value={r.id}>
                        {new Date(r.start).toLocaleString('en-CA')} ({r.durationMinutes} min)
                      </option>
                    ))}
                </select>
              </label>
            )}
          </div>

          {serviceId && services.some((s) => s.kind === 'addon' && s.parentId === serviceId) && (
            <fieldset disabled={busy} style={{margin: '1rem 0'}}>
              <legend>Select add-ons</legend>
              {services
                .filter((s) => s.kind === 'addon' && s.parentId === serviceId)
                .map((addon) => (
                  <label key={addon.id} className="form-field">
                    <span>
                      <input
                        type="checkbox"
                        checked={addonIds.includes(addon.id)}
                        onChange={(e) =>
                          setAddonIds((prev) =>
                            e.target.checked ? [...prev, addon.id] : prev.filter((id) => id !== addon.id)
                          )
                        }
                      />
                      {addon.name}
                    </span>
                  </label>
                ))}
            </fieldset>
          )}

          <label className="form-field">
            Appointment notes
            <input
              type="text"
              maxLength={1000}
              placeholder="e.g. Needs interior child seat removal, exterior mud removal"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
          </label>

          <button className="primary" disabled={busy || !customerId || !serviceId || !bookingTime}>
            {busy ? 'Saving…' : 'Confirm appointment'}
          </button>
        </form>
      )}

      <div style={{display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap'}}>
        {['', 'pending', 'awaiting_payment', 'confirmed', 'scheduled', 'in_progress', 'completed', 'cancelled'].map((st) => (
          <button
            key={st}
            type="button"
            className={statusFilter === st ? 'secondary active' : 'secondary'}
            onClick={() => {
              bookingsReqIdRef.current++;
              setBookings([]);
              setBookingsLoading(true);
              setStatusFilter(st);
            }}
            style={{textTransform: 'capitalize'}}
            aria-pressed={statusFilter === st}
          >
            {st === 'awaiting_payment' ? 'Awaiting payment' : (st ? st.replace('_', ' ') : 'All statuses')}
          </button>
        ))}
      </div>

      <div className="management-list">
        {bookings.map((b) => (
          <article key={b.id} style={{display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '1rem'}}>
            <div>
              <div style={{display: 'flex', gap: '0.5rem', alignItems: 'center', marginBottom: '0.25rem'}}>
                <strong>{b.serviceName}</strong>
                <span className={`status ${b.status === 'completed' ? 'progress' : b.status === 'cancelled' ? 'quote' : 'scheduled'}`}>
                  {b.status.replace('_', ' ')}
                </span>
                <small style={{textTransform: 'uppercase', fontSize: '0.75rem', color: 'var(--muted)'}}>
                  via {b.channel}
                </small>
              </div>
              <small>
                Customer: <strong>{b.customerName}</strong> ({b.customerPhone || b.customerEmail || 'No phone'}) · Scheduled for {formatTime(b.bookingTime)}
              </small>
              {b.addons?.length > 0 && (
                <small>Add-ons: {b.addons.map((a) => a.name).join(', ')}</small>
              )}
              {b.reservationId && (
                <small>Capacity reservation: {b.reservationId.slice(0, 8)}</small>
              )}
              {b.notes && <small>Notes: {b.notes}</small>}
              {b.cancellationReason && (
                <small style={{color: 'var(--error)'}}>Cancelled: {b.cancellationReason}</small>
              )}
            </div>

            <div style={{display: 'flex', gap: '0.5rem', flexWrap: 'wrap'}}>
              <button className="secondary" onClick={() => viewEvents(b)}>
                History trail
              </button>

              {canManage && (
                <>
                  {b.status === 'pending' && (
                    <button className="primary" disabled={busy} onClick={() => updateStatus(b.id, 'confirmed')}>
                      Confirm
                    </button>
                  )}
                  {b.status === 'confirmed' && (
                    <>
                      <button className="secondary" disabled={busy} onClick={() => updateStatus(b.id, 'scheduled')}>
                        Schedule
                      </button>
                    </>
                  )}
                  {b.status === 'scheduled' && (
                    <>
                      <button className="primary" disabled={busy} onClick={() => updateStatus(b.id, 'in_progress')}>
                        Start bay work
                      </button>
                    </>
                  )}
                  {b.status === 'in_progress' && (
                    <button className="primary" disabled={busy} onClick={() => updateStatus(b.id, 'completed')}>
                      Complete
                    </button>
                  )}
                  {b.status !== 'completed' && b.status !== 'cancelled' && (
                    <button
                      className="secondary"
                      disabled={busy}
                      onClick={(event) => {
                        cancellationTrigger.current = event.currentTarget;
                        setCancellationReason(''); setError(''); setCancellingBooking(b);
                      }}
                    >
                      Cancel
                    </button>
                  )}
                  {b.reservationId && b.status !== 'completed' && b.status !== 'cancelled' && b.status !== 'in_progress' && <button className="secondary" disabled={busy} onClick={event => {rescheduleTrigger.current=event.currentTarget; setReschedulingBooking(b);}}>Reschedule</button>}
                </>
              )}
              {canCreateWorkOrders && b.status !== 'cancelled' && b.status !== 'completed' && (
                <button className="secondary" disabled={busy} onClick={() => createWorkOrder(b)}>
                  Create work order
                </button>
              )}
            </div>
          </article>
        ))}
        {bookingsLoading && <p style={{color: 'var(--text-muted, var(--text-muted, #6b5558))', margin: '0.75rem 0'}}>Loading appointments…</p>}
        {!bookingsLoading && !bookings.length && <p>No bookings found matching filter.</p>}
      </div>

      {reschedulingBooking && <BookingRescheduler base={base} booking={reschedulingBooking} onSaved={loadBookings} onClose={() => setReschedulingBooking(null)}/>}
      {cancellingBooking && (
        <dialog ref={cancellationDialog} aria-labelledby="booking-cancel-title" onCancel={(event) => {event.preventDefault(); if (!busy) closeCancellation();}} style={{maxWidth:'min(32rem, calc(100vw - 2rem))',padding:'1.5rem',background:'var(--panel, #fff)',color:'var(--text, var(--text, #1a0b0d))',border:'1px solid var(--border, #f0dcde)',borderRadius:'12px'}}>
          <form onSubmit={(event) => {event.preventDefault(); updateStatus(cancellingBooking.id, 'cancelled', cancellationReason);}}>
            <h3 id="booking-cancel-title">Cancel booking</h3>
            <p>{cancellingBooking.customerName} · {cancellingBooking.serviceName}</p>
            <p>This cancels the booking and releases its reserved capacity. Payment and work order changes require their own actions.</p>
            <label>Cancellation reason<textarea autoFocus maxLength={2000} value={cancellationReason} disabled={busy} onChange={(event) => setCancellationReason(event.target.value)} style={{display:'block',width:'100%',boxSizing:'border-box'}} /></label>
            {error && <p ref={cancellationError} role="alert" tabIndex={-1}>{error}</p>}
            <div style={{display:'flex',gap:'.75rem',marginTop:'1rem',flexWrap:'wrap'}}>
              <button type="button" className="secondary" disabled={busy} onClick={closeCancellation}>Keep booking</button>
              <button type="submit" className="primary" disabled={busy}>{busy?'Cancelling…':'Cancel booking'}</button>
            </div>
          </form>
        </dialog>
      )}
      {selectedBooking && (
        <dialog
          open
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            border: 'none',
            zIndex: 100,
          }}
          onClick={() => setSelectedBooking(null)}
        >
          <div
            style={{
              background: 'var(--panel)',
              padding: '1.5rem',
              borderRadius: '8px',
              maxWidth: '550px',
              width: '90%',
              border: '1px solid var(--border)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <h3>Booking Audit Trail · #{selectedBooking.id.slice(0, 8)}</h3>
            <p style={{fontSize: '0.85rem', color: 'var(--muted)'}}>
              Customer: {selectedBooking.customerName} · Service: {selectedBooking.serviceName}
            </p>
            <div style={{margin: '1rem 0', maxHeight: '300px', overflowY: 'auto'}}>
              {events.map((ev) => (
                <div key={ev.id} style={{padding: '0.5rem 0', borderBottom: '1px solid var(--border)'}}>
                  <div style={{display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem'}}>
                    <strong>
                      {ev.fromStatus ? `${ev.fromStatus} → ${ev.toStatus}` : `Status: ${ev.toStatus}`}
                    </strong>
                    <small>{formatTime(ev.createdAt)}</small>
                  </div>
                  {ev.notes && <p style={{fontSize: '0.8rem', margin: '0.25rem 0'}}>{ev.notes}</p>}
                </div>
              ))}
              {!events.length && <p>Loading event ledger…</p>}
            </div>
            <button className="secondary" onClick={() => setSelectedBooking(null)}>
              Close
            </button>
          </div>
        </dialog>
      )}
    </section>
  );
}
