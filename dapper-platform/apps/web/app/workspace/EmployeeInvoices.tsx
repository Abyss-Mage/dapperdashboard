'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { FileText, Plus, Trash2 } from 'lucide-react';
import styles from './employee.module.css';

type Booking = { id: string; customerName: string; serviceName: string; bookingTime: string; status: string; addons: { name: string }[] };
type Invoice = { id: string; invoiceNumber: string; status: string; totalCents: number; dueCents: number; subtotalCents: number; taxCents: number; customerName: string; bookingId?: string; createdAt: string };
type Rate = { id: string; name: string; rateBasisPoints: number; active: boolean };
type Prefill = { bookingId: string; customerName: string; lines: { description: string }[]; taxes: { rates: Rate[]; settings: { pricesIncludeTax: boolean; rounding: string | null } }; existing: { id: string; invoiceNumber: string; status: string; totalCents: number }[] };
type Line = { description: string; quantity: string; price: string };

const money = (cents: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(cents / 100);
const FILTERS = [['all', 'All'], ['draft', 'Drafts'], ['open', 'Unpaid'], ['paid', 'Paid']] as const;
type Filter = (typeof FILTERS)[number][0];

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The request could not be completed.'));
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
const cents = (price: string) => Math.round(Number(price.replace(/[^0-9.]/g, '') || '0') * 100);

// Same rule as the server (the server's figure is the one saved): each active tax rounded to the cent, summed.
function taxFor(subtotal: number, rates: Rate[], rounding: string | null) {
  return rates.filter((r) => r.active).reduce((sum, r) => {
    const num = subtotal * r.rateBasisPoints;
    let q = Math.floor(num / 10000);
    const rem = num % 10000;
    if (rounding === 'down') q += 0;
    else if (rounding === 'half_even') q += rem > 5000 || (rem === 5000 && q % 2 === 1) ? 1 : 0;
    else q += rem >= 5000 ? 1 : 0;
    return sum + q;
  }, 0);
}

export default function EmployeeInvoices({ bookings, canCreate, canIssue, timezone, onPay }: { bookings: Booking[]; canCreate: boolean; canIssue: boolean; timezone: string; onPay: (bookingId: string) => void }) {
  const [invoices, setInvoices] = useState<Invoice[] | null>(null);
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');
  const [creating, setCreating] = useState(false);
  const [bookingId, setBookingId] = useState('');
  const [prefill, setPrefill] = useState<Prefill | null>(null);
  const [lines, setLines] = useState<Line[]>([]);
  const [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    try {
      setInvoices((await readJson(await fetch('/api/v1/employee/invoices', { cache: 'no-store' }))).items);
    } catch (e) {
      setInvoices(null);
      setError(e instanceof Error ? e.message : 'Invoices could not be loaded.');
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);

  const sortedBookings = useMemo(() => [...bookings].filter((b) => b.status !== 'cancelled').sort((a, b) => b.bookingTime.localeCompare(a.bookingTime)), [bookings]);
  const bookingLabel = (b: Booking) => `${b.customerName} · ${b.serviceName} · ${new Date(b.bookingTime).toLocaleDateString('en-CA', { month: 'short', day: 'numeric', timeZone: timezone })}`;

  async function choose(id: string) {
    setBookingId(id);
    setPrefill(null);
    setError('');
    if (!id) return;
    try {
      const p: Prefill = await readJson(await fetch(`/api/v1/employee/bookings/${id}/invoice-draft`, { cache: 'no-store' }));
      setPrefill(p);
      setLines(p.lines.map((l) => ({ description: l.description, quantity: '1', price: '' })));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'This booking could not be prepared.');
    }
  }

  const subtotal = lines.reduce((s, l) => s + Math.max(0, Number(l.quantity) || 0) * cents(l.price), 0);
  const tax = prefill ? taxFor(subtotal, prefill.taxes.rates, prefill.taxes.settings.rounding) : 0;
  const setLine = (i: number, patch: Partial<Line>) => setLines((cur) => cur.map((l, x) => (x === i ? { ...l, ...patch } : l)));

  async function save(issue: boolean) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const items = lines.filter((l) => l.description.trim()).map((l) => ({ description: l.description.trim(), quantity: Math.floor(Number(l.quantity) || 0), unitPriceCents: cents(l.price) }));
      const made = await readJson(await fetch('/api/v1/employee/invoices', send('POST', { bookingId, notes: notes.trim(), items })));
      let inv: Invoice = made.invoice;
      if (issue) inv = await readJson(await fetch(`/api/v1/employee/invoices/${inv.id}/issue`, send('POST')));
      setNotice(issue ? `Invoice ${inv.invoiceNumber} issued.` : `Draft invoice saved.${made.roundingAssumed ? ' Rounding has not been chosen for this franchise, so nearest cent was used.' : ''}`);
      setCreating(false);
      setBookingId('');
      setPrefill(null);
      setNotes('');
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The invoice could not be saved.');
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function issue(inv: Invoice) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const done = await readJson(await fetch(`/api/v1/employee/invoices/${inv.id}/issue`, send('POST')));
      setNotice(`Invoice ${done.invoiceNumber} issued.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The invoice could not be issued.');
    } finally {
      setBusy(false);
    }
  }

  const visible = (invoices ?? []).filter((i) => {
    const q = query.trim().toLowerCase();
    const f = filter === 'all' || (filter === 'draft' && i.status === 'draft') || (filter === 'open' && (i.status === 'issued' || i.status === 'partially_paid')) || (filter === 'paid' && i.status === 'paid');
    return f && (!q || i.invoiceNumber.toLowerCase().includes(q) || i.customerName.toLowerCase().includes(q));
  });
  const canSave = lines.some((l) => l.description.trim() && cents(l.price) > 0 && Number(l.quantity) >= 1);

  return (
    <section className={styles.invPage}>
      <div className={styles.invTop}>
        <div className={styles.invFilters} role="group" aria-label="Filter invoices">
          {FILTERS.map(([id, label]) => <button key={id} type="button" aria-pressed={filter === id} onClick={() => setFilter(id)}>{label}</button>)}
        </div>
        <input type="search" className={styles.invSearch} placeholder="Search number or customer" aria-label="Search invoices" value={query} onChange={(e) => setQuery(e.target.value)} />
        {canCreate && !creating && <button type="button" className={styles.invNew} onClick={() => { setCreating(true); setNotice(''); setError(''); }}><Plus size={16} aria-hidden="true" /> New invoice</button>}
      </div>
      {error && <p className={styles.error} role="alert">{error}</p>}
      {notice && <p className={styles.notice} role="status">{notice}</p>}

      {creating && (
        <form className={styles.invForm} onSubmit={(e) => { e.preventDefault(); void save(false); }}>
          <h2>New invoice</h2>
          <label>Booking
            <select value={bookingId} onChange={(e) => void choose(e.target.value)} disabled={busy}>
              <option value="">Choose the booking to bill</option>
              {sortedBookings.map((b) => <option key={b.id} value={b.id}>{bookingLabel(b)}</option>)}
            </select>
          </label>
          {prefill && prefill.existing.length > 0 && <p className={styles.help}>This booking already has: {prefill.existing.map((x) => `${x.invoiceNumber} (${x.status})`).join(', ')}.</p>}
          {prefill && (
            <>
              <div className={styles.invLines}>
                <div className={styles.invLineHead} aria-hidden="true"><span>Item</span><span>Qty</span><span>Price</span><span /></div>
                {lines.map((l, i) => (
                  <div key={i} className={styles.invLine}>
                    <input aria-label="Item description" maxLength={255} value={l.description} onChange={(e) => setLine(i, { description: e.target.value })} disabled={busy} />
                    <input aria-label="Quantity" inputMode="numeric" value={l.quantity} onChange={(e) => setLine(i, { quantity: e.target.value })} disabled={busy} />
                    <input aria-label="Unit price in dollars" inputMode="decimal" placeholder="0.00" value={l.price} onChange={(e) => setLine(i, { price: e.target.value })} disabled={busy} />
                    <button type="button" className={styles.invRemove} aria-label="Remove line" onClick={() => setLines((cur) => cur.filter((_, x) => x !== i))} disabled={busy || lines.length === 1}><Trash2 size={16} aria-hidden="true" /></button>
                  </div>
                ))}
                <button type="button" className={styles.invAdd} onClick={() => setLines((cur) => [...cur, { description: '', quantity: '1', price: '' }])} disabled={busy}><Plus size={14} aria-hidden="true" /> Add a line</button>
              </div>
              <label>Notes (optional)<input maxLength={2000} value={notes} onChange={(e) => setNotes(e.target.value)} disabled={busy} /></label>
              <dl className={styles.invTotals}>
                <div><dt>Subtotal</dt><dd>{money(subtotal)}</dd></div>
                {prefill.taxes.rates.filter((r) => r.active).map((r) => <div key={r.id}><dt>{r.name} {(r.rateBasisPoints / 100).toFixed(2)}%</dt><dd>{money(taxFor(subtotal, [r], prefill.taxes.settings.rounding))}</dd></div>)}
                {prefill.taxes.rates.filter((r) => r.active).length === 0 && <div><dt>Tax</dt><dd>None set for this franchise</dd></div>}
                <div className={styles.invGrand}><dt>Total</dt><dd>{money(subtotal + tax)}</dd></div>
              </dl>
              <p className={styles.help}>Prices are entered without tax. Taxes come from this franchise&rsquo;s settings and are added when the invoice is saved.</p>
            </>
          )}
          <div className={styles.invActions}>
            <button type="submit" className={styles.invSecondary} disabled={busy || !prefill || !canSave}>{busy ? 'Saving…' : 'Save as draft'}</button>
            {canIssue && <button type="button" className={styles.invPrimary} disabled={busy || !prefill || !canSave} onClick={() => void save(true)}>Save and issue</button>}
            <button type="button" className={styles.invGhost} disabled={busy} onClick={() => { setCreating(false); setPrefill(null); setBookingId(''); }}>Cancel</button>
          </div>
        </form>
      )}

      {!invoices && !error && <p role="status" className={styles.help}>Loading invoices…</p>}
      {invoices && visible.length === 0 && <div className={styles.empty}><FileText size={28} aria-hidden="true" /><h3>{invoices.length ? 'No invoices match.' : 'No invoices yet.'}</h3><p>{canCreate ? 'Choose “New invoice” to bill a booking.' : 'Invoices appear here once they are created.'}</p></div>}
      <ul className={styles.invList}>
        {visible.map((i) => (
          <li key={i.id} className={styles.invRow}>
            <div className={styles.invMain}>
              <strong>{i.invoiceNumber.startsWith('DRAFT-') ? 'Draft invoice' : i.invoiceNumber}</strong>
              <span>{i.customerName} · {new Date(i.createdAt).toLocaleDateString('en-CA', { month: 'short', day: 'numeric', timeZone: timezone })}</span>
            </div>
            <span className={`${styles.invChip} ${styles['invChip_' + i.status]}`}>{i.status.replace('_', ' ')}</span>
            <div className={styles.invAmount}><strong>{money(i.totalCents)}</strong>{i.dueCents > 0 && i.status !== 'draft' && <span>{money(i.dueCents)} due</span>}</div>
            <div className={styles.invRowActions}>
              {i.status === 'draft' && canIssue && <button type="button" className={styles.invSecondary} disabled={busy} onClick={() => void issue(i)}>Issue</button>}
              {(i.status === 'issued' || i.status === 'partially_paid') && i.bookingId && <button type="button" className={styles.invPrimary} onClick={() => onPay(i.bookingId as string)}>Take payment</button>}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
