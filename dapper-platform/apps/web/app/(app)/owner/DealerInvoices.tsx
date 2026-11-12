'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Download, FileText, Pencil, Plus, Trash2 } from 'lucide-react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import { DatePicker } from '../Pickers';
import { askConfirm } from '../confirm';
import { openPdf } from '../workspace/InvoicePrint';
import FormPopup from '../FormPopup';

/* Dealership (B2B) invoices: the franchise's invoices to its dealerships, filtered by dealership, date and status, with totals, a PDF per
   invoice and a ZIP of all of them. A draft has an invoice date and due date (calendar), an optional typed invoice number and PO number,
   item lines (item, description, qty, rate) and a discount as a percent or an amount, taken off before tax. Issuing gives it the next
   dealership number (unless one was typed) and emails the dealership a private link to view, download and pay it without signing in. */

type Dealer = { id: string; accountName: string; active: boolean; perCarRateCents: number | null };
type Summary = { id: string; invoiceNumber: string; dealershipId: string; dealershipName: string; status: string; overdue: boolean; invoiceDate?: string; dueDate?: string; poNumber: string; issuedAt?: string; dueAt?: string; createdAt: string; totalCents: number; dueCents: number; viewedAt?: string };
type FullLine = { description: string; detail: string; quantity: number; unitPriceCents: number; vin: string; stockNumber: string };
type Full = { id: string; invoiceNumber: string; manualNumber: string; status: string; overdue: boolean; dealershipId: string; dealershipName: string; invoiceDate?: string; dueDate?: string; poNumber: string; discountPercent: number; discountCents: number; notes: string; version: number; subtotalCents: number; taxCents: number; totalCents: number; paidCents: number; dueCents: number; issuedAt?: string; dueAt?: string; viewedAt?: string; lines: FullLine[] };
type Series = { prefix: string; nextNumber: number; version: number };
type Line = { description: string; detail: string; quantity: string; price: string; vin: string; stockNumber: string };
type Draft = { id?: string; version: number; dealershipId: string; invoiceNumber: string; poNumber: string; invoiceDate: string; dueDate: string; discountKind: 'percent' | 'amount'; discount: string; notes: string; lines: Line[]; saved?: Full };

const money = (c: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);
const isoDay = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const plusDays = (iso: string, n: number) => { const [y, m, d] = iso.split('-').map(Number); return isoDay(new Date(y, m - 1, d + n)); };
const showDay = (iso?: string) => { if (!iso) return '—'; const [y, m, d] = iso.slice(0, 10).split('-').map(Number); return new Date(y, m - 1, d).toLocaleDateString('en-CA', { dateStyle: 'medium' }); };
const cents = (s: string) => Math.round(Number(s.replace(/[^0-9.]/g, '')) * 100) || 0;
const STATUS: Record<string, string> = { draft: 'Draft', issued: 'Sent', partially_paid: 'Part paid', paid: 'Paid', void: 'Void' };
// Draft, Sent, Viewed (the dealership opened it in its portal), Part paid, Paid, Overdue, Void (error404 #44).
const statusOf = (i: { status: string; overdue: boolean; viewedAt?: string }) => i.overdue ? 'Overdue' : i.status === 'issued' && i.viewedAt ? 'Viewed' : STATUS[i.status] ?? i.status;

async function readJson<T>(r: Response): Promise<T> {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body as T;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

export default function DealerInvoices({ base, organizationId, canCreate, canIssue, canManage, canConfigure }: { base: string; organizationId: string; canCreate: boolean; canIssue: boolean; canManage: boolean; canConfigure: boolean }) {
  const url = `${base}/dealer-invoices`;
  const [dealers, setDealers] = useState<Dealer[]>([]);
  const [items, setItems] = useState<Summary[] | null>(null);
  const [filter, setFilter] = useState({ dealershipId: '', from: '', to: '', status: '' });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [open, setOpen] = useState<Full | null>(null);
  // Recording a payment received outside the portal (error404 #44); the key makes a double click record it once.
  // Card attempts still holding the open invoice's balance (the server could not confirm them with Square).
  const [held, setHeld] = useState<string[]>([]);
  useEffect(() => {
    setHeld([]);
    if (!open || open.dueCents <= 0) return;
    fetch(`${url}/${open.id}/held-payments`, { cache: 'no-store' }).then((r) => readJson<{ items: string[] }>(r)).then((b) => setHeld(b.items)).catch(() => undefined);
  }, [open?.id, open?.dueCents, url]); // eslint-disable-line react-hooks/exhaustive-deps
  const [paying, setPaying] = useState<{ amount: string; method: 'bank_transfer' | 'cash'; note: string; key: string } | null>(null);
  const [series, setSeries] = useState<Series | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const query = useMemo(() => new URLSearchParams(Object.entries(filter).filter(([, v]) => v)).toString(), [filter]);
  const load = useCallback(async () => setItems((await readJson<{ items: Summary[] }>(await fetch(`${url}?${query}`, { cache: 'no-store' }))).items), [url, query]);
  useEffect(() => { setItems(null); load().catch((e) => setError(e.message)); }, [load]);
  const loadSeries = useCallback(() => { fetch(`/api/v1/organizations/${organizationId}/dealer-invoice-series`, { cache: 'no-store' }).then((r) => readJson<Series>(r)).then(setSeries).catch(() => undefined); }, [organizationId]);
  useEffect(() => {
    fetch(`${base}/dealerships`, { cache: 'no-store' }).then((r) => readJson<{ items?: Dealer[] } | Dealer[]>(r)).then((b) => setDealers(Array.isArray(b) ? b : b.items ?? [])).catch(() => undefined);
    loadSeries();
  }, [base, loadSeries]);

  async function act(fn: () => Promise<string | void>) {
    setBusy(true); setError(''); setNotice('');
    try { const msg = await fn(); if (msg) setNotice(msg); await load(); } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); } finally { setBusy(false); }
  }
  const rateOf = (id: string) => dealers.find((d) => d.id === id)?.perCarRateCents ?? null;
  const blankLine = (dealer: string): Line => { const r = rateOf(dealer); return { description: '', detail: '', quantity: '1', price: r != null ? (r / 100).toFixed(2) : '', vin: '', stockNumber: '' }; };
  const startNew = () => {
    const d = filter.dealershipId || dealers.find((x) => x.active)?.id || '';
    const today = isoDay(new Date());
    setOpen(null);
    setDraft({ version: 0, dealershipId: d, invoiceNumber: '', poNumber: '', invoiceDate: today, dueDate: plusDays(today, 30), discountKind: 'percent', discount: '', notes: '', lines: [blankLine(d)] });
  };
  const fromFull = (f: Full): Draft => ({
    id: f.id, version: f.version, dealershipId: f.dealershipId, invoiceNumber: f.manualNumber, poNumber: f.poNumber, invoiceDate: f.invoiceDate ?? isoDay(new Date()), dueDate: f.dueDate ?? plusDays(f.invoiceDate ?? isoDay(new Date()), 30),
    discountKind: f.discountCents > 0 && !f.discountPercent ? 'amount' : 'percent', discount: f.discountPercent ? String(f.discountPercent) : f.discountCents > 0 ? (f.discountCents / 100).toFixed(2) : '', notes: f.notes,
    lines: f.lines.map((l) => ({ description: l.description, detail: l.detail, quantity: String(l.quantity), price: (l.unitPriceCents / 100).toFixed(2), vin: l.vin, stockNumber: l.stockNumber })), saved: f,
  });
  const view = (id: string) => act(async () => {
    const f = await readJson<Full>(await fetch(`${url}/${id}`, { cache: 'no-store' }));
    if (f.status === 'draft' && canCreate) { setOpen(null); setDraft(fromFull(f)); } else { setDraft(null); setOpen(f); }
  });
  const body = (d: Draft) => ({
    dealershipId: d.dealershipId, invoiceNumber: d.invoiceNumber.trim(), poNumber: d.poNumber.trim(), invoiceDate: d.invoiceDate, dueDate: d.dueDate, notes: d.notes, version: d.version,
    discountPercent: d.discountKind === 'percent' ? Number(d.discount) || 0 : 0, discountCents: d.discountKind === 'amount' ? cents(d.discount) : 0,
    lines: d.lines.map((l) => ({ description: l.description, detail: l.detail, quantity: Number(l.quantity) || 1, unitPriceCents: cents(l.price), vin: l.vin, stockNumber: l.stockNumber })),
  });
  const saveDraft = async (d: Draft) => readJson<Full>(await fetch(d.id ? `${url}/${d.id}` : url, send(d.id ? 'PUT' : 'POST', body(d))));
  const emailWords = (r: { emailStatus: string; emailedTo?: string; emailError?: string }) => r.emailError ? `not emailed: ${r.emailError}` : r.emailStatus === 'sent' ? `emailed to ${r.emailedTo}` : r.emailStatus === 'sending' ? `is being emailed to ${r.emailedTo} (it can take a minute; if it does not arrive, check spam, then use Resend email)` : r.emailStatus === 'no_email' ? 'not emailed: the dealership has no contact email' : r.emailStatus === 'not_configured' ? 'not emailed: email sending is not set up' : 'the email could not be sent; use Resend email';
  const issue = (d: Draft) => act(async () => {
    if (!(await askConfirm({ title: 'Issue and email this invoice?', message: `It gets ${d.invoiceNumber.trim() ? `number ${d.invoiceNumber.trim()}` : 'the next dealership number'}, can no longer be edited, and the dealership is emailed the PDF with a link to view and pay it online.`, confirmLabel: 'Issue and email' }))) return;
    const saved = await saveDraft(d);
    const r = await readJson<Full & { emailStatus: string; emailedTo?: string }>(await fetch(`${url}/${saved.id}/issue`, send('POST')));
    setDraft(null); setOpen(r); loadSeries();
    return `Invoice ${r.invoiceNumber} issued and ${emailWords(r)}.`;
  });
  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => draft && setDraft({ ...draft, [k]: v });
  const setInvoiceDate = (v: string) => draft && setDraft({ ...draft, invoiceDate: v, dueDate: !draft.dueDate || draft.dueDate < v ? plusDays(v, 30) : draft.dueDate });
  const setLine = (i: number, k: keyof Line, v: string) => draft && setDraft({ ...draft, lines: draft.lines.map((l, j) => (j === i ? { ...l, [k]: v } : l)) });

  const subtotal = draft ? draft.lines.reduce((t, l) => t + (Number(l.quantity) || 0) * cents(l.price), 0) : 0;
  const discount = draft ? Math.min(subtotal, draft.discountKind === 'percent' ? Math.round(subtotal * (Number(draft.discount) || 0) / 100) : cents(draft.discount)) : 0;
  // The tax and total as you type, worked out by the server with the franchise's own tax (error404 #24).
  const [preview, setPreview] = useState<{ subtotalCents: number; discountCents: number; taxCents: number; totalCents: number } | null>(null);
  const previewKey = draft ? JSON.stringify([draft.lines.map((l) => [l.quantity, l.price]), draft.discountKind, draft.discount]) : '';
  useEffect(() => {
    if (!draft) { setPreview(null); return; }
    const ctl = new AbortController();
    const t = setTimeout(() => {
      fetch(`${url}/preview`, { ...send('POST', { discountPercent: draft.discountKind === 'percent' ? Number(draft.discount) || 0 : 0, discountCents: draft.discountKind === 'amount' ? cents(draft.discount) : 0, lines: draft.lines.map((l) => ({ quantity: Number(l.quantity) || 0, unitPriceCents: cents(l.price) })) }), signal: ctl.signal })
        .then((r) => readJson<{ subtotalCents: number; discountCents: number; taxCents: number; totalCents: number }>(r)).then(setPreview).catch(() => undefined);
    }, 250);
    return () => { clearTimeout(t); ctl.abort(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [previewKey, url]);
  const live = preview && preview.subtotalCents === subtotal ? preview : null;
  const totals = (items ?? []).filter((i) => i.status !== 'void' && i.status !== 'draft').reduce((t, i) => ({ billed: t.billed + i.totalCents, owing: t.owing + i.dueCents }), { billed: 0, owing: 0 });
  const nextNumber = series ? `${series.prefix}${series.nextNumber}` : 'the next number';

  return (
    <div className={`${styles.stack} ${styles.dlPage}`}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.dlTitle}>Dealership invoices</h2>
          <p className={styles.dlLead}>Bill your dealerships, email them the invoice with a link to pay online, and track what is still owed.</p>
        </div>
        {canCreate && !draft && <button type="button" className={`primary ${styles.dlBig}`} disabled={busy || !dealers.some((d) => d.active)} onClick={startNew}><Plus size={18} aria-hidden="true" /> New invoice</button>}
      </div>
      {error && <p role="alert" className={`field-error ${styles.dlMsg}`}>{error}</p>}
      {notice && <p role="status" className={styles.dlMsg}>{notice}</p>}
      {!dealers.length && <p className={styles.dlLead}>Add a dealership under Dealerships first.</p>}

      {draft && (
        <FormPopup label={draft.id ? 'Edit draft invoice' : 'New dealership invoice'} width={1180} onClose={() => { if (!busy) setDraft(null); }}>
        <form className={styles.dlPage} onSubmit={(e) => { e.preventDefault(); void act(async () => { const s = await saveDraft(draft); setDraft(fromFull(s)); return 'Draft saved.'; }); }}>
          <h3 className={styles.dlH3}>{draft.id ? 'Edit draft invoice' : 'New dealership invoice'}</h3>
          {error && <p role="alert" className={`field-error ${styles.dlMsg}`}>{error}</p>}
          <div className={styles.dlGrid}>
            <label className={styles.dlField}>Dealership<Select value={draft.dealershipId} onChange={(e) => set('dealershipId', e.target.value)} disabled={busy || !!draft.id} required><option value="">Choose a dealership</option>{dealers.filter((d) => d.active || d.id === draft.dealershipId).map((d) => <option key={d.id} value={d.id}>{d.accountName}</option>)}</Select></label>
            <label className={styles.dlField}>Invoice number<input value={draft.invoiceNumber} onChange={(e) => set('invoiceNumber', e.target.value)} maxLength={40} placeholder={`Automatic (${nextNumber})`} disabled={busy} /><small>Leave empty to use {nextNumber}.</small></label>
            <label className={styles.dlField}>PO number<input value={draft.poNumber} onChange={(e) => set('poNumber', e.target.value)} maxLength={40} placeholder="Optional" disabled={busy} /></label>
            <div className={styles.dlField}><span id="dl-inv-date">Invoice date</span><DatePicker value={draft.invoiceDate} onChange={(e) => setInvoiceDate(e.target.value)} aria-labelledby="dl-inv-date" required disabled={busy} /></div>
            <div className={styles.dlField}><span id="dl-due-date">Due date</span><DatePicker value={draft.dueDate} min={draft.invoiceDate} max={plusDays(draft.invoiceDate, 365)} onChange={(e) => set('dueDate', e.target.value)} aria-labelledby="dl-due-date" required disabled={busy} /></div>
          </div>

          <div className={styles.dlLines}>
            <div className={styles.dlHead} aria-hidden="true"><span>Item</span><span>Description</span><span>Qty</span><span>Rate</span><span>Amount</span><span /></div>
            {draft.lines.map((l, i) => (
              <div key={i} className={styles.dlRow}>
                <input aria-label={`Item ${i + 1}`} value={l.description} onChange={(e) => setLine(i, 'description', e.target.value)} placeholder="e.g. Full detail" maxLength={255} minLength={2} required disabled={busy} />
                <input aria-label={`Item ${i + 1} description`} value={l.detail} onChange={(e) => setLine(i, 'detail', e.target.value)} placeholder="Optional details" maxLength={500} disabled={busy} />
                <input aria-label={`Item ${i + 1} quantity`} type="number" min={1} value={l.quantity} onChange={(e) => setLine(i, 'quantity', e.target.value)} required disabled={busy} />
                <input aria-label={`Item ${i + 1} rate in dollars`} inputMode="decimal" value={l.price} onChange={(e) => setLine(i, 'price', e.target.value)} placeholder="0.00" required disabled={busy} />
                <span className={styles.dlAmount}>{money((Number(l.quantity) || 0) * cents(l.price))}</span>
                <button type="button" className="secondary" aria-label={`Remove item ${i + 1}`} disabled={busy || draft.lines.length === 1} onClick={() => setDraft({ ...draft, lines: draft.lines.filter((_, j) => j !== i) })}><Trash2 size={17} aria-hidden="true" /></button>
              </div>
            ))}
            <div><button type="button" className={`secondary ${styles.dlBig}`} disabled={busy || draft.lines.length >= 100} onClick={() => setDraft({ ...draft, lines: [...draft.lines, blankLine(draft.dealershipId)] })}><Plus size={17} aria-hidden="true" /> Add item</button></div>
          </div>

          <div className={styles.dlBottom}>
            <label className={styles.dlField}>Notes on the invoice (optional)<textarea rows={4} maxLength={2000} value={draft.notes} onChange={(e) => set('notes', e.target.value)} disabled={busy} /></label>
            <div className={styles.dlSum}>
              <div><span>Subtotal</span><strong>{money(subtotal)}</strong></div>
              <div className={styles.dlDiscount}>
                <span>Discount</span>
                <div role="group" aria-label="Discount type" className={styles.dlToggle}>
                  <button type="button" aria-pressed={draft.discountKind === 'percent'} onClick={() => set('discountKind', 'percent')} disabled={busy}>%</button>
                  <button type="button" aria-pressed={draft.discountKind === 'amount'} onClick={() => set('discountKind', 'amount')} disabled={busy}>$</button>
                </div>
                <input aria-label={draft.discountKind === 'percent' ? 'Discount percent' : 'Discount amount in dollars'} inputMode="decimal" value={draft.discount} onChange={(e) => set('discount', e.target.value.replace(/[^0-9.]/g, ''))} placeholder="0" disabled={busy} />
              </div>
              {discount > 0 && <div><span>Discount</span><strong>- {money(discount)}</strong></div>}
              <div><span>Tax</span><strong>{live ? money(live.taxCents) : '…'}</strong></div>
              <div className={styles.dlTotal}><span>Total</span><strong>{live ? money(live.totalCents) : money(subtotal - discount) + ' + tax'}</strong></div>
            </div>
          </div>

          <div className={styles.rowBetween}>
            <div className={styles.dlActions}>
              <button className="secondary" disabled={busy || !draft.dealershipId}>{busy ? 'Saving…' : 'Save draft'}</button>
              {draft.id && <button type="button" className="secondary" disabled={busy} onClick={() => openPdf(`${url}/${draft.id}/pdf`)}><FileText size={17} aria-hidden="true" /> Download PDF</button>}
              {canIssue && <button type="button" className="primary" disabled={busy || !draft.dealershipId} onClick={() => void issue(draft)}>Issue and email</button>}
            </div>
            <button type="button" className="secondary" disabled={busy} onClick={() => setDraft(null)}>Close</button>
          </div>
        </form>
        </FormPopup>
      )}

      {open && (
        <FormPopup label={`Invoice ${open.invoiceNumber}`} width={980} onClose={() => { if (!busy) { setOpen(null); setPaying(null); } }}>
        <section className={styles.dlPage}>
          <div className={styles.rowBetween}>
            <h3 className={styles.dlH3}>Invoice {open.invoiceNumber} · {open.dealershipName}</h3>
            <span className={open.overdue ? styles.dlOverdue : styles.dlBadge}>{statusOf(open)}</span>
          </div>
          <dl className={styles.dlFacts}>
            <div><dt>Invoice date</dt><dd>{showDay(open.invoiceDate ?? open.issuedAt)}</dd></div>
            <div><dt>Due date</dt><dd>{showDay(open.dueDate ?? open.dueAt)}</dd></div>
            {open.poNumber && <div><dt>PO number</dt><dd>{open.poNumber}</dd></div>}
            <div><dt>Total</dt><dd>{money(open.totalCents)}</dd></div>
            <div><dt>Balance</dt><dd>{money(open.dueCents)}</dd></div>
          </dl>
          <div style={{ overflowX: 'auto' }}>
            <table className={`${styles.table} ${styles.dlTable}`}>
              <thead><tr><th scope="col">Item</th><th scope="col">Description</th><th scope="col">Qty</th><th scope="col">Rate</th><th scope="col">Amount</th></tr></thead>
              <tbody>{open.lines.map((l, i) => (
                <tr key={i}><td><strong>{l.description}</strong></td><td style={{ whiteSpace: 'normal' }}>{[l.detail, l.vin && `VIN ${l.vin}`, l.stockNumber && `Stock ${l.stockNumber}`].filter(Boolean).join(' · ') || '—'}</td><td>{l.quantity}</td><td>{money(l.unitPriceCents)}</td><td>{money(l.unitPriceCents * l.quantity)}</td></tr>
              ))}</tbody>
            </table>
          </div>
          {held.length > 0 && (
            <div className={`${styles.dlSum} ${styles.dlMsg}`} role="status" style={{ marginBottom: 14 }}>
              <strong>A card payment for this invoice was never confirmed by Square</strong>
              <span>{held.join('; ')}. It holds the balance, so the dealership cannot pay until it is sorted out.</span>
              {canManage && <div><button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => {
                if (!(await askConfirm({ title: 'Release the unconfirmed card payment?', message: 'Only do this after checking in Square (including a Square account connected before) that no payment for this invoice was taken at that time. If it was, record it with Mark as paid instead.', confirmLabel: 'Release it', tone: 'danger' }))) return;
                const r = await readJson<{ released: number }>(await fetch(`${url}/${open.id}/release-held-payments`, send('POST')));
                setHeld([]);
                return `${r.released} unconfirmed card payment${r.released === 1 ? '' : 's'} released; the dealership can pay again.`;
              })}>Release unconfirmed card payment</button></div>}
            </div>
          )}
          <div className={styles.dlActions}>
            {canManage && (open.status === 'issued' || open.status === 'partially_paid') && !paying && <button type="button" className="primary" disabled={busy} onClick={() => setPaying({ amount: (open.dueCents / 100).toFixed(2), method: 'bank_transfer', note: '', key: crypto.randomUUID() })}>Mark as paid</button>}
            <button type="button" className="secondary" onClick={() => openPdf(`${url}/${open.id}/pdf`)}><FileText size={17} aria-hidden="true" /> Download PDF</button>
            {canIssue && open.status !== 'void' && open.status !== 'draft' && <button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => { const r = await readJson<{ emailStatus: string; emailedTo?: string }>(await fetch(`${url}/${open.id}/email`, send('POST'))); return `Invoice ${open.invoiceNumber} ${emailWords(r)}.`; })}>Resend email</button>}
            {canManage && canCreate && open.status === 'issued' && open.paidCents === 0 && <button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => {
              if (!(await askConfirm({ title: `Edit invoice ${open.invoiceNumber}?`, message: 'It goes back to a draft with the same number. The pay link stops working until you issue it again, which emails the dealership the updated invoice.', confirmLabel: 'Edit invoice' }))) return;
              const f = await readJson<Full>(await fetch(`${url}/${open.id}/reopen`, send('POST')));
              setOpen(null); setDraft(fromFull(f));
              return `Invoice ${open.invoiceNumber} is a draft again. Make your changes, then Issue and email.`;
            })}><Pencil size={17} aria-hidden="true" /> Edit</button>}
            {canManage && open.status === 'issued' && <button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => { if (!(await askConfirm({ title: `Void invoice ${open.invoiceNumber}?`, message: 'It stays on record as void and can no longer be paid.', confirmLabel: 'Void invoice', tone: 'danger' }))) return; setOpen(await readJson<Full>(await fetch(`${url}/${open.id}/void`, send('POST')))); return 'Invoice voided.'; })}>Void</button>}
            <button type="button" className="secondary" onClick={() => { setOpen(null); setPaying(null); }}>Close</button>
          </div>
          {paying && (
            <form className={styles.dlSum} style={{ marginTop: 16 }} onSubmit={(e) => { e.preventDefault(); void act(async () => {
              await readJson(await fetch(`${base}/invoices/${open.id}/payments`, send('POST', { amountCents: cents(paying.amount), method: paying.method, notes: paying.note, idempotencyKey: paying.key })));
              setOpen(await readJson<Full>(await fetch(`${url}/${open.id}`, { cache: 'no-store' }))); setPaying(null);
              return `Payment of ${money(cents(paying.amount))} recorded on invoice ${open.invoiceNumber}.`;
            }); }}>
              <strong>Mark as paid</strong>
              <div className={styles.dlGrid} style={{ margin: 0 }}>
                <label className={styles.dlField}>Amount received<input inputMode="decimal" value={paying.amount} onChange={(e) => setPaying({ ...paying, amount: e.target.value.replace(/[^0-9.]/g, '') })} required disabled={busy} /></label>
                <label className={styles.dlField}>Paid by<Select value={paying.method} onChange={(e) => setPaying({ ...paying, method: e.target.value as 'bank_transfer' | 'cash' })} disabled={busy}><option value="bank_transfer">Bank transfer, e-transfer or cheque</option><option value="cash">Cash</option></Select></label>
                <label className={styles.dlField}>Note (optional)<input value={paying.note} onChange={(e) => setPaying({ ...paying, note: e.target.value })} maxLength={200} placeholder="e.g. Cheque 1042" disabled={busy} /></label>
              </div>
              <div className={styles.dlActions}>
                <button className="primary" disabled={busy || !(cents(paying.amount) > 0)}>Record payment</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => setPaying(null)}>Cancel</button>
              </div>
            </form>
          )}
        </section>
        </FormPopup>
      )}

      <section className={`${styles.card} ${styles.dlCard}`}>
        <div className={styles.dlFilters} role="group" aria-label="Filter invoices">
          <label>Dealership<Select value={filter.dealershipId} onChange={(e) => setFilter({ ...filter, dealershipId: e.target.value })}><option value="">All dealerships</option>{dealers.map((d) => <option key={d.id} value={d.id}>{d.accountName}</option>)}</Select></label>
          <div className={styles.dlFilterField}><span>From</span><DatePicker value={filter.from} max={filter.to || undefined} onChange={(e) => setFilter({ ...filter, from: e.target.value })} aria-label="From date" placeholder="Any date" /></div>
          <div className={styles.dlFilterField}><span>To</span><DatePicker value={filter.to} min={filter.from || undefined} onChange={(e) => setFilter({ ...filter, to: e.target.value })} aria-label="To date" placeholder="Any date" /></div>
          <label>Status<Select value={filter.status} onChange={(e) => setFilter({ ...filter, status: e.target.value })}><option value="">Any status</option><option value="draft">Draft</option><option value="sent">Sent</option><option value="viewed">Viewed</option><option value="issued">Unpaid (sent or viewed)</option><option value="overdue">Overdue</option><option value="partially_paid">Part paid</option><option value="paid">Paid</option><option value="void">Void</option></Select></label>
          <a className="secondary" href={`${url}/zip?${query}`} aria-disabled={!items?.length} onClick={(e) => { if (!items?.length) e.preventDefault(); }}><Download size={17} aria-hidden="true" /> Download all (ZIP)</a>
        </div>
        {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading invoices" />}
        {items && items.length === 0 && <p className={styles.dlLead}>No dealership invoices match.</p>}
        {items && items.length > 0 && <>
          <p className={styles.dlLead}>{items.length} invoice{items.length === 1 ? '' : 's'} · billed {money(totals.billed)} · still owing {money(totals.owing)}</p>
          <div style={{ overflowX: 'auto' }}>
            <table className={`${styles.table} ${styles.dlTable}`}>
              <thead><tr><th scope="col">Invoice</th><th scope="col">Dealership</th><th scope="col">PO</th><th scope="col">Date</th><th scope="col">Due</th><th scope="col">Total</th><th scope="col">Balance</th><th scope="col">Status</th><th scope="col" aria-label="PDF" /></tr></thead>
              <tbody>{items.map((i) => (
                <tr key={i.id}>
                  <td><button type="button" className={styles.dlLink} onClick={() => void view(i.id)}>{i.status === 'draft' ? `Draft${i.invoiceNumber ? ` (${i.invoiceNumber})` : ''}` : i.invoiceNumber}</button></td>
                  <td>{i.dealershipName}</td><td>{i.poNumber || '—'}</td><td>{showDay(i.invoiceDate ?? i.issuedAt ?? i.createdAt)}</td><td>{showDay(i.dueDate ?? i.dueAt)}</td><td>{money(i.totalCents)}</td><td>{money(i.dueCents)}</td>
                  <td><span className={i.overdue ? styles.dlOverdue : styles.dlBadge}>{statusOf(i)}</span></td>
                  <td><button type="button" className="secondary" onClick={() => openPdf(`${url}/${i.id}/pdf`)} aria-label={`Download PDF of ${i.status === 'draft' ? 'draft' : `invoice ${i.invoiceNumber}`}`}><FileText size={16} aria-hidden="true" /> PDF</button></td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        </>}
      </section>

      {series && canConfigure && (
        <form className={`${styles.resource} ${styles.dlCard}`} onSubmit={(e) => { e.preventDefault(); void act(async () => { setSeries(await readJson<Series>(await fetch(`/api/v1/organizations/${organizationId}/dealer-invoice-series`, send('PUT', series)))); return 'Number series saved.'; }); }}>
          <h3 className={styles.dlH3}>Automatic invoice numbers</h3>
          <p className={styles.dlLead}>Dealership invoices have their own numbers, apart from customer invoices. The next one will be {series.prefix}{series.nextNumber}. A number typed on an invoice is used instead and does not change this.</p>
          <div className={styles.dlGrid}>
            <label className={styles.dlField}>Prefix<input value={series.prefix} maxLength={24} onChange={(e) => setSeries({ ...series, prefix: e.target.value.toUpperCase().replace(/[^A-Z0-9-]/g, '') })} required disabled={busy} /></label>
            <label className={styles.dlField}>Next number<input type="number" min={1} value={series.nextNumber} onChange={(e) => setSeries({ ...series, nextNumber: Number(e.target.value) })} required disabled={busy} /></label>
          </div>
          <div><button className="secondary" disabled={busy}>Save numbers</button></div>
        </form>
      )}
    </div>
  );
}
