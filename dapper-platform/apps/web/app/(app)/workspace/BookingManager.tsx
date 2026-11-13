'use client';
import { ChevronDown } from 'lucide-react';
import {useCallback,useEffect,useRef,useState} from 'react';
import BookingRescheduler from './BookingRescheduler';
import FormPopup from '../FormPopup';
import { fillInvoiceWindow, openInvoiceWindow, openPdf, ownLayout, usePrintTemplate } from './InvoicePrint';
import Select from '../GlassSelect';
import { DatePicker, DateTimePicker } from '../Pickers';
import t from './bookingTable.module.css';

type Customer = {id: string; name: string; email: string; phone: string};
type Vehicle = {id: string; customerId: string; description: string; identifier: string};
type Service = {id: string; name: string; kind: string; archived: boolean; parentId: string | null; parentIds?: string[]; durationMinutes?: number | null};
type Reservation = {id: string; start: string; durationMinutes: number; cancelledAt: string | null};

type Booking = {
  id: string;
  /** The short booking number (one sequence per organization; imported bookings keep the old site's ID). */
  number?: number;
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
  serviceAddress?: string;
  notes: string;
  cancellationReason: string;
  createdAt: string;
  customerName: string;
  customerEmail: string;
  customerPhone: string;
};

/** What staff and customers call a booking: its number, or the start of its id when it has none. */
const bookingRef = (b: { id: string; number?: number }) => '#' + (b.number ?? b.id.slice(0, 6).toUpperCase());
type SortKey = 'id' | 'date' | 'customer' | 'service' | 'status' | 'payment' | 'created';

type BookingEvent = {
  id: string;
  bookingId: string;
  actorId?: string;
  fromStatus?: string;
  toStatus: string;
  notes: string;
  createdAt: string;
};

const STATUS_LABEL: Record<string, string> = { pending: 'Pending', awaiting_payment: 'Awaiting payment', confirmed: 'Approved', scheduled: 'Approved', in_progress: 'Approved', completed: 'Completed', cancelled: 'Cancelled' };
const money = (c: number) => '$' + (c / 100).toLocaleString('en-CA', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

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
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [customerQ, setCustomerQ] = useState('');
  const [serviceF, setServiceF] = useState('');
  const [idQ, setIdQ] = useState('');
  const [sortKey, setSortKey] = useState<SortKey | ''>('');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');
  const [textQ, setTextQ] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(20);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  // Service lengths for the Duration column (loaded once; the new-appointment form reloads the full choices).
  const [lengths, setLengths] = useState<Record<string, number>>({});
  useEffect(() => {
    if (!base) return;
    let live = true;
    fetch(base + '/services', { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null)).then((b) => {
      if (!live || !b) return;
      const map: Record<string, number> = {};
      for (const s of (b.items ?? []) as Service[]) if (s.durationMinutes) map[s.id] = s.durationMinutes;
      setLengths(map);
    }).catch(() => undefined);
    return () => { live = false; };
  }, [base]);
  const [open, setOpen] = useState<Set<string>>(new Set());
  // Each booking's invoice (one per booking), so the appointment shows what is still owed or already paid.
  const [invoices, setInvoices] = useState<Record<string, { id: string; version: number; invoiceNumber: string; status: string; totalCents: number; paidCents: number; discountCents?: number; taxCents?: number; subtotalCents?: number; couponCode?: string; legacy?: boolean; issuedAt?: string; paidAt?: string }>>({});
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
  const [serviceAddress, setServiceAddress] = useState('');
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

  const loadInvoices = useCallback(async () => {
    if (!base) { setInvoices({}); return; }
    try {
      const r = await fetch(base + '/invoices', { cache: 'no-store' });
      if (!r.ok) return;
      const map: Record<string, { id: string; version: number; invoiceNumber: string; status: string; totalCents: number; paidCents: number; discountCents?: number; taxCents?: number; subtotalCents?: number; couponCode?: string; legacy?: boolean; issuedAt?: string; paidAt?: string }> = {};
      for (const i of ((await r.json()).items ?? []) as { id: string; version: number; bookingId?: string; invoiceNumber: string; status: string; totalCents: number; paidCents: number; discountCents?: number; taxCents?: number; subtotalCents?: number; couponCode?: string; legacy?: boolean; issuedAt?: string; paidAt?: string }[]) if (i.bookingId && i.status !== 'void') map[i.bookingId] = i;
      // Bookings imported from the old site have no stored invoice: their billing comes as a summary and the document is built when opened.
      try {
        const lr = await fetch(base + '/legacy-billing', { cache: 'no-store' });
        if (lr.ok) for (const l of ((await lr.json()).items ?? []) as { bookingId: string; invoiceNumber: string; status: string; totalCents: number; paidCents: number; discountCents?: number; taxCents?: number; subtotalCents?: number; couponCode?: string }[]) if (!map[l.bookingId]) map[l.bookingId] = { ...l, id: 'legacy-' + l.bookingId, version: 1, legacy: true };
      } catch { /* imported-booking billing is optional here */ }
      setInvoices(map);
    } catch { /* invoices are optional here */ }
  }, [base]);
  useEffect(() => { void loadInvoices(); }, [loadInvoices, revision, bookings]);
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
      const query = new URLSearchParams({ limit: '500' });
      if (statusFilter) query.set('status', statusFilter);
      if (dateFrom || dateTo) {
        const from = dateFrom ? new Date(dateFrom + 'T00:00:00') : new Date(new Date(dateTo + 'T00:00:00').getTime() - 365 * 86400000);
        const to = dateTo ? new Date(dateTo + 'T23:59:59') : new Date(from.getTime() + 365 * 86400000);
        query.set('from', from.toISOString()); query.set('to', to.toISOString());
      }
      const url = `${base}/bookings?${query}`;
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
  }, [base, statusFilter, dateFrom, dateTo]);

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

  async function updateStatus(bookingId: string, toStatus: string, reason = ''): Promise<boolean> {
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
        return false;
      }
      setNotice(`Booking marked ${STATUS_LABEL[toStatus] ?? toStatus}.${body.paymentNotice ? ' ' + body.paymentNotice : ''}`);
      void loadInvoices();
      if (toStatus === 'cancelled') closeCancellation();
      if (selectedBooking?.id === bookingId) {
        setSelectedBooking(body);
        const evR = await fetch(`${base}/bookings/${bookingId}`);
        if (evR.ok) {const detail=await evR.json();setEvents(detail.events??[]);setSelectedBooking(detail.booking);}
      }
      await loadBookings();
      return true;
    } catch {
      setError('Could not update booking status. Refresh before retrying.');
      return false;
    } finally {
      setBusy(false);
    }
  }

  // Completing a booking: the invoice can be settled in cash at the same moment (it is issued first if it is still a draft),
  // or the booking can be completed with the money still owed.
  const printTemplate = usePrintTemplate(`${base}/invoice-template`);
  // The invoice opens as its own page in a new window (opened straight from the click so the browser allows it).
  function viewInvoice(b: Booking) {
    const inv = invoices[b.id];
    if (!inv) return;
    if (!ownLayout(printTemplate)) { openPdf(inv.legacy ? `${base}/legacy-invoices/${b.id}/pdf` : `${base}/invoices/${inv.id}/pdf`); return; }
    const w = openInvoiceWindow();
    if (!w) { setError('Allow pop-ups for this site to open the invoice in a new window.'); return; }
    fetch(inv.legacy ? base + '/legacy-invoices/' + b.id : base + '/invoices/' + inv.id, { cache: 'no-store' })
      .then(async (r) => { const body = await r.json().catch(() => ({})); if (!r.ok) throw new Error(body.error ?? 'The invoice could not be loaded.'); return body; })
      .then((body) => fillInvoiceWindow(w, body, printTemplate))
      .catch((e) => { w.close(); setError(e instanceof Error ? e.message : 'The invoice could not be loaded.'); });
  }
  // A booking that holds no calendar reservation (entered by hand) is moved by changing its time directly.
  const [moving, setMoving] = useState<Booking | null>(null);
  const [moveTime, setMoveTime] = useState('');
  const [moveBusy, setMoveBusy] = useState(false);
  const [moveError, setMoveError] = useState('');
  async function saveMove() {
    if (!moving || !moveTime) return;
    setMoveBusy(true); setMoveError('');
    try {
      const r = await fetch(`${base}/bookings/${moving.id}/move`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ bookingTime: new Date(moveTime).toISOString() }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'The time could not be changed.');
      setMoving(null); setNotice('The appointment was moved.');
      await loadBookings();
    } catch (e) { setMoveError(e instanceof Error ? e.message : 'The time could not be changed.'); }
    finally { setMoveBusy(false); }
  }
  // A coupon can be put on an invoice that is still a draft (before it is issued), from the appointment.
  const [couponCodes, setCouponCodes] = useState<Record<string, string>>({});
  const [couponBusy, setCouponBusy] = useState(false);
  async function couponAction(b: Booking, remove: boolean) {
    const inv = invoices[b.id];
    if (!inv) return;
    setCouponBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(base + '/invoices/' + inv.id + '/coupon', remove ? { method: 'DELETE' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code: couponCodes[b.id] ?? '' }) });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error ?? 'The coupon could not be applied.');
      setNotice(remove ? 'Coupon removed.' : 'Coupon applied.');
      setCouponCodes((prev) => ({ ...prev, [b.id]: '' }));
      await loadInvoices();
    } catch (e) { setError(e instanceof Error ? e.message : 'The coupon could not be applied.'); }
    finally { setCouponBusy(false); }
  }
  // A website booking paid by card is paid within moments of its invoice being issued; anything else is paid at the shop.
  // shortcut: judged from the issue and payment times, so a card payment taken more than ten minutes later reads as paid at the shop; store the booking's payment choice if that matters.
  const payWay = (inv?: { status: string; issuedAt?: string; paidAt?: string }) => {
    if (inv?.issuedAt && inv.paidAt && new Date(inv.paidAt).getTime() - new Date(inv.issuedAt).getTime() < 10 * 60 * 1000) return 'Online · paid by card';
    return 'Online · pay local';
  };
  const [completing, setCompleting] = useState<Booking | null>(null);
  const [completeBusy, setCompleteBusy] = useState(false);
  const [completeError, setCompleteError] = useState('');
  async function finishBooking(b: Booking, method: '' | 'cash' | 'bank_transfer') {
    setCompleteBusy(true); setCompleteError('');
    try {
      const inv = invoices[b.id];
      if (method && inv) {
        let id = inv.id;
        if (inv.status === 'draft') {
          const pr = await fetch(base + '/invoices/' + id + '/issuance-preview', { cache: 'no-store' });
          const preview = await pr.json().catch(() => ({}));
          if (!pr.ok) throw new Error('Your business details are not set up yet. Open Settings > Invoice template, add the business name, address and invoice numbering, and link them to this franchise. Then try again.');
          const ir = await fetch(base + '/invoices/' + id + '/issue', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ issuerId: preview.id, issuerVersion: preview.version, invoiceVersion: inv.version }) });
          const issued = await ir.json().catch(() => ({}));
          if (!ir.ok) throw new Error(/issuer/i.test(issued.error ?? '') ? 'Your business details are not set up yet. Open Settings > Invoice template, add the business name, address and invoice numbering, and link them to this franchise. Then try again.' : issued.error ?? 'The invoice could not be issued.');
          id = issued.id;
        }
        const fr = await fetch(base + '/invoices/' + id, { cache: 'no-store' });
        const full = await fr.json().catch(() => ({}));
        if (!fr.ok) throw new Error(full.error ?? 'The invoice could not be read.');
        if (full.dueCents > 0) {
          const pay = await fetch(base + '/invoices/' + id + '/payments', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ amountCents: full.dueCents, method, idempotencyKey: 'complete-' + b.id, notes: method === 'cash' ? 'Cash received when the booking was completed' : 'Interac e-transfer received when the booking was completed' }) });
          const paid = await pay.json().catch(() => ({}));
          if (!pay.ok) throw new Error(paid.error ?? 'The cash payment could not be recorded.');
        }
      }
      if (await updateStatus(b.id, 'completed')) setCompleting(null);
      else setCompleteError('The booking could not be completed. See the message above.');
    } catch (e) {
      setCompleteError(e instanceof Error ? e.message : 'The booking could not be completed.');
    } finally {
      setCompleteBusy(false);
      void loadInvoices();
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
          description: `Job from booking ${bookingRef(booking)}`,
        }),
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not start the job for this booking');
        return;
      }
      const created = await r.json();
      setNotice(`Job #${created.id.slice(0, 8)} is linked to booking ${bookingRef(booking)}.`);
      if (onJobSpawned) await onJobSpawned();
    } catch {
      setError('Could not start the job.');
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
          status: 'confirmed',
          bookingTime: parsedTime,
          notes,
          serviceAddress: serviceAddress.trim(),
        }),
      });
      const body = await r.json();
      if (!r.ok) {
        setError(body.error ?? Object.values(body.errors ?? {}).join('. ') ?? 'Could not create booking');
        return;
      }
      setShowCreateForm(false);
      setNotice(`Booking ${bookingRef(body)} created successfully.`);
      setCustomerId('');
      setVehicleId('');
      setServiceId('');
      setAddonIds([]);
      setReservationId('');
      setBookingTime('');
      setNotes('');
      setServiceAddress('');
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

  const serviceChoices = [...new Set(bookings.map((b) => b.serviceName))].sort();
  const filtered = bookings.filter((b) => {
    const q = customerQ.trim().toLowerCase();
    const idq = idQ.trim().replace(/^#/, '').toLowerCase(), tq = textQ.trim().toLowerCase();
    return (!serviceF || b.serviceName === serviceF) && (!q || [b.customerName, b.customerEmail, b.customerPhone].some((v) => (v ?? '').toLowerCase().includes(q)))
      && (!idq || String(b.number ?? '').startsWith(idq) || b.id.toLowerCase().startsWith(idq)) && (!tq || [b.customerName, b.customerEmail, b.customerPhone, b.serviceName].some((v) => (v ?? '').toLowerCase().includes(tq)));
  });
  // Ascending or descending by any column; with no column chosen the list keeps the order it arrived in.
  const owedOf = (id: string) => { const i = invoices[id]; return i && i.status !== 'void' ? i.totalCents - i.paidCents : -1; };
  const shown = !sortKey ? filtered : [...filtered].sort((x, y) => {
    let c = 0;
    if (sortKey === 'id') c = (x.number ?? 0) - (y.number ?? 0);
    else if (sortKey === 'date') c = x.bookingTime.localeCompare(y.bookingTime);
    else if (sortKey === 'created') c = x.createdAt.localeCompare(y.createdAt);
    else if (sortKey === 'customer') c = x.customerName.localeCompare(y.customerName, undefined, { sensitivity: 'base' });
    else if (sortKey === 'service') c = x.serviceName.localeCompare(y.serviceName, undefined, { sensitivity: 'base' });
    else if (sortKey === 'status') c = (STATUS_LABEL[x.status] ?? x.status).localeCompare(STATUS_LABEL[y.status] ?? y.status);
    else if (sortKey === 'payment') c = owedOf(x.id) - owedOf(y.id);
    return (c || (x.number ?? 0) - (y.number ?? 0)) * (sortDir === 'asc' ? 1 : -1);
  });
  function cycleSort(k: SortKey) {
    setPage(1);
    if (sortKey !== k) { setSortKey(k); setSortDir('asc'); } else if (sortDir === 'asc') setSortDir('desc'); else setSortKey('');
  }
  const sortHead = (k: SortKey, label: string) => (
    <button type="button" className={t.sortBtn} aria-label={'Sort by ' + label} aria-sort={sortKey === k ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'} onClick={() => cycleSort(k)}>{label}<span aria-hidden="true">{sortKey === k ? (sortDir === 'asc' ? ' ↑' : ' ↓') : ''}</span></button>
  );
  const pageCount = Math.max(1, Math.ceil(shown.length / perPage));
  const pageNow = Math.min(page, pageCount);
  const pageRows = shown.slice((pageNow - 1) * perPage, pageNow * perPage);
  const lengthText = (m?: number) => (!m ? '—' : m % 1440 === 0 ? m / 1440 + (m === 1440 ? ' Day' : ' Days') : m % 60 === 0 ? m / 60 + (m === 60 ? ' Hour' : ' Hours') : m + ' Mins');
  const stamp = (iso: string) => new Date(iso).toLocaleString('en-CA', { year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
  function exportCsv() {
    const rows = (picked.size ? shown.filter((b) => picked.has(b.id)) : shown);
    const lead = ['=', '+', '-', '@', String.fromCharCode(9), String.fromCharCode(13)];
    const cell = (v: string) => { let x = String(v ?? ''); if (lead.includes(x.charAt(0)) && !/^-?\$?[\d,.]+$/.test(x)) x = "'" + x; return '"' + x.replaceAll('"', '""') + '"'; };
    const lines = [['ID', 'Date', 'Customer', 'Phone', 'Email', 'Service', 'Duration', 'Status', 'Payment', 'Created'], ...rows.map((b) => {
      const inv = invoices[b.id]; const owed = inv && inv.status !== 'void' ? inv.totalCents - inv.paidCents : 0;
      return [bookingRef(b), stamp(b.bookingTime), b.customerName, b.customerPhone, b.customerEmail, b.serviceName, lengthText(b.serviceId ? lengths[b.serviceId] : undefined), STATUS_LABEL[b.status] ?? b.status, inv ? (inv.status === 'void' ? 'Void' : owed <= 0 && inv.totalCents > 0 ? money(inv.paidCents) + ' paid' : money(owed) + ' due') : 'No invoice', stamp(b.createdAt)];
    })].map((r) => r.map(cell).join(','));
    const blob = new Blob([String.fromCharCode(0xfeff) + lines.join(String.fromCharCode(13, 10))], { type: 'text/csv;charset=utf-8' });
    const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = 'appointments.csv'; a.click(); URL.revokeObjectURL(a.href);
  }
  function shareUrl() {
    const url = window.location.origin + '/book-appointment';
    navigator.clipboard?.writeText(url).then(() => setNotice('Booking page link copied: ' + url), () => setNotice('Booking page link: ' + url));
  }
  return (
    <section className="panel detail-card">
      <div className={t.pageHead}>
        <h2>Manage Appointments</h2>
        <div className={t.headActions}>
          {canCreate && (
            <button className={`primary ${t.addNew}`} disabled={busy} onClick={() => setShowCreateForm((prev) => !prev)}>
              {showCreateForm ? 'Cancel' : '+ Add New'}
            </button>
          )}
          <button type="button" className="secondary" onClick={shareUrl}>Share URL</button>
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
              <Select required disabled={busy} value={customerId} onChange={(e) => {setCustomerId(e.target.value); setVehicleId('');}}>
                <option value="">Select customer</option>
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.phone || c.email})
                  </option>
                ))}
              </Select>
            </label>
            <label className="form-field">
              Customer vehicle or equipment (optional)
              <Select disabled={busy || !customerId} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)}>
                <option value="">No equipment selected</option>
                {vehicles.filter((v) => v.customerId === customerId).map((v) => (
                  <option key={v.id} value={v.id}>{v.description}{v.identifier ? ` · ${v.identifier}` : ''}</option>
                ))}
              </Select>
              <small>Only equipment owned by the selected customer and visible with your permissions is listed.</small>
            </label>
            <label className="form-field">
              Main service
              <Select
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
              </Select>
            </label>
          </div>

          <div className="management-form">
            <label className="form-field">
              Intake channel
              <Select disabled={busy} value={channel} onChange={(e) => setChannel(e.target.value as any)}>
                <option value="internal">Internal / Shop</option>
                <option value="phone">Phone call</option>
                <option value="walk_in">Walk-in</option>
                <option value="dealership">Dealership / Fleet</option>
              </Select>
            </label>
            <label className="form-field">
              Appointment date & time
              <DateTimePicker
                required
                disabled={busy}
                value={bookingTime}
                onChange={(e) => setBookingTime(e.target.value)}
              />
            </label>
            {reservations.length > 0 && (
              <label className="form-field">
                Capacity slot (optional)
                <Select disabled={busy} value={reservationId} onChange={(e) => setReservationId(e.target.value)}>
                  <option value="">None</option>
                  {reservations
                    .filter((r) => !r.cancelledAt)
                    .map((r) => (
                      <option key={r.id} value={r.id}>
                        {new Date(r.start).toLocaleString('en-CA')} ({r.durationMinutes} min)
                      </option>
                    ))}
                </Select>
              </label>
            )}
          </div>

          {serviceId && services.some((s) => s.kind === 'addon' && (s.parentIds?.includes(serviceId) ?? s.parentId === serviceId)) && (
            <fieldset disabled={busy} style={{margin: '1rem 0'}}>
              <legend>Select add-ons</legend>
              {services
                .filter((s) => s.kind === 'addon' && (s.parentIds?.includes(serviceId) ?? s.parentId === serviceId))
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
            Service address (mobile detailing only)
            <input type="text" maxLength={300} autoComplete="street-address" placeholder="Leave empty for in-shop work" value={serviceAddress} onChange={(e) => setServiceAddress(e.target.value)} />
          </label>

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

      <div className={t.filters}>
        <label>Appointment date
          <span className={t.dates}>
            <DatePicker value={dateFrom} max={dateTo || undefined} onChange={(e) => { setBookings([]); setBookingsLoading(true); setPage(1); setDateFrom(e.target.value); }} aria-label="From date" />
            <DatePicker value={dateTo} min={dateFrom || undefined} onChange={(e) => { setBookings([]); setBookingsLoading(true); setPage(1); setDateTo(e.target.value); }} aria-label="To date" />
          </span>
        </label>
        <label>Customer name<input type="search" value={customerQ} onChange={(e) => { setPage(1); setCustomerQ(e.target.value); }} placeholder="Start typing to fetch customer" /></label>
        <label>Service
          <Select value={serviceF} onChange={(e) => { setPage(1); setServiceF(e.target.value); }} aria-label="Service"><option value="">Select service</option>{serviceChoices.map((n) => <option key={n} value={n}>{n}</option>)}</Select>
        </label>
        <label>Status
          <Select value={statusFilter} onChange={(e) => { bookingsReqIdRef.current++; setBookings([]); setBookingsLoading(true); setPage(1); setStatusFilter(e.target.value); }} aria-label="Status">
            <option value="">Select status</option><option value="confirmed">Approved</option><option value="completed">Completed</option><option value="cancelled">Cancelled</option>
          </Select>
        </label>
      </div>
      <div className={t.filters2}>
        <input type="search" aria-label="Appointment ID" value={idQ} onChange={(e) => { setPage(1); setIdQ(e.target.value); }} placeholder="Appointment ID" />
        <input type="search" aria-label="Search" value={textQ} onChange={(e) => { setPage(1); setTextQ(e.target.value); }} placeholder="Search for customers, services…" />
        <button type="button" className="secondary" onClick={() => { setDateFrom(''); setDateTo(''); setCustomerQ(''); setServiceF(''); setStatusFilter(''); setIdQ(''); setTextQ(''); setSortKey(''); setPage(1); setPicked(new Set()); }}>Reset</button>
        <button type="button" className={`primary ${t.apply}`} disabled={bookingsLoading} onClick={() => void loadBookings()}>Apply</button>
        <button type="button" className={`primary ${t.exportBtn}`} disabled={!shown.length} onClick={exportCsv}>Export{picked.size ? ' (' + picked.size + ')' : ''}</button>
      </div>
      <div className={t.table} role="table" aria-label="Appointments">
        <div className={t.head} role="row">
          <span /><input type="checkbox" aria-label="Select all on this page" checked={pageRows.length > 0 && pageRows.every((b) => picked.has(b.id))} onChange={(e) => setPicked((prev) => { const next = new Set(prev); for (const b of pageRows) { if (e.target.checked) next.add(b.id); else next.delete(b.id); } return next; })} />
          {sortHead('id', 'ID')}{sortHead('date', 'Date')}{sortHead('customer', 'Customer')}{sortHead('service', 'Service')}<span>Duration</span>{sortHead('status', 'Status')}{sortHead('payment', 'Payment')}{sortHead('created', 'Created date')}
        </div>
        {pageRows.map((b) => {
          const inv = invoices[b.id];
          const owed = inv && inv.status !== 'void' ? inv.totalCents - inv.paidCents : 0;
          const isOpen = open.has(b.id);
          const final = b.status === 'completed' || b.status === 'cancelled'; // finished or cancelled: actions stay hidden, but the status can still be corrected
          return (
            <div key={b.id} className={t.item}>
              <div className={t.row} role="row">
                <button type="button" className={t.chev} aria-expanded={isOpen} aria-label={isOpen ? 'Hide details' : 'Show details'} onClick={() => setOpen((prev) => { const next = new Set(prev); if (next.has(b.id)) next.delete(b.id); else next.add(b.id); return next; })}><ChevronDown size={15} aria-hidden="true" /></button>
                <input type="checkbox" aria-label={`Select appointment ${bookingRef(b)}`} checked={picked.has(b.id)} onChange={(e) => setPicked((prev) => { const next = new Set(prev); if (e.target.checked) next.add(b.id); else next.delete(b.id); return next; })} />
                <span className={t.id}>{bookingRef(b)}</span>
                <span>{formatTime(b.bookingTime)}</span>
                <span><strong>{b.customerName}</strong><br /><span className={t.muted}>{b.customerPhone || b.customerEmail}</span></span>
                <span>{b.serviceName}{b.addons?.length > 0 && <span className={t.muted} style={{ whiteSpace: 'nowrap' }}> + {b.addons.length}</span>}</span>
                <span>{lengthText(b.serviceId ? lengths[b.serviceId] : undefined)}</span>
                <span className={t.status}>
                  {canManage ? (
                    <Select value={b.status === 'pending' ? 'pending' : b.status === 'completed' || b.status === 'cancelled' ? b.status : 'confirmed'} disabled={busy} aria-label="Change status" onChange={(e) => { const v = e.target.value; if (v === 'completed') { setCompleteError(''); setCompleting(b); } else if (v === 'cancelled') { setCancellationReason(''); setError(''); setCancellingBooking(b); } else if (v === 'confirmed' && b.status !== 'confirmed') void updateStatus(b.id, 'confirmed'); }}>
                      {b.status === 'pending' && <option value="pending">Pending</option>}
                      <option value="confirmed">Approved</option><option value="completed">Completed</option><option value="cancelled">Cancelled</option>
                    </Select>
                  ) : <span className={`status ${b.status === 'completed' ? 'progress' : b.status === 'cancelled' ? 'quote' : 'scheduled'}`}>{STATUS_LABEL[b.status] ?? b.status}</span>}
                </span>
                <span>{inv ? (inv.status === 'void' ? <span className={t.muted}>Void</span> : owed <= 0 && inv.totalCents > 0 ? <><strong>{money(inv.paidCents)}</strong> <span className={t.muted}>paid</span></> : <><strong>{money(owed)}</strong> <span className={t.muted}>due</span></>) : <span className={t.muted}>No invoice</span>}</span>
                <span className={t.muted}>{stamp(b.createdAt)}</span>
              </div>
              {isOpen && (
                <div className={t.detail}>
                  <div className={t.detailTop}>
                    <h3>{b.serviceName}{inv && <span className={t.price}>{money(inv.totalCents)}</span>}</h3>
                    <div className={t.actions}>
                      {inv && <button className="secondary" onClick={() => viewInvoice(b)}>Invoice</button>}
                      <button className="secondary" onClick={() => viewEvents(b)}>History</button>
                      {canManage && !final && b.status !== 'pending' && <button className="primary" disabled={busy} onClick={() => { setCompleteError(''); void loadInvoices().then(() => setCompleting(b)); }}>Mark completed</button>}
                      {canManage && !final && !b.reservationId && <button className="secondary" disabled={busy} onClick={() => { setMoveError(''); setMoveTime(''); setMoving(b); }}>Reschedule</button>}
                      {canManage && !final && b.reservationId && <button className="secondary" disabled={busy} onClick={(event) => { rescheduleTrigger.current = event.currentTarget; setReschedulingBooking(b); }}>Reschedule</button>}
                      {canManage && !final && <button className="secondary" disabled={busy} onClick={(event) => { cancellationTrigger.current = event.currentTarget; setCancellationReason(''); setError(''); setCancellingBooking(b); }}>Cancel</button>}
                      {canCreateWorkOrders && !final && <button className="secondary" disabled={busy} onClick={() => createWorkOrder(b)}>Start the job</button>}
                    </div>
                  </div>
                  <div className={t.detailCols}>
                    <div><h4>Booking</h4><dl className={t.kv}><dt>Date</dt><dd>{formatTime(b.bookingTime)}</dd><dt>Booked</dt><dd>{b.channel === 'online' ? payWay(invoices[b.id]) : b.channel === 'walk_in' ? 'Walk-in' : b.channel === 'phone' ? 'Phone' : 'By staff'}</dd>{b.addons?.length > 0 && <><dt>Add-ons</dt><dd>{b.addons.map((a) => a.name).join(', ')}</dd></>}{b.notes.replace(/Imported from BookingPress[^\n]*/g, '').trim() && <><dt>Notes</dt><dd>{b.notes.replace(/Imported from BookingPress[^\n]*/g, '').trim()}</dd></>}{b.cancellationReason && <><dt>Cancelled</dt><dd>{b.cancellationReason}</dd></>}</dl></div>
                    <div><h4>Customer</h4><dl className={t.kv}><dt>Name</dt><dd>{b.customerName}</dd>{b.customerEmail && <><dt>Email</dt><dd>{b.customerEmail}</dd></>}{b.customerPhone && <><dt>Phone</dt><dd>{b.customerPhone}</dd></>}<dt>Service location</dt><dd>{b.serviceAddress ? b.serviceAddress : 'At the shop'}</dd></dl></div>
                    <div><h4>Payment</h4><dl className={t.kv}>{inv ? <><dt>Invoice</dt><dd>{inv.invoiceNumber}{inv.status === 'draft' ? ' (draft)' : ''}</dd>{(inv.discountCents ?? 0) > 0 && <><dt>Discount</dt><dd>−{money(inv.discountCents ?? 0)} ({inv.couponCode})</dd></>}{(inv.taxCents ?? 0) > 0 && <><dt>Subtotal</dt><dd>{money(inv.totalCents - (inv.taxCents ?? 0))}</dd><dt>Tax</dt><dd>{money(inv.taxCents ?? 0)}</dd></>}<dt>Total</dt><dd>{money(inv.totalCents)}</dd><dt>Paid</dt><dd>{money(inv.paidCents)}</dd><dt>Pending</dt><dd>{money(Math.max(0, owed))}</dd></> : <><dt>Invoice</dt><dd>None yet</dd></>}</dl>
                      {inv && inv.status === 'draft' && canManage && (
                        <div style={{ display: 'flex', gap: 8, marginTop: 10, flexWrap: 'wrap' }}>
                          {(inv.discountCents ?? 0) > 0
                            ? <button type="button" className="secondary" disabled={couponBusy} onClick={() => void couponAction(b, true)}>Remove coupon</button>
                            : <><input aria-label="Coupon code" placeholder="Coupon code" value={couponCodes[b.id] ?? ''} maxLength={32} onChange={(e) => setCouponCodes((prev) => ({ ...prev, [b.id]: e.target.value.toUpperCase() }))} style={{ flex: '1 1 140px', minWidth: 0, padding: '9px 12px', borderRadius: 12, border: '1px solid var(--glass-line, #3a3a42)', background: 'transparent', color: 'inherit', font: 'inherit' }} /><button type="button" className="secondary" disabled={couponBusy || !(couponCodes[b.id] ?? '').trim()} onClick={() => void couponAction(b, false)}>Apply</button></>}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
          );
        })}
        {!bookingsLoading && shown.length === 0 && <p className={t.empty}>No appointments match these filters.</p>}
      </div>
      <div className={t.pager}>
        <span className={t.muted}>{bookingsLoading ? 'Loading appointments…' : <>Showing <strong>{pageRows.length}</strong> out of <strong>{shown.length}</strong></>}</span>
        <label className={t.perPage}>Per page
          <Select value={String(perPage)} onChange={(e) => { setPerPage(Number(e.target.value)); setPage(1); }} aria-label="Per page">{[10, 20, 50, 100].map((n) => <option key={n} value={n}>{n}</option>)}</Select>
        </label>
        <span className={t.pages}>
          <button type="button" className="secondary" disabled={pageNow <= 1} onClick={() => setPage(pageNow - 1)} aria-label="Previous page">‹</button>
          <span aria-live="polite">{pageNow} / {pageCount}</span>
          <button type="button" className="secondary" disabled={pageNow >= pageCount} onClick={() => setPage(pageNow + 1)} aria-label="Next page">›</button>
        </span>
      </div>

      {moving && (
        <FormPopup label="Reschedule appointment" width={520} onClose={() => { if (!moveBusy) setMoving(null); }}>
          <div style={{ display: 'grid', gap: 14 }}>
            <h3 style={{ margin: 0 }}>Reschedule “{moving.serviceName}”</h3>
            <p>Currently {formatTime(moving.bookingTime)}. This appointment holds no calendar slot, so choose the new date and time yourself.</p>
            <label className="form-field">New date and time<DateTimePicker value={moveTime} onChange={(e) => setMoveTime(e.target.value)} /></label>
            {moveError && <p role="alert" className="field-error">{moveError}</p>}
            <div className="form-actions" style={{ justifyContent: 'flex-start' }}>
              <button className="primary" disabled={moveBusy || !moveTime} onClick={() => void saveMove()}>{moveBusy ? 'Saving…' : 'Move appointment'}</button>
              <button className="secondary" disabled={moveBusy} onClick={() => setMoving(null)}>Back</button>
            </div>
          </div>
        </FormPopup>
      )}
      {completing && (() => { const inv = invoices[completing.id]; const owed = inv ? inv.totalCents - inv.paidCents : 0; return (
        <FormPopup label="Complete booking" width={560} onClose={() => { if (!completeBusy) setCompleting(null); }}>
          <div style={{ display: 'grid', gap: 14 }}>
            <h3 style={{ margin: 0 }}>Complete “{completing.serviceName}”</h3>
            {!inv && <p>This booking has no invoice, so there is nothing to collect here.</p>}
            {inv && owed > 0 && <p>{money(owed)} is still owed on invoice {inv.invoiceNumber}. {inv.status === 'draft' ? 'Recording a payment will issue the invoice first.' : ''}</p>}
            {inv && owed <= 0 && inv.totalCents > 0 && <p>Invoice {inv.invoiceNumber} is already paid.</p>}
            {completeError && <p role="alert" className="field-error">{completeError}</p>}
            <div className="form-actions" style={{ justifyContent: 'flex-start', flexWrap: 'wrap' }}>
              {inv && owed > 0 && <button className="primary" disabled={completeBusy} onClick={() => void finishBooking(completing, 'cash')}>{completeBusy ? 'Saving…' : `Cash received · ${money(owed)}`}</button>}
              {inv && owed > 0 && <button className="primary" disabled={completeBusy} onClick={() => void finishBooking(completing, 'bank_transfer')}>E-transfer received · {money(owed)}</button>}
              {!(inv && owed > 0) && <button className="primary" disabled={completeBusy} onClick={() => void finishBooking(completing, '')}>Complete</button>}
              <button className="secondary" disabled={completeBusy} onClick={() => setCompleting(null)}>Back</button>
            </div>
          </div>
        </FormPopup>
      ); })()}
      {reschedulingBooking && <BookingRescheduler base={base} booking={reschedulingBooking} onSaved={loadBookings} onClose={() => setReschedulingBooking(null)}/>}
      {cancellingBooking && (
        <dialog ref={cancellationDialog} aria-labelledby="booking-cancel-title" onCancel={(event) => {event.preventDefault(); if (!busy) closeCancellation();}} style={{maxWidth:'min(32rem, calc(100vw - 2rem))',padding:'1.5rem',background:'var(--surface, #fff)',color:'var(--text, var(--text, #1a0b0d))',border:'1px solid var(--border, #f0dcde)',borderRadius:'12px'}}>
          <form onSubmit={(event) => {event.preventDefault(); updateStatus(cancellingBooking.id, 'cancelled', cancellationReason);}}>
            <h3 id="booking-cancel-title">Cancel booking</h3>
            <p>{cancellingBooking.customerName} · {cancellingBooking.serviceName}</p>
            <p>This cancels the booking and frees up the time. Payments and jobs are not changed automatically.</p>
            <label>Why is it being cancelled? (optional)<textarea autoFocus maxLength={2000} value={cancellationReason} disabled={busy} onChange={(event) => setCancellationReason(event.target.value)} style={{display:'block',width:'100%',boxSizing:'border-box'}} /></label>
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
              background: 'var(--surface)',
              padding: '1.5rem',
              borderRadius: '8px',
              maxWidth: '550px',
              width: '90%',
              border: '1px solid var(--border)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <h3>Booking Audit Trail · {bookingRef(selectedBooking)}</h3>
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
