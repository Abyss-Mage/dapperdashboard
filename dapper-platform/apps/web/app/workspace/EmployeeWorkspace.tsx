'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import EmployeeInvoices from './EmployeeInvoices';
import StaffBookingWizard from './StaffBookingWizard';
import { printInvoiceDocument, usePrintTemplate, type PrintInvoice } from './InvoicePrint';
import AddonPicker from './AddonPicker';
import gsap from 'gsap';
import { useGSAP } from '@gsap/react';
import { ArrowLeft, ArrowUpRight, Banknote, Building2, CalendarDays, Car, Coffee, Images, Menu, Tags, UserRound, X, Check, ChevronLeft, ChevronRight, Clock3, CreditCard, House, LogOut, MapPin, Plus, RefreshCw, ShieldCheck, Trash2, UserRoundPlus, FileText } from 'lucide-react';
import { availabilityError, dateOffset, readableDay, worksiteDate, contactFormatError, type AvailabilityInterval } from './employee_helpers';
import styles from './employee.module.css';
import EmployeeCalendar from './EmployeeCalendar';
import { RecentPunches, ShiftHero, StatCards } from './EmployeeHome';
import DocumentsCard from './EmployeeDocuments';
import EmployeeAccount from './EmployeeAccount';
import OnboardingGate, { type Onboarding } from './OnboardingGate';
import { MfaSetup } from './AccountSecurity';
import { getClockInFix } from './clockLocation';
import { BreakHistory, MyUploads, UploadComposer } from './EmployeeUploads';
import SquareCardPayment from './SquareCardPayment';
import ThemeToggle from '../ThemeToggle';
import Select from '../GlassSelect';
import { DatePicker, TimePicker } from '../Pickers';

type Worksite = { principalId: string; organizationId: string; locationId: string; locationName: string; dealershipId: string; dealershipName: string; timezone: string; version: number };
type Booking = { id: string; customerName: string; customerEmail: string; customerPhone: string; serviceName: string; bookingTime: string; status: string; addons: { name: string }[] };
type Shift = { id: string; status: 'clocked_in' | 'on_break' | 'clocked_out'; clockInAt: string; clockOutAt?: string; currentBreakStart?: string; totalBreakMinutes?: number; totalWorkMinutes: number };
type Availability = { weekStart: string; timezone: string; intervals: AvailabilityInterval[]; notes: string; version: number; submittedAt: string | null };
type Desk = { employee: string; assignment: Worksite | null; actions: string[]; bookings: Booking[]; attendance: Shift[]; availability: Availability | null; generatedAt: string };
type Service = { id: string; name: string; kind: string; parentId: string | null; archived: boolean };
type Invoice = { id: string; invoiceNumber: string; status: string; currency: string; dueCents: number; subtotalCents: number; taxCents: number; totalCents: number; paidCents: number; customerName: string; issuedAt?: string; issuerSnapshot?: { legalName: string; address: string; taxRegistration: string }; customerSnapshot?: { name: string; email: string; phone: string }; items: { description: string; quantity: number; totalPriceCents: number }[] };
const mobileTabs: string[] = ['today', 'walk-in', 'cars'];
type Tab = 'today' | 'bookings' | 'walk-in' | 'invoices' | 'tags' | 'cars' | 'breaks' | 'uploads' | 'shifts' | 'account';
const tabs = [{ id: 'today', label: 'Dashboard', icon: House }, { id: 'bookings', label: 'Schedule', icon: CalendarDays }, { id: 'walk-in', label: 'Walk-in', icon: UserRoundPlus }, { id: 'invoices', label: 'Invoices', icon: FileText }, { id: 'tags', label: 'Dealership Tags', icon: Tags }, { id: 'cars', label: 'Customer Cars', icon: Car }, { id: 'breaks', label: 'Break History', icon: Coffee }, { id: 'uploads', label: 'My Uploads', icon: Images }, { id: 'shifts', label: 'Availability', icon: Clock3 }] as const;
const money = (amount: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(amount / 100);
gsap.registerPlugin(useGSAP);
async function responseJSON<T>(response: Response): Promise<T> {
  // A successful login (and other no-content replies) has an empty body by design; that is not an error.
  if (response.ok && response.status === 204) return undefined as T;
  const body = await response.json().catch(() => null);
  if (!response.ok) throw Object.assign(new Error(body?.error ?? (response.status === 403 ? 'Your access has changed. Contact your administrator.' : 'The service is unavailable. Please try again.')), { code: body?.code as string | undefined, mfaRequired: !!body?.mfaRequired });
  if (body === null) throw new Error('The service returned an incomplete response. Please refresh.');
  return body as T;
}

export default function EmployeeWorkspace() {
  const motionRoot = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDialogElement>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [desk, setDesk] = useState<Desk | null>(null), [signedIn, setSignedIn] = useState(false), [ready, setReady] = useState(false);
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [email, setEmail] = useState(''), [password, setPassword] = useState(''), [tab, setTab] = useState<Tab>('today'), [day, setDay] = useState(''), [code, setCode] = useState(''), [needsCode, setNeedsCode] = useState(false), [mfaBlocked, setMfaBlocked] = useState(false), [onboarding, setOnboarding] = useState<Onboarding | null>(null);
  const printTemplate = usePrintTemplate('/api/v1/employee/invoice-template');
  const [checkout, setCheckout] = useState<Booking | null>(null), [invoices, setInvoices] = useState<Invoice[]>([]), [invoiceLoading, setInvoiceLoading] = useState(false), [invoiceError, setInvoiceError] = useState('');
  const [tick, setTick] = useState(0), [cashConfirmation, setCashConfirmation] = useState(false);
  const request = useRef(0), checkoutRequest = useRef(0), assignmentKey = useRef(''), heading = useRef<HTMLHeadingElement>(null), errorRef = useRef<HTMLParagraphElement>(null), paymentKeys = useRef<Record<string, string>>({});
  const refresh = useCallback(async () => {
    const generation = ++request.current; setLoading(true); setError('');
    try {
      const response = await fetch('/api/v1/employee/workspace', { cache: 'no-store' });
      if (generation !== request.current) return;
      if (response.status === 401) { checkoutRequest.current++; setSignedIn(false); setDesk(null); setCheckout(null); setInvoices([]); paymentKeys.current = {}; assignmentKey.current = ''; return; }
      const data = await responseJSON<Desk>(response);
      if (generation !== request.current) return;
      const nextKey = data.assignment ? `${data.assignment.organizationId}:${data.assignment.dealershipId}:${data.assignment.locationId}:${data.assignment.version}` : '';
      if (assignmentKey.current !== nextKey) { checkoutRequest.current++; setCheckout(null); setInvoices([]); setCashConfirmation(false); setDay(''); setTab('today'); paymentKeys.current = {}; assignmentKey.current = nextKey; }
      setSignedIn(true); setDesk(data);
      if (data.assignment) setDay(d => d || worksiteDate(data.generatedAt, data.assignment!.timezone));
    } catch (e) { if (generation === request.current) { setDesk(null); if ((e as { code?: string }).code === 'mfa_setup_required') { setMfaBlocked(true); setSignedIn(true); } else setError(e instanceof Error ? e.message : 'Unable to load your worksite.'); } }
    finally { if (generation === request.current) setLoading(false); }
  }, []);
  const loadOnboarding = useCallback(() => fetch('/api/v1/account/onboarding', { cache: 'no-store' }).then(r => r.ok ? r.json() : null).then(setOnboarding).catch(() => undefined), []);
  useEffect(() => {
    let live = true;
    fetch('/api/v1/health', { cache: 'no-store' }).then(responseJSON<{ identityProviderConfigured: boolean }>).then(h => { if (live) setReady(location.protocol === 'https:' && h.identityProviderConfigured); }).catch(() => { if (live) setReady(false); });
    refresh(); return () => { live = false; request.current++; checkoutRequest.current++; };
  }, [refresh]);
  useEffect(() => { const timer = setInterval(() => setTick(Date.now()), 60_000); return () => clearInterval(timer); }, []);
  useEffect(() => { if (signedIn && desk) void loadOnboarding(); }, [signedIn, desk?.generatedAt, loadOnboarding]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (error && !busy && !loading) errorRef.current?.focus(); }, [error, busy, loading]);
  const gate = signedIn && !!desk && !!onboarding && !onboarding.complete;
  const site = desk?.assignment, shift = desk?.attendance.find(s => s.status !== 'clocked_out'), permitted = (action: string) => desk?.actions.includes(action) ?? false;
  useGSAP(() => {
    const media = gsap.matchMedia();
    media.add('(prefers-reduced-motion: no-preference)', () => {
      const title = motionRoot.current?.querySelector('h1');
      if (title) gsap.fromTo(title, { y: 6 }, { y: 0, duration: 0.18, ease: 'power3.out', overwrite: true });
    });
    return () => media.revert();
  }, { scope: motionRoot, dependencies: [tab, checkout?.id, !!site, signedIn], revertOnUpdate: true });
  useGSAP(() => {
    const media = gsap.matchMedia();
    media.add('(prefers-reduced-motion: no-preference)', () => {
      const feedback = motionRoot.current?.querySelector('[role="status"]');
      if (notice && feedback) gsap.fromTo(feedback, { y: 4 }, { y: 0, duration: 0.16, ease: 'power3.out', overwrite: true });
    });
    return () => media.revert();
  }, { scope: motionRoot, dependencies: [notice], revertOnUpdate: true });
  const formatTime = (value: string) => new Intl.DateTimeFormat('en-CA', { hour: 'numeric', minute: '2-digit', timeZone: site?.timezone ?? 'UTC' }).format(new Date(value));
  const minutes = shift ? shift.totalWorkMinutes + (shift.status === 'clocked_in' ? Math.max(0, Math.floor(((tick || Date.now()) - new Date(desk!.generatedAt).getTime()) / 60_000)) : 0) : 0;
  const today = site ? worksiteDate(desk!.generatedAt, site.timezone) : '';
  const agenda = desk?.bookings.filter(b => site && worksiteDate(b.bookingTime, site.timezone) === (tab === 'today' ? today : day)) ?? [];
  function navigate(next: Tab) { checkoutRequest.current++; setTab(next); setCheckout(null); setInvoices([]); setInvoiceError(''); setCashConfirmation(false); setNotice(''); setTimeout(() => heading.current?.focus(), 0); }
  async function signIn(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setError('');
    try { await responseJSON(await fetch('/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(needsCode ? { email, password, code: code.trim() } : { email, password }) })); setPassword(''); setCode(''); setNeedsCode(false); await refresh(); }
    catch (e) { if ((e as { mfaRequired?: boolean }).mfaRequired) { setNeedsCode(true); setCode(''); } setError(e instanceof Error ? e.message : 'Sign-in failed. Try again.'); } finally { setBusy(false); }
  }
  async function signOut() {
    setBusy(true); setError('');
    try { const r = await fetch('/api/v1/auth/logout', { method: 'POST' }); if (!r.ok) throw new Error('Sign-out was not confirmed. Please try again.'); request.current++; checkoutRequest.current++; assignmentKey.current = ''; setDesk(null); setSignedIn(false); setCheckout(null); setInvoices([]); setDay(''); setPassword(''); setMfaBlocked(false); setOnboarding(null); setNeedsCode(false); paymentKeys.current = {}; }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function punch(action: string) {
    setBusy(true); setError(''); setNotice('');
    try { const location = action === 'clock-in' ? await getClockInFix() : null; await responseJSON(await fetch(`/api/v1/employee/attendance/${action}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(location ? { location } : {}) })); setNotice(action === 'clock-in' ? 'You are clocked in.' : action === 'clock-out' ? 'You are clocked out.' : action === 'start-break' ? 'Break started.' : 'Break ended.'); await refresh(); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function openCheckout(booking: Booking) {
    const generation = ++checkoutRequest.current;
    setCheckout(booking); setInvoices([]); setInvoiceError(''); setInvoiceLoading(true); setCashConfirmation(false);
    try { const data = await responseJSON<{ items: Invoice[] }>(await fetch(`/api/v1/employee/bookings/${booking.id}/invoices`, { cache: 'no-store' })); if (generation === checkoutRequest.current) setInvoices(data.items); }
    catch (e) { if (generation === checkoutRequest.current) setInvoiceError((e as Error).message); } finally { if (generation === checkoutRequest.current) setInvoiceLoading(false); }
  }
  async function recordCash(invoice: Invoice) {
    if (!checkout || !cashConfirmation) return; setBusy(true); setInvoiceError('');
    const key = paymentKeys.current[invoice.id] ??= crypto.randomUUID();
    const generation = checkoutRequest.current;
    try { const data = await responseJSON<{ invoice: Invoice }>(await fetch(`/api/v1/employee/bookings/${checkout.id}/cash`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ invoiceId: invoice.id, amountCents: invoice.dueCents, idempotencyKey: key }) })); if (generation === checkoutRequest.current) { setInvoices(items => items.map(i => i.id === invoice.id ? { ...i, ...data.invoice } : i)); setCashConfirmation(false); setNotice('Cash receipt recorded. Your invoice balance has been updated.'); } }
    catch (e) { if (generation === checkoutRequest.current) setInvoiceError((e as Error).message); } finally { setBusy(false); }
  }

  return <div className={styles.app} ref={motionRoot}>
    <a href="#employee-content" className={styles.skip}>Skip to your workspace</a>
    <aside className={styles.sidebar} aria-label="Employee navigation"><a className={styles.brand} href="/workspace" aria-label="Dapper Detailing Pros, Employee workspace"><img className={styles.brandLogo} src="/site/assets/logo.webp" alt="" width={44} height={44} /><span className={styles.brandText}><strong>Dapper Detailing Pros</strong><em>Employee workspace</em></span></a>
      {signedIn && site && !gate && <nav>{tabs.map(({ id, label, icon: Icon }) => <button key={id} className={tab === id && !checkout ? styles.navActive : styles.navItem} aria-current={tab === id && !checkout ? 'page' : undefined} onClick={() => navigate(id)}><Icon size={20} aria-hidden="true" />{label}</button>)}</nav>}
      <div className={styles.sidebarFoot}><ShieldCheck size={18} aria-hidden="true" /><p>Your worksite and access are assigned by your administrator.</p></div>
    </aside>
    <div className={styles.workspace}>
      <header className={styles.masthead}><div className={styles.site}><Building2 size={20} aria-hidden="true" /><div><strong>{site?.locationName ?? 'Dapper Detailing'}</strong><span>{site ? (site.dealershipName && site.dealershipName !== site.locationName ? `${site.dealershipName} · ${site.timezone}` : site.timezone) : 'Employee access'}</span></div></div><div className={styles.headerActions}><ThemeToggle className={styles.iconButton} compact />{signedIn && site && !gate && <button className={`${styles.iconButton} ${tab === 'account' && !checkout ? styles.iconActive : ''}`} aria-label="Account" aria-current={tab === 'account' && !checkout ? 'page' : undefined} onClick={() => navigate('account')}><UserRound size={19} /></button>}{signedIn && <button className={styles.iconButton} aria-label="Sign out" disabled={busy} onClick={signOut}><LogOut size={19} /></button>}</div></header>
      <main id="employee-content" className={styles.main}>
        {error && <p ref={errorRef} tabIndex={-1} role="alert" className={styles.error}>{error}</p>}
        {notice && <p role="status" className={styles.notice}><Check size={18} aria-hidden="true" />{notice}</p>}
        {loading && !desk ? <div className={styles.loading} role="status">Opening your workspace…</div> : !signedIn ? <section className={styles.login}>
          <h1>Good work starts here.</h1><p>Sign in to your workspace.</p>
          <form onSubmit={signIn}><label>Work email<input type="email" autoComplete="username" required maxLength={254} value={email} onChange={e => setEmail(e.target.value)} disabled={busy || !ready} /></label><label>Password<input type="password" autoComplete="current-password" required maxLength={1024} value={password} onChange={e => setPassword(e.target.value)} disabled={busy || !ready} /></label>{needsCode && <label>Authenticator code<input inputMode="numeric" autoComplete="one-time-code" autoFocus required maxLength={16} value={code} onChange={e => setCode(e.target.value)} disabled={busy} aria-describedby="code-hint" /><span id="code-hint" className={styles.help}>Enter the 6-digit code from your authenticator app, or a recovery code.</span></label>}<button className={styles.primary} disabled={busy || !ready}>{busy ? 'Signing in…' : needsCode ? 'Verify and sign in' : 'Sign in'}<ArrowUpRight size={18} /></button></form>
          {!ready && <p className={styles.help}>Secure sign-in is unavailable. Contact your administrator or refresh when the service is back.</p>}
        </section> : mfaBlocked ? <section className={styles.setup}><h1>Set up two-factor authentication</h1><MfaSetup required onDone={() => { setMfaBlocked(false); void refresh(); }} /></section> : !desk ? <section className={styles.setup}><h1>Your workspace could not load.</h1><p>We could not confirm your current worksite. Refresh to try again.</p><button className={styles.secondary} onClick={refresh}>Try again</button></section> : gate ? <OnboardingGate timezone={site?.timezone ?? 'America/Toronto'} onChanged={() => { void loadOnboarding(); void refresh(); }} /> : !site ? <section className={styles.setup}><Building2 size={36} strokeWidth={1.4} aria-hidden="true" /><h1>Your worksite comes first.</h1><p>Your administrator needs to assign your franchise before you can clock in, view customers or take payments.</p><button className={styles.secondary} disabled={loading} onClick={refresh}><RefreshCw size={17} />Check assignment</button></section> : <>
          <div className={styles.pageHeading}><div><h1 ref={heading} tabIndex={-1}>{checkout ? 'Payment & invoice' : tab === 'today' ? 'A clear start to your day.' : tab === 'bookings' ? 'Your customer agenda.' : tab === 'walk-in' ? 'Welcome a walk-in.' : tab === 'invoices' ? 'Bill your work.' : tab === 'tags' ? 'Post key tags.' : tab === 'cars' ? 'Inspect a customer car.' : tab === 'account' ? 'Your account.' : tab === 'breaks' ? 'Your break history.' : tab === 'uploads' ? 'Everything you posted.' : 'Make next week work.'}</h1><p>{checkout ? checkout.customerName : tab === 'today' ? readableDay(today) : tab === 'bookings' ? 'Prebooked customers at your franchise.' : tab === 'walk-in' ? 'Customer details, services and a reserved time. No OTP required.' : tab === 'invoices' ? 'Create invoices from bookings, issue them and take payment.' : tab === 'tags' ? 'Key tags for a partner dealership' : tab === 'cars' ? 'Pre and post detail inspection photos' : tab === 'account' ? desk!.employee : tab === 'breaks' ? `Recorded breaks · ${site.timezone}` : tab === 'uploads' ? 'Your dealership tags and customer car photos' : `Personal availability and recorded shifts · ${site.timezone}`}</p></div><button className={styles.iconButton} aria-label="Refresh workspace" disabled={loading} onClick={refresh}><RefreshCw size={19} className={loading ? styles.spinning : ''} /></button></div>
          {checkout ? <section className={styles.checkout}>
            <button className={styles.textButton} onClick={() => navigate('bookings')}><ArrowLeft size={17} />Back to bookings</button>
            <div className={styles.checkoutIntro}><h2>{checkout.serviceName}</h2><p>{readableDay(worksiteDate(checkout.bookingTime, site.timezone))} · {formatTime(checkout.bookingTime)}</p></div>
            {invoiceError && <p role="alert" className={styles.error}>{invoiceError}</p>}{invoiceLoading && <p role="status">Loading reviewed invoices…</p>}
            {!invoiceLoading && !invoiceError && !invoices.length && <div className={styles.empty}><h3>No invoice prepared yet.</h3><p>Ask your administrator to prepare and issue the invoice for this booking. No amount has been collected.</p></div>}
            {invoices.map(inv => <article key={inv.id} id={`employee-invoice-${inv.id}`} className={styles.invoice}><div className={styles.sectionTop}><h3>Invoice {inv.invoiceNumber}</h3><span className={styles.status}>{inv.status.replaceAll('_', ' ')}</span></div><div className={styles.invoiceIdentity}><strong>{inv.issuerSnapshot?.legalName}</strong><p>{inv.issuerSnapshot?.address}</p>{inv.issuerSnapshot?.taxRegistration && <p>Tax registration: {inv.issuerSnapshot.taxRegistration}</p>}<p>Bill to: {inv.customerSnapshot?.name ?? inv.customerName}<br />{inv.customerSnapshot?.email}<br />{inv.customerSnapshot?.phone}</p>{inv.issuedAt && <p>Issued {readableDay(worksiteDate(inv.issuedAt, site.timezone))}</p>}</div>{inv.items?.map((item, n) => <div className={styles.invoiceLine} key={n}><span>{item.description} × {item.quantity}</span><strong>{money(item.totalPriceCents)}</strong></div>)}<div className={styles.invoiceLine}><span>Subtotal</span><strong>{money(inv.subtotalCents)}</strong></div><div className={styles.invoiceLine}><span>Tax</span><strong>{money(inv.taxCents)}</strong></div><div className={styles.invoiceLine}><span>Total ({inv.currency})</span><strong>{money(inv.totalCents)}</strong></div><div className={styles.invoiceLine}><span>Paid</span><strong>{money(inv.paidCents)}</strong></div><div className={styles.total}><span>Balance due</span><strong>{money(inv.dueCents)}</strong></div>
              {inv.status !== 'draft' && inv.status !== 'void' && <button className={styles.secondary} onClick={() => printInvoiceDocument(inv as unknown as PrintInvoice, printTemplate)}>Print invoice</button>}
              {inv.dueCents > 0 && ['issued', 'partially_paid'].includes(inv.status) && <><div className={styles.paymentOptions}><h3>Payment method</h3><label className={styles.checkLabel}><input type="checkbox" checked={cashConfirmation} onChange={e => setCashConfirmation(e.target.checked)} disabled={busy} />I have received {money(inv.dueCents)} in cash.</label><button className={styles.primary} disabled={busy || !cashConfirmation || !permitted('payments.record')} onClick={() => recordCash(inv)}><Banknote size={18} />{busy ? 'Recording…' : 'Record cash received'}</button><div className={styles.reader}><CreditCard size={20} /><div><strong>Square Reader</strong><p>Reader payments are not connected on this device. Use the approved mobile integration once configured.</p></div></div></div>{permitted('payments.record') && <SquareCardPayment key={`card-${inv.id}`} infoUrl="/api/v1/employee/square" payUrl={`/api/v1/employee/bookings/${checkout.id}/square-payments`} extraBody={{ invoiceId: inv.id }} invoiceId={inv.id} dueCents={inv.dueCents} onPaid={async () => { const data = await responseJSON<{ items: Invoice[] }>(await fetch(`/api/v1/employee/bookings/${checkout.id}/invoices`, { cache: 'no-store' })); setInvoices(data.items); setNotice('Card payment confirmed by Square and recorded. Your invoice balance has been updated.'); }} />}</>}
            </article>)}
          </section> : tab === 'walk-in' ? <StaffBookingWizard key={assignmentKey.current} mode="employee" timezone={site.timezone} onBooked={async () => { setNotice('Booking saved. The time is held and nothing has been paid yet.'); await refresh(); }} /> : tab === 'invoices' ? <EmployeeInvoices key={'inv-' + assignmentKey.current} bookings={desk!.bookings} canCreate={permitted('invoices.create')} canIssue={permitted('invoices.issue')} timezone={site.timezone} onPay={(id) => { const b = desk!.bookings.find(x => x.id === id); if (b) void openCheckout(b); }} /> : tab === 'tags' ? <UploadComposer key={'tags-' + assignmentKey.current} kind="dealership_tag" timezone={site.timezone} employee={desk!.employee} bookings={desk!.bookings} onPosted={() => setNotice('')} /> : tab === 'cars' ? <UploadComposer key={'cars-' + assignmentKey.current} kind="customer_car" timezone={site.timezone} employee={desk!.employee} bookings={desk!.bookings} onPosted={() => setNotice('')} /> : tab === 'account' ? <EmployeeAccount employee={desk!.employee} site={site} actions={desk!.actions} busy={busy} onSignOut={signOut} /> : tab === 'breaks' ? <BreakHistory key={'breaks-' + shift?.status} timezone={site.timezone} /> : tab === 'uploads' ? <MyUploads timezone={site.timezone} /> : tab === 'shifts' ? <div className={styles.shiftGrid}><AvailabilityForm key={assignmentKey.current} data={desk!.availability!} onSaved={async message => { setNotice(message); await refresh(); }} /><section className={styles.history}><h2>Your recorded shifts</h2>{desk!.attendance.length ? desk!.attendance.map(s => <div className={styles.historyRow} key={s.id}><div><strong>{readableDay(worksiteDate(s.clockInAt, site.timezone), { month: 'short', day: 'numeric' })}</strong><span>{formatTime(s.clockInAt)} — {s.clockOutAt ? formatTime(s.clockOutAt) : 'Active'}</span></div><span>{Math.floor(s.totalWorkMinutes / 60)}h {s.totalWorkMinutes % 60}m</span></div>) : <p className={styles.help}>No recorded shifts at this worksite yet.</p>}</section></div> : tab === 'bookings' ? <EmployeeCalendar key={assignmentKey.current} today={today} timezone={site.timezone} onPayment={openCheckout} /> : <><div className={styles.homeTop}><ShiftHero shift={shift} siteName={site.dealershipName && site.dealershipName !== site.locationName ? `${site.locationName} · ${site.dealershipName}` : site.locationName} canRecord={permitted('attendance.record')} busy={busy || loading} onPunch={punch} /><StatCards refreshKey={desk!.generatedAt} /></div><DocumentsCard timezone={site.timezone} /><div className={styles.deskGrid}>
            <section className={styles.agenda} aria-labelledby="agenda-title"><div className={styles.sectionTop}><h2 id="agenda-title">Today’s customers</h2><span className={styles.count}>{agenda.length} shown</span></div>

              {!permitted('bookings.read') ? <div className={styles.empty}><h3>Booking access isn’t assigned.</h3><p>Contact your administrator to enable customer bookings for this worksite.</p></div> : !agenda.length ? <div className={styles.empty}><CalendarDays size={34} strokeWidth={1.3} aria-hidden="true" /><h3>A little room in the day.</h3><p>No bookings are shown for this date. Bookings appear here once an administrator links them to your franchise.</p><button className={styles.textButton} onClick={() => navigate('walk-in')}>Add a walk-in<ArrowUpRight size={17} /></button></div> : agenda.slice(0, 100).map(b => <article className={styles.booking} key={b.id}><time dateTime={b.bookingTime}>{formatTime(b.bookingTime)}</time><div><span className={styles.status}>{b.status.replaceAll('_', ' ')}</span><h3>{b.customerName}</h3><p>{b.serviceName}{b.addons?.length ? ` · ${b.addons.map(a => a.name).join(', ')}` : ''}</p><div className={styles.contacts}>{b.customerPhone && <a href={`tel:${b.customerPhone}`}>{b.customerPhone}</a>}{b.customerEmail && <a href={`mailto:${b.customerEmail}`}>{b.customerEmail}</a>}</div><button className={styles.textButton} onClick={() => openCheckout(b)}>Payment & invoice<ArrowUpRight size={17} /></button></div></article>)}
              <p className={styles.listNote}>Bookings linked to your franchise · window: previous day through next 30 days · up to 100 loaded records{desk!.bookings.length > 100 ? ' · more records may exist' : ''}</p>
            </section>
          </div><RecentPunches shifts={desk!.attendance} timezone={site.timezone} /></>}
        </>}
      </main>
      <footer className={styles.footer}>Dapper Detailing Pros<span>Personal workspace · secure session</span></footer>
    </div>
    {signedIn && site && !gate && <>
      <nav className={styles.bottomNav} aria-label="Employee navigation">
        {tabs.filter(t => mobileTabs.includes(t.id)).map(({ id, label, icon: Icon }) => <button key={id} aria-current={tab === id && !checkout ? 'page' : undefined} onClick={() => navigate(id)}><Icon size={22} aria-hidden="true" /><span>{id === 'today' ? 'Home' : label}</span></button>)}
        <button aria-haspopup="dialog" aria-expanded={menuOpen} onClick={() => { setMenuOpen(true); menuRef.current?.showModal(); }}><Menu size={22} aria-hidden="true" /><span>Menu</span></button>
      </nav>
      <dialog ref={menuRef} className={styles.menuSheet} aria-label="All sections" onClose={() => setMenuOpen(false)} onClick={e => { if (e.target === menuRef.current) menuRef.current?.close(); }}>
        <div className={styles.menuHead}><h2>Menu</h2><button className={styles.iconButton} aria-label="Close menu" onClick={() => menuRef.current?.close()}><X size={19} aria-hidden="true" /></button></div>
        <div className={styles.menuGrid}>
          {tabs.map(({ id, label, icon: Icon }) => <button key={id} aria-current={tab === id && !checkout ? 'page' : undefined} onClick={() => { menuRef.current?.close(); navigate(id); }}><Icon size={22} aria-hidden="true" /><span>{label}</span></button>)}
          <button aria-current={tab === 'account' && !checkout ? 'page' : undefined} onClick={() => { menuRef.current?.close(); navigate('account'); }}><UserRound size={22} aria-hidden="true" /><span>Account</span></button>
          <button onClick={() => { menuRef.current?.close(); void signOut(); }}><LogOut size={22} aria-hidden="true" /><span>Sign out</span></button>
        </div>
      </dialog>
    </>}
  </div>;
}

function AvailabilityForm({ data, onSaved }: { data: Availability; onSaved: (message: string) => Promise<void> }) {
  const [ranges, setRanges] = useState<AvailabilityInterval[]>(data.intervals), [notes, setNotes] = useState(data.notes), [unavailable, setUnavailable] = useState(data.version > 0 && !data.intervals.length);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''); const errorRef = useRef<HTMLParagraphElement>(null);
  useEffect(() => { setRanges(data.intervals); setNotes(data.notes); setUnavailable(data.version > 0 && !data.intervals.length); }, [data]);
  useEffect(() => { if (error && !busy) errorRef.current?.focus(); }, [error, busy]);
  async function submit(e: React.FormEvent) {
    e.preventDefault(); const issue = availabilityError(ranges, unavailable); if (issue) { setError(issue); return; } setBusy(true); setError('');
    try { await responseJSON(await fetch('/api/v1/employee/availability', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ weekStart: data.weekStart, intervals: unavailable ? [] : ranges, notes, version: data.version }) })); await onSaved('Availability submitted to your administrator. This does not create an assigned shift.'); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  return <section className={styles.availability}><div className={styles.sectionTop}><h2>Next-week availability</h2><span className={styles.status}>{data.submittedAt ? 'Submitted' : 'Not submitted'}</span></div><p>{readableDay(data.weekStart)} — {readableDay(dateOffset(data.weekStart, 6))}<br />All times in {data.timezone}. Availability is not a confirmed shift.</p><form onSubmit={submit}>
    {error && <p className={styles.error} role="alert" ref={errorRef} tabIndex={-1}>{error}</p>}
    <label className={styles.checkLabel}><input type="checkbox" checked={unavailable} onChange={e => { setUnavailable(e.target.checked); if (e.target.checked) setRanges([]); }} disabled={busy} />I’m unavailable for all of next week.</label>
    {!unavailable && Array.from({ length: 7 }, (_, day) => <fieldset className={styles.dayRange} key={day} disabled={busy}><legend>{readableDay(dateOffset(data.weekStart, day), { weekday: 'long' })}</legend><span>{readableDay(dateOffset(data.weekStart, day), { month: 'short', day: 'numeric' })}</span>{ranges.map((r, i) => r.day === day && <div className={styles.timeRange} key={i}><label>From<TimePicker value={r.start} required onChange={e => setRanges(items => items.map((v, n) => n === i ? { ...v, start: e.target.value } : v))} /></label><label>Until<TimePicker value={r.end} required onChange={e => setRanges(items => items.map((v, n) => n === i ? { ...v, end: e.target.value } : v))} /></label><button type="button" className={styles.iconButton} aria-label={`Remove ${readableDay(dateOffset(data.weekStart, day), { weekday: 'long' })} time range`} onClick={() => setRanges(items => items.filter((_, n) => n !== i))}><Trash2 size={17} /></button></div>)}<button type="button" className={styles.textButton} disabled={ranges.length >= 28} onClick={() => setRanges(items => [...items, { day, start: '', end: '' }])}><Plus size={17} />Add time</button></fieldset>)}
    <label className={styles.noteLabel}>Note to your supervisor <span>Optional</span><textarea rows={3} maxLength={1000} value={notes} disabled={busy} onChange={e => setNotes(e.target.value)} /></label><button className={styles.primary} disabled={busy}>{busy ? 'Submitting…' : data.submittedAt ? 'Update availability' : 'Submit availability'}<ArrowUpRight size={18} /></button>
  </form></section>;
}

type WalkInRule = { id: string; resourceId: string; serviceId: string; serviceName: string; addonIds: string[]; addons: { serviceId: string; name: string }[]; availableAddons?: { serviceId: string; name: string }[]; version: number; durationMinutes: number };
function WalkIn({ site, onBooked }: { site: Worksite; onBooked: () => Promise<void> }) {
  const [options, setOptions] = useState<{ resources: { id: string; name: string }[]; rules: WalkInRule[] } | null>(null), [ruleId, setRuleId] = useState(''), [error, setError] = useState(''), [loading, setLoading] = useState(true), [busy, setBusy] = useState(false);
  const [name, setName] = useState(''), [email, setEmail] = useState(''), [phone, setPhone] = useState(''), [date, setDate] = useState(''), [time, setTime] = useState(''), [picked, setPicked] = useState<string[]>([]);
  const errorRef = useRef<HTMLParagraphElement>(null), retry = useRef<{ payload: string; key: string } | null>(null);
  useEffect(() => { let live = true; fetch('/api/v1/employee/walk-in/options', { cache: 'no-store' }).then(responseJSON<{ resources: { id: string; name: string }[]; rules: WalkInRule[] }>).then(data => { if (live) setOptions(data); }).catch(e => { if (live) setError(e.message); }).finally(() => { if (live) setLoading(false); }); return () => { live = false; }; }, [site.version]);
  useEffect(() => { if (error) errorRef.current?.focus(); }, [error]);
  const rule = options?.rules.find(r => r.id === ruleId);
  useEffect(() => setPicked([]), [ruleId]);
  async function submit(e: React.FormEvent) {
    e.preventDefault(); const issue = contactFormatError(name, email, phone); if (issue) { setError(issue); return; } if (!rule) return;
    const body = { name: name.trim(), email: email.trim(), phone: phone.trim(), ruleId: rule.id, ruleVersion: rule.version, resourceId: rule.resourceId, localStart: `${date}T${time}`, ...(picked.length ? { addonIds: picked } : {}) };
    const payload = JSON.stringify(body); if (!retry.current || retry.current.payload !== payload) retry.current = { payload, key: crypto.randomUUID() };
    setBusy(true); setError('');
    try { await responseJSON(await fetch('/api/v1/employee/walk-in', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...body, requestKey: retry.current.key }) })); await onBooked(); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  return <section className={styles.walkIn}><div className={styles.verificationNote}><ShieldCheck size={23} aria-hidden="true" /><p>Email and mobile number formats are checked. No OTP is required. Contact ownership is not verified. Capacity is reserved only when the booking saves successfully.</p></div><form onSubmit={submit}>
    {error && <p className={styles.error} role="alert" ref={errorRef} tabIndex={-1}>{error}</p>}
    <fieldset disabled={busy}><legend>Customer details</legend><label>Full name<input required minLength={2} maxLength={100} autoComplete="name" value={name} onChange={e => setName(e.target.value)} /></label><div className={styles.fieldPair}><label><span>Email</span><input type="email" required maxLength={254} autoComplete="email" value={email} onChange={e => setEmail(e.target.value)} /></label><label><span>Mobile number</span><input type="tel" required maxLength={40} autoComplete="tel" placeholder="+1 416 555 0123" aria-describedby="mobile-format" value={phone} onChange={e => setPhone(e.target.value)} /></label></div><p id="mobile-format" className={styles.contactHelp}>For mobile numbers, include the country code. Spaces, brackets and dashes are accepted.</p></fieldset>
    <fieldset disabled={busy}><legend>Service & add-ons</legend>{loading ? <p role="status">Loading configured services…</p> : <><label>Service combination<Select required value={ruleId} onChange={e => setRuleId(e.target.value)}><option value="">Choose service and available add-on combination</option>{options?.rules.map(r => <option key={r.id} value={r.id}>{r.serviceName}{r.addons.length ? ` + ${r.addons.map(a => a.name).join(', ')}` : ' · add-ons optional'} · {r.durationMinutes} min · {options.resources.find(v => v.id === r.resourceId)?.name}</option>)}</Select></label>{!options?.rules.length && <p className={styles.help}>No bookable service combinations are configured at this worksite. Your administrator must configure resources, opening hours and service durations.</p>}{rule && <p className={styles.help}>{rule.serviceName} · {rule.addons.map(a => a.name).join(', ') || 'No add-ons included'} · {rule.durationMinutes} minutes. Only explicitly configured combinations are bookable.</p>}{rule && !rule.addonIds.length && !!rule.availableAddons?.length && <AddonPicker serviceName={rule.serviceName} addons={rule.availableAddons} picked={picked} onChange={setPicked} />}</>}</fieldset>
    <fieldset disabled={busy}><legend>Appointment time</legend><div className={styles.fieldPair}><label>Date<DatePicker required value={date} onChange={e => setDate(e.target.value)} /></label><label>Time<TimePicker required value={time} onChange={e => setTime(e.target.value)} /></label></div><p className={styles.help}>{site.timezone} · opening hours, buffers and remaining capacity are checked when saving. Repeated or nonexistent daylight-saving times are rejected.</p></fieldset>
    <p className={styles.help}>This creates a pending booking and reserves capacity. It does not collect payment or issue an invoice.</p><button className={styles.primary} disabled={busy || loading || !rule}>{busy ? 'Saving booking…' : 'Save walk-in & reserve time'}<ArrowUpRight size={18} /></button>
  </form></section>;
}
