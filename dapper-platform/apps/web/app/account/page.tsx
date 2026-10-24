'use client';

import {useEffect, useState} from 'react';
import ThemeToggle from '../ThemeToggle';
import CustomerAvailabilityBrowser from './CustomerAvailabilityBrowser';
import CustomerBookingRequestList from './CustomerBookingRequestList';
import CustomerQuoteList from './CustomerQuoteList';
import InvoicePaymentForm from './InvoicePaymentForm';

type Account = {customerId: string; name: string; locationName: string};
type Records = {
  account: Account;
  vehicles: {id: string; description: string; categoryDescription: string; identifier: string}[];
  bookings: {id: string; serviceName: string; status: string; bookingTime: string}[];
  invoices: {id: string; number: string; status: string; currency: string; totalCents: number; dueCents: number}[];
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
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('');
  const [identifier, setIdentifier] = useState('');
  const [notice, setNotice] = useState('');
  const [requestRevision,setRequestRevision]=useState(0);

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

  async function addEquipment(event: React.FormEvent) {
    event.preventDefault();
    if (!selected || busy) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const response = await fetch(`/api/v1/portal/accounts/${encodeURIComponent(selected)}/vehicles`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({description, categoryDescription: category, identifier})});
      if (response.status === 401) {setSignedIn(false); setRecords(null); setAccounts([]); setSelected(''); throw new Error('Your session expired. Please sign in again.');}
      if (!response.ok) throw new Error(response.status === 403 ? 'Customer access is no longer available. Your input is preserved.' : 'Could not add equipment. Check the fields and retry; your input is preserved.');
      setDescription(''); setCategory(''); setIdentifier(''); setNotice('Equipment added to your customer account.'); setRevision((value) => value + 1);
    } catch (err) {setError(err instanceof Error ? err.message : 'Equipment could not be saved.');}
    finally {setBusy(false);}
  }

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

  return <main className="secure-workspace" style={{maxWidth: 1000, margin: '0 auto', padding: '1.5rem'}}>
    <header style={{display: 'flex', flexWrap: 'wrap', gap: '1rem', alignItems: 'center', justifyContent: 'space-between', height: 'auto', padding: 0, background: 'transparent', border: 0}}>
      <a className="brand" href="/" style={{textDecoration: 'none', padding: 0}}><span className="brandmark">D<span>↗</span></span><span>DAPPER<small>DETAILING PROS</small></span></a>
      <ThemeToggle />
    </header>
    <h1>Your customer account</h1>
    <p>Your vehicles, appointments and invoices in one place.</p>
    {error && <p className="field-error" role="alert">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {checking ? <p role="status">Checking account service…</p> : !ready ? <section className="panel detail-card"><h2>Account service unavailable</h2><p>Secure sign-in is unavailable at this address. Please use the configured Dapper customer account address.</p></section> : !signedIn ? <form className="panel detail-card secure-login" onSubmit={signIn}>
      <h2>Sign in</h2>
      <label className="form-field">Email<input type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} /></label>
      <label className="form-field">Password<input type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} disabled={busy} /></label>
      <button className="primary" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
      <p>Use your Dapper customer account to access your records.</p>
    </form> : <>
      <button type="button" className="secondary" onClick={signOut} disabled={busy}>Sign out</button>
      {accounts.length === 0 ? <section className="panel detail-card"><h2>No customer records available</h2><p>Contact Dapper to connect your customer records to this account.</p></section> : <>
        <label className="form-field">Customer account<select disabled={busy} value={selected} onChange={(e) => {setRecords(null); setNotice(''); setSelected(e.target.value);}}>{accounts.map((a) => <option key={a.customerId} value={a.customerId}>{a.name} · {a.locationName}</option>)}</select></label>
        {!records && !error && <p role="status">Loading your records…</p>}
        {records && <>
          <h2>{records.account.name} · {records.account.locationName}</h2>
          <CustomerAvailabilityBrowser key={selected+'-availability'} customerId={selected} vehicles={records.vehicles} onSubmitted={()=>setRequestRevision(current=>current+1)}/>
          <CustomerBookingRequestList key={selected+'-requests'} endpoint={`/api/v1/portal/accounts/${encodeURIComponent(selected)}/booking-requests`} revision={requestRevision}/>
          <CustomerQuoteList key={selected+'-quotes'} customerId={selected} revision={requestRevision}/>
          <section className="panel detail-card"><h3>Your vehicles and equipment</h3>{records.vehicles.length ? records.vehicles.map((v) => <p key={v.id}>{v.description} · {v.categoryDescription || 'Category not provided'}{v.identifier ? ` · ${v.identifier}` : ''}</p>) : <p>No equipment recorded.</p>}</section>
          <form className="panel detail-card" onSubmit={addEquipment}>
            <h3>Add your vehicle or equipment</h3>
            <label className="form-field">Description<input required minLength={2} maxLength={120} value={description} onChange={(e) => setDescription(e.target.value)} disabled={busy} /></label>
            <label className="form-field">Category (optional)<input maxLength={60} value={category} onChange={(e) => setCategory(e.target.value)} disabled={busy} /></label>
            <label className="form-field">Identifier (optional)<input maxLength={60} value={identifier} onChange={(e) => setIdentifier(e.target.value)} disabled={busy} /></label>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Add equipment'}</button>
          </form>
          <section className="panel detail-card"><h3>Your bookings</h3>{records.bookings.length ? records.bookings.map((b) => <article key={b.id}><h4>{b.serviceName}</h4><p>{b.status.replaceAll('_', ' ')} · {new Intl.DateTimeFormat('en-CA', {dateStyle: 'medium', timeStyle: 'short', timeZone: 'America/Toronto'}).format(new Date(b.bookingTime))} (Toronto time)</p></article>) : <p>No bookings recorded.</p>}</section>
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
                  {i.dueCents > 0 && (
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
          <p>Up to {records.limit} recent records per section. Online payments are recorded directly to your account ledger.</p>
        </>}
      </>}
    </>}
  </main>;
}
