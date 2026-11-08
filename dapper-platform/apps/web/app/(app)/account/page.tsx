'use client';

import {useEffect, useState} from 'react';
import InvoicePaymentForm from './InvoicePaymentForm';
import Select from '../GlassSelect';
import { SignInDetails } from '../workspace/AccountSecurity';
import AuthShell from '../AuthShell';
import AccountProfile, {Avatar, type SavedAddress} from './AccountProfile';
import BookingHistory from './BookingHistory';
import { SiteHeader } from '../../_chrome/SiteHeader';
import { SiteFooter } from '../../_chrome/SiteFooter';
import ForceDark from '../../_chrome/ForceDark';
import a from './account.module.css';
import { fillInvoiceWindow, loadPrintTemplate, openInvoiceWindow } from '../workspace/InvoicePrint';

type Account = {customerId: string; name: string; locationName: string; email?: string; phone?: string; address?: string};
type Records = {
  account: Account;
  vehicles: {id: string; description: string; categoryDescription: string; identifier: string}[];
  bookings: {id: string; serviceName: string; status: string; bookingTime: string}[];
  invoices: {id: string; number: string; status: string; currency: string; totalCents: number; dueCents: number; bookingId: string}[];
  addresses: SavedAddress[];
  hasAvatar: boolean;
  limit: number;
};

function amount(cents: number, currency: string) {
  if (!Number.isSafeInteger(cents)) return 'Amount unavailable';
  try { return new Intl.NumberFormat('en-CA', {style: 'currency', currency}).format(cents / 100); }
  catch { return 'Amount unavailable'; }
}

export default function CustomerAccount() {
  const [ready, setReady] = useState(false);
  const [checking, setChecking] = useState(true);
  const [signedIn, setSignedIn] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selected, setSelected] = useState('');
  const [records, setRecords] = useState<Records | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);
  const [detailName, setDetailName] = useState(''), [detailPhone, setDetailPhone] = useState(''), [detailAddress, setDetailAddress] = useState('');
  useEffect(() => { if (records) { setDetailName(records.account.name); setDetailPhone(records.account.phone ?? ''); setDetailAddress(records.account.address ?? ''); } }, [records]);
  async function saveDetails(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const response = await fetch(`/api/v1/portal/accounts/${encodeURIComponent(selected)}/details`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({name: detailName, phone: detailPhone, address: detailAddress})});
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.error ?? 'Your details could not be saved.');
      setNotice('Your details were saved.'); setRevision((value) => value + 1);
    } catch (err) { setError(err instanceof Error ? err.message : 'Your details could not be saved.'); }
    finally { setBusy(false); }
  }
  function openInvoice(invoiceId: string) {
    setError('');
    const w = openInvoiceWindow();
    if (!w) { setError('Allow pop-ups for this site to open the invoice in a new window.'); return; }
    fetch(`/api/v1/portal/accounts/${encodeURIComponent(selected)}/invoices/${encodeURIComponent(invoiceId)}`, {cache: 'no-store'})
      .then(async (response) => { const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(body.error ?? 'The invoice could not be opened.'); return body; })
      .then(async (body) => fillInvoiceWindow(w, body, await loadPrintTemplate(`/api/v1/portal/accounts/${encodeURIComponent(selected)}/invoice-template`)))
      .catch((err) => { w.close(); setError(err instanceof Error ? err.message : 'The invoice could not be opened.'); });
  }
  const [notice, setNotice] = useState('');

  async function loadAccounts() {
    const response = await fetch('/api/v1/portal/accounts', {cache: 'no-store'});
    if (response.status === 401) {setSignedIn(false); setAccounts([]); setSelected(''); setRecords(null); return;}
    if (!response.ok) throw new Error('Your customer accounts could not be loaded.');
    const body = await response.json();
    setSignedIn(true);
    setAccounts(body.items);
    setSelected((current) => body.items.some((a: Account) => a.customerId === current) ? current : body.items[0]?.customerId ?? '');
  }

  useEffect(() => {
    async function initialize() {
      try {
        const response = await fetch('/api/v1/health', {cache: 'no-store'});
        if (!response.ok) throw new Error();
        const health = await response.json();
        const available = window.location.protocol === 'https:' && health.mode === 'scoped-postgres-foundation' && health.identityProviderConfigured === true;
        setReady(available);
        if (available) await loadAccounts();
      } catch {setError('The customer account service is unavailable.');}
      finally {setChecking(false);}
    }
    initialize();
  }, []);

  useEffect(() => {
    let current = true;
    setRecords(null);
    if (!signedIn || !selected) return;
    setError('');
    async function loadRecords() {
      try {
        const response = await fetch(`/api/v1/portal/accounts/${encodeURIComponent(selected)}/records`, {cache: 'no-store'});
        if (!current) return;
        if (response.status === 401) {setSignedIn(false); setAccounts([]); setSelected(''); return;}
        if (!response.ok) throw new Error('This customer account is unavailable or access has been withdrawn.');
        const body = await response.json();
        if (current) setRecords(body);
      } catch (err) {if (current) setError(err instanceof Error ? err.message : 'Records could not be loaded.');}
    }
    loadRecords();
    return () => {current = false;};
  }, [selected, signedIn, revision]);


  async function signIn(event: React.FormEvent) {
    event.preventDefault();
    if (!ready) return;
    setBusy(true); setError('');
    try {
      const response = await fetch('/api/v1/auth/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({email, password})});
      setPassword('');
      if (!response.ok) throw new Error(response.status === 429 ? 'Too many attempts. Please try again later.' : 'Sign-in failed. Check your account details.');
      await loadAccounts();
    } catch (err) {setError(err instanceof Error ? err.message : 'Sign-in unavailable.');}
    finally {setBusy(false);}
  }

  async function signOut() {
    setBusy(true); setError('');
    try {
      const response = await fetch('/api/v1/auth/logout', {method: 'POST'});
      if (!response.ok && response.status !== 401) throw new Error();
      setRecords(null); setAccounts([]); setSelected(''); setSignedIn(false); setPassword('');
    } catch {setError('Could not sign out. Please retry.');}
    finally {setBusy(false);}
  }

  if (!checking && !signedIn) {
    return (
      <AuthShell kind="Customer account" title="Sign in to your account" lead="Your bookings and invoices in one place.">
        {error && <p className="field-error" role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        {!ready && <p>Secure sign-in is unavailable at this address. Please use the configured Dapper customer account address.</p>}
        <form onSubmit={signIn} style={{ display: 'grid', gap: 14 }}>
          <label className="form-field">Email<input type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy || !ready} /></label>
          <label className="form-field">Password<input type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} disabled={busy || !ready} /></label>
          <p style={{ margin: 0 }}><a href="/reset-password" style={{ textDecoration: 'underline' }}>Forgot your password?</a></p>
          <button className="primary" disabled={busy || !ready}>{busy ? 'Signing in…' : 'Sign in'}</button>
          <p style={{ margin: 0, opacity: .75 }}>Use your Dapper customer account to access your records. New here? <a href="/booking" style={{ textDecoration: 'underline' }}>Book a detail</a> and we will set you up.</p>
        </form>
      </AuthShell>
    );
  }

  const firstName = records ? records.account.name.trim().split(/\s+/)[0] : '';
  const upcoming = records ? records.bookings.filter((b) => new Date(b.bookingTime).getTime() > Date.now() && !/cancel|complete/i.test(b.status)).length : 0;
  const owing = records ? records.invoices.reduce((t, i) => t + (i.dueCents > 0 ? i.dueCents : 0), 0) : 0;
  const currency = records?.invoices[0]?.currency ?? 'CAD';
  return <div className={a.page}>
    <ForceDark theme="light" />
    <SiteHeader />
    {error && <p className="field-error" role="alert" style={{margin: '0 auto', maxWidth: 1100, padding: '1rem clamp(16px,3.4vw,56px)'}}>{error}</p>}
    {notice && <p role="status" style={{margin: '0 auto', maxWidth: 1100, padding: '0 clamp(16px,3.4vw,56px)'}}>{notice}</p>}
    {checking ? <p role="status" className={a.wait}>Checking your account…</p> : !ready ? <section className={a.wait + ' panel detail-card'}><h2>Account service unavailable</h2><p>Secure sign-in is unavailable at this address. Please use the configured Dapper customer account address.</p></section> : <>
      <section className={a.hello} aria-labelledby="acct-title">
        {records && <div className={a.avatar}><Avatar customerId={selected} name={records.account.name} has={records.hasAvatar} version={revision} size={72} /></div>}
        <p className={a.kicker}>Your account</p>
        <h1 id="acct-title">{records ? <>Hello, <em>{firstName}.</em></> : <>Your <em>account.</em></>}</h1>
        <p className={a.sub}>Your bookings and invoices in one place.</p>
        {records && <div className={a.stats}>
          <div><small>Upcoming bookings</small><b>{upcoming}</b></div>
          <div><small>Balance due</small><b>{amount(owing, currency)}</b></div>
          <div><small>Bookings</small><b>{records.bookings.length}</b></div>
        </div>}
        <div className={a.actions}>
          <a className={a.book} href="/booking">Book a detail</a>
          <button type="button" className={a.out} onClick={signOut} disabled={busy}>Sign out</button>
        </div>
      </section>
      {accounts.length === 0 ? <section className={a.body + ' panel detail-card'}><h2>No customer records available</h2><p>Contact Dapper to connect your customer records to this account.</p></section> : <div className={a.body}>
        <label className="form-field">Customer account<Select disabled={busy} value={selected} onChange={(e) => {setRecords(null); setNotice(''); setSelected(e.target.value);}}>{accounts.map((a) => <option key={a.customerId} value={a.customerId}>{a.name} · {a.locationName}</option>)}</Select></label>
        {!records && !error && <p role="status">Loading your records…</p>}
        {records && <>
          <nav className={a.tabs} aria-label="Account sections">
            <a href="#profile">Profile</a><a href="#bookings">Bookings</a><a href="#invoices">Invoices</a><a href="#payment">Payment</a>
          </nav>
          <div id="profile" className={a.sec}>
            <AccountProfile customerId={selected} name={records.account.name} email={records.account.email} hasAvatar={records.hasAvatar} addresses={records.addresses} onChanged={() => setRevision((v) => v + 1)} />
          <form className="panel detail-card" onSubmit={saveDetails}>
            <h3>Your details</h3>
            {records.account.email && <p>You sign in with <strong>{records.account.email}</strong>.</p>}
            <label className="form-field">Name<input required minLength={2} maxLength={100} autoComplete="name" value={detailName} onChange={(e) => setDetailName(e.target.value)} disabled={busy} /></label>
            <label className="form-field">Phone<input type="tel" maxLength={30} autoComplete="tel" value={detailPhone} onChange={(e) => setDetailPhone(e.target.value)} disabled={busy} /></label>
            <label className="form-field">Address (for mobile detailing)<input maxLength={300} autoComplete="street-address" value={detailAddress} onChange={(e) => setDetailAddress(e.target.value)} disabled={busy} /></label>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save my details'}</button>
          </form>
            <section className="panel detail-card"><h3>Email and password</h3><SignInDetails email={records.account.email} /></section>
          </div>
          <div id="bookings" className={a.sec}>
            <BookingHistory bookings={records.bookings} invoices={records.invoices} onInvoice={openInvoice} />
          </div>
          <div id="invoices" className={a.sec}>
          <section className="panel detail-card">
            <h3>Your invoices</h3>
            {records.invoices.length ? (
              records.invoices.map((i) => (
                <article key={i.id} style={{ borderBottom: '1px solid var(--border, #f0dcde)', padding: '0.75rem 0' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap' }}>
                    <h4 style={{ margin: 0 }}>Invoice #{i.number}</h4>
                    <span style={{ fontWeight: 600, fontSize: '0.85rem' }}>{i.status.replaceAll('_', ' ').toUpperCase()}</span>
                  </div>
                  <p style={{ margin: '0.3rem 0' }}>
                    Total: {amount(i.totalCents, i.currency)} · Balance Due: {amount(i.dueCents, i.currency)}
                  </p>
                  <button type="button" className="secondary" onClick={() => openInvoice(i.id)}>View or download invoice</button>
                  {i.dueCents > 0 && !i.id.startsWith('legacy-') && (
                    <InvoicePaymentForm
                      customerId={selected}
                      invoiceId={i.id}
                      invoiceNumber={i.number}
                      dueCents={i.dueCents}
                      currency={i.currency}
                      onPaymentSuccess={() => setRevision((v) => v + 1)}
                    />
                  )}
                </article>
              ))
            ) : (
              <p>No issued invoices recorded.</p>
            )}
          </section>
          </div>
          <div id="payment" className={a.sec}>
            <section className="panel detail-card">
              <h3>Payment methods</h3>
              <p>Saved cards are not available yet. You pay your deposit by card when you book (a secure Square card form), and any balance from the Invoices section above.</p>
            </section>
          </div>
          <p>Up to {records.limit} recent records per section. Online payments are recorded directly to your account ledger.</p>
        </>}
      </div>}
    </>}
    <SiteFooter />
  </div>;
}
