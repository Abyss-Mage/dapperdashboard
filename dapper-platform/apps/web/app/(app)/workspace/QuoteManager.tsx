'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import Select from '../GlassSelect';
import { DateTimePicker } from '../Pickers';
import { askConfirm } from '../confirm';
import NumberField from '../NumberField';
import form from './quote.module.css';

type Req = { id: string; customerId: string; vehicleId: string; customerName: string; status: string; requestedStart: string; customerNote: string; vehicle?: { description?: string; identifier?: string }; offer?: { title?: string } };

type QuoteItem = {
  id: string;
  sortOrder: number;
  description: string;
  quantity: number;
  unitPriceCents: number;
  lineTotalCents: number;
  serviceId?: string;
  serviceName?: string;
};

type QuoteAcceptance = {
  id: string;
  quotationId: string;
  quotationVersion: number;
  customerId: string;
  actorId: string;
  contentSha256: string;
  createdAt: string;
};

type Quote = {
  id: string;
  organizationId: string;
  locationId: string;
  requestId: string;
  customerId: string;
  vehicleId: string;
  referenceNumber: string;
  status: 'draft' | 'issued' | 'accepted' | 'declined' | 'expired' | 'superseded' | 'fulfilled';
  version: number;
  currency: string;
  subtotalCents: number;
  taxCents: number;
  depositCents: number;
  totalCents: number;
  internalNotes?: string;
  customerNote?: string;
  expiresAt?: string;
  issuedAt?: string;
  issuedBy?: string;
  decidedAt?: string;
  bookingId?: string;
  createdAt: string;
  updatedAt: string;
  items: QuoteItem[];
  acceptance?: QuoteAcceptance;
};

type LineDraft = {
  description: string;
  quantity: number;
  unitPriceDollars: string;
};

function formatMoney(cents: number, currency: string = 'CAD'): string {
  if (!Number.isSafeInteger(cents)) return '$0.00';
  return new Intl.NumberFormat('en-CA', { style: 'currency', currency }).format(cents / 100);
}

export default function QuoteManager({
  organizationId,
  locationId,
  canCreate,
  canManage,
  canIssue,
}: {
  organizationId: string;
  locationId: string;
  canCreate: boolean;
  canManage: boolean;
  canIssue: boolean;
}) {
  const [quotes, setQuotes] = useState<Quote[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');

  // Form states for creating / editing quotes
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editingQuoteId, setEditingQuoteId] = useState<string | null>(null);
  const [editingVersion, setEditingVersion] = useState<number>(1);
  const [formRequestId, setFormRequestId] = useState('');
  const [formCustomerId, setFormCustomerId] = useState('');
  const [clientName, setClientName] = useState('');
  const [clientPhone, setClientPhone] = useState('');
  const [clientEmail, setClientEmail] = useState('');
  const [formVehicleId, setFormVehicleId] = useState('');
  const [formInternalNotes, setFormInternalNotes] = useState('');
  const [formCustomerNote, setFormCustomerNote] = useState('');
  const [formTaxDollars, setFormTaxDollars] = useState('0.00');
  const [formDepositDollars, setFormDepositDollars] = useState('0.00');
  const [formExpiresAt, setFormExpiresAt] = useState('');
  const [lines, setLines] = useState<LineDraft[]>([
    { description: 'Service Detail', quantity: 1, unitPriceDollars: '150.00' },
  ]);
  const [submitting, setSubmitting] = useState(false);
  const [requests, setRequests] = useState<Req[] | null>(null);

  // Fulfill modal states
  const [fulfillingQuote, setFulfillingQuote] = useState<Quote | null>(null);
  const [fulfillChannel, setFulfillChannel] = useState('online');
  const [fulfillNotes, setFulfillNotes] = useState('');
  const [fulfillExempt, setFulfillExempt] = useState(true);
  const [fulfillPaymentId, setFulfillPaymentId] = useState('');
  const [fulfilling, setFulfilling] = useState(false);

  const errorRef = useRef<HTMLParagraphElement>(null);
  const base = `/api/v1/organizations/${organizationId}/locations/${locationId}/quotes`;

  const loadQuotes = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fetch(base, { cache: 'no-store' });
      if (!res.ok) throw new Error('Could not load quotations');
      const data = await res.json();
      setQuotes(data.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error loading quotes');
    } finally {
      setLoading(false);
    }
  }, [base]);

  useEffect(() => {
    loadQuotes();
  }, [loadQuotes]);

  useEffect(() => {
    if (error && !loading) errorRef.current?.focus();
  }, [error, loading]);

  // The booking requests a quotation can be written for. Choosing one fills in the customer and vehicle.
  useEffect(() => {
    if (!isFormOpen) return;
    let live = true;
    fetch(`/api/v1/organizations/${organizationId}/locations/${locationId}/customer-booking-requests`, { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : { items: [] }))
      .then((b) => { if (live) setRequests((b.items ?? []) as Req[]); })
      .catch(() => { if (live) setRequests([]); });
    return () => { live = false; };
  }, [isFormOpen, organizationId, locationId]);
  const pickRequest = (id: string) => {
    setFormRequestId(id);
    const r = (requests ?? []).find((x) => x.id === id);
    setFormCustomerId(r?.customerId ?? ''); setFormVehicleId(r?.vehicleId ?? '');
  };
  const pickedRequest = (requests ?? []).find((x) => x.id === formRequestId);
  const lineCents = (l: LineDraft) => Math.round((Number(l.unitPriceDollars) || 0) * 100) * (Number(l.quantity) || 0);
  const subtotalCents = lines.reduce((n, l) => n + lineCents(l), 0);
  const taxCents = Math.round((Number(formTaxDollars) || 0) * 100), depositCents = Math.round((Number(formDepositDollars) || 0) * 100);

  function resetForm() {
    setIsFormOpen(false);
    setEditingQuoteId(null);
    setEditingVersion(1);
    setFormRequestId('');
    setFormCustomerId('');
    setFormVehicleId('');
    setClientName(''); setClientPhone(''); setClientEmail('');
    setFormInternalNotes('');
    setFormCustomerNote('');
    setFormTaxDollars('0.00');
    setFormDepositDollars('0.00');
    setFormExpiresAt('');
    setLines([{ description: '', quantity: 1, unitPriceDollars: '' }]);
  }

  function startEdit(q: Quote) {
    setEditingQuoteId(q.id);
    setEditingVersion(q.version);
    setFormRequestId(q.requestId);
    setFormCustomerId(q.customerId);
    setFormVehicleId(q.vehicleId);
    setFormInternalNotes(q.internalNotes || '');
    setFormCustomerNote(q.customerNote || '');
    setFormTaxDollars((q.taxCents / 100).toFixed(2));
    setFormDepositDollars((q.depositCents / 100).toFixed(2));
    setFormExpiresAt(q.expiresAt ? q.expiresAt.substring(0, 16) : '');
    setLines(
      q.items.map((it) => ({
        description: it.description,
        quantity: it.quantity,
        unitPriceDollars: (it.unitPriceCents / 100).toFixed(2),
      }))
    );
    setIsFormOpen(true);
  }

  async function handleSaveQuote(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError('');
    setNotice('');

    try {
      const taxCents = Math.round(parseFloat(formTaxDollars || '0') * 100);
      const depositCents = Math.round(parseFloat(formDepositDollars || '0') * 100);
      const items = lines.map((l) => ({
        description: l.description.trim(),
        quantity: Math.max(1, Number(l.quantity) || 1),
        unitPriceCents: Math.round(parseFloat(l.unitPriceDollars || '0') * 100),
      }));

      if (items.some((it) => !it.description || it.unitPriceCents < 0)) {
        throw new Error('All line items must have a description and valid price');
      }

      if (editingQuoteId) {
        // Edit draft
        const payload = {
          version: editingVersion,
          internalNotes: formInternalNotes,
          customerNote: formCustomerNote,
          taxCents,
          depositCents,
          expiresAt: formExpiresAt ? new Date(formExpiresAt).toISOString() : undefined,
          items,
        };
        const res = await fetch(`${base}/${editingQuoteId}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        if (!res.ok) {
          const body = await res.json();
          throw new Error(body.error || 'Failed to update quotation');
        }
        setNotice('Quotation draft updated successfully');
      } else {
        // Create draft
        const payload = {
          requestId: formRequestId.trim(),
          customerId: formCustomerId.trim(),
          vehicleId: formVehicleId.trim(),
          ...(formRequestId ? {} : { clientName: clientName.trim(), clientPhone: clientPhone.trim(), clientEmail: clientEmail.trim() }),
          internalNotes: formInternalNotes,
          customerNote: formCustomerNote,
          taxCents,
          depositCents: 0,
          expiresAt: formExpiresAt ? new Date(formExpiresAt).toISOString() : undefined,
          items,
        };
        const res = await fetch(base, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        if (!res.ok) {
          const body = await res.json();
          throw new Error(body.error || 'Failed to create quotation draft');
        }
        setNotice('Quotation draft created successfully');
      }

      resetForm();
      loadQuotes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleIssueQuote(quote: Quote) {
    if (!(await askConfirm({title: 'Issue this quotation?', message: `Issue quotation ${quote.referenceNumber} to the customer?`, detail: 'Price amounts will be frozen once it is issued.', confirmLabel: 'Issue quotation'}))) {
      return;
    }
    setError('');
    setNotice('');
    try {
      const res = await fetch(`${base}/${quote.id}/issue`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version: quote.version, note: 'Issued via workspace' }),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Failed to issue quotation');
      }
      setNotice(`Quotation ${quote.referenceNumber} issued successfully`);
      loadQuotes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Issuance failed');
    }
  }

  async function handleFulfillQuote(e: React.FormEvent) {
    e.preventDefault();
    if (!fulfillingQuote) return;
    setFulfilling(true);
    setError('');
    setNotice('');

    try {
      const requestKey = `staff-fulfill-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
      const payload = {
        requestKey,
        quoteId: fulfillingQuote.id,
        approvedNoDepositConfig: fulfillExempt,
        depositPaymentId: fulfillPaymentId.trim() || undefined,
        channel: fulfillChannel,
        notes: fulfillNotes.trim(),
      };
      const res = await fetch(`${base}/${fulfillingQuote.id}/fulfill`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Conversion to booking failed');
      }
      const data = await res.json();
      setNotice(`Quotation fulfilled! Confirmed Booking Reference: ${data.booking.id}`);
      setFulfillingQuote(null);
      loadQuotes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Fulfillment failed');
    } finally {
      setFulfilling(false);
    }
  }

  const filteredQuotes = quotes.filter((q) => {
    if (statusFilter === 'all') return true;
    return q.status === statusFilter;
  });

  return (
    <section className="panel detail-card" style={{ marginTop: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.5rem' }}>
        <h3>Quotation Management</h3>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          <label htmlFor="quote-filter" style={{ fontSize: '0.9rem' }}>Status:</label>
          <Select
            id="quote-filter"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            style={{ padding: '0.3rem 0.6rem' }}
          >
            <option value="all">All</option>
            <option value="draft">Draft</option>
            <option value="issued">Issued</option>
            <option value="accepted">Accepted</option>
            <option value="fulfilled">Fulfilled</option>
            <option value="declined">Declined</option>
            <option value="expired">Expired</option>
          </Select>
          {canCreate && !isFormOpen && (
            <button type="button" className="primary" style={{ whiteSpace: 'nowrap' }} onClick={() => setIsFormOpen(true)}>
              + New Quotation
            </button>
          )}
        </div>
      </div>


      {notice && <p role="status" style={{ color: 'var(--success, #15804f)', fontWeight: 600 }}>{notice}</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1} className="field-error">{error}</p>}

      {/* Create / Edit Form Modal */}
      {isFormOpen && (
        <form onSubmit={handleSaveQuote} className={form.form}>
          <h4>{editingQuoteId ? `Edit draft (revision ${editingVersion})` : 'New quotation'}</h4>
          {!editingQuoteId && !formRequestId && (
            <div className={form.grid3}>
              <label className={form.field} htmlFor="q-client">Client name
                <input id="q-client" required minLength={2} maxLength={100} autoComplete="off" value={clientName} onChange={(e) => setClientName(e.target.value)} placeholder="Who is this for?" />
              </label>
              <label className={form.field} htmlFor="q-client-phone">Phone (optional)<input id="q-client-phone" type="tel" maxLength={30} autoComplete="off" value={clientPhone} onChange={(e) => setClientPhone(e.target.value)} /></label>
              <label className={form.field} htmlFor="q-client-email">Email (optional)<input id="q-client-email" type="email" maxLength={254} autoComplete="off" value={clientEmail} onChange={(e) => setClientEmail(e.target.value)} /></label>
            </div>
          )}
          <div className={form.grid}>
            <label className={form.field} htmlFor="q-req-id">{editingQuoteId ? 'Booking request' : 'Booking request (optional)'}
              {editingQuoteId ? (
                <input id="q-req-id" value={formRequestId || 'None, quoted by client name'} disabled readOnly />
              ) : (
                <Select id="q-req-id" value={formRequestId} onChange={(e) => pickRequest(e.target.value)}>
                  <option value="">{requests === null ? 'Loading requests…' : 'None, quote a client by name'}</option>
                  {(requests ?? []).map((r) => <option key={r.id} value={r.id}>{r.customerName} · {r.offer?.title ?? 'Request'} · {new Date(r.requestedStart).toLocaleDateString('en-CA', { month: 'short', day: 'numeric' })}</option>)}
                </Select>
              )}
            </label>
            <label className={form.field} htmlFor="q-expires">Valid until (optional)
              <DateTimePicker id="q-expires" value={formExpiresAt} onChange={(e) => setFormExpiresAt(e.target.value)} />
            </label>
          </div>
          {pickedRequest && !editingQuoteId && (
            <div className={form.picked}>
              <strong>{pickedRequest.customerName}</strong>
              <span>{[pickedRequest.vehicle?.description, pickedRequest.vehicle?.identifier].filter(Boolean).join(' · ') || 'Vehicle on the request'}</span>
              {pickedRequest.customerNote && <span>“{pickedRequest.customerNote}”</span>}
            </div>
          )}

          <div className={form.lines}>
            <strong>Line items</strong>
            <div className={form.lineHead} aria-hidden="true"><span>Description</span><span>Qty</span><span>Unit price ($)</span><span style={{ textAlign: 'right' }}>Total</span><span /></div>
            {lines.map((line, idx) => (
              <div key={idx} className={form.line}>
                <input required placeholder="Service or part" aria-label={`Line ${idx + 1} description`} value={line.description} onChange={(e) => { const updated = [...lines]; updated[idx] = { ...updated[idx], description: e.target.value }; setLines(updated); }} />
                <NumberField min="1" aria-label={`Line ${idx + 1} quantity`} value={line.quantity} onChange={(e) => { const updated = [...lines]; updated[idx] = { ...updated[idx], quantity: parseInt(e.target.value, 10) || 1 }; setLines(updated); }} />
                <NumberField step="0.01" min="0" inputMode="decimal" placeholder="0.00" aria-label={`Line ${idx + 1} unit price`} value={line.unitPriceDollars} onChange={(e) => { const updated = [...lines]; updated[idx] = { ...updated[idx], unitPriceDollars: e.target.value }; setLines(updated); }} />
                <span className={form.lineTotal}>{formatMoney(lineCents(line))}</span>
                {lines.length > 1 ? <button type="button" className={`secondary ${form.remove}`} aria-label={`Remove line ${idx + 1}`} onClick={() => setLines(lines.filter((_, i) => i !== idx))}>✕</button> : <span />}
              </div>
            ))}
            <div><button type="button" className="secondary" onClick={() => setLines([...lines, { description: '', quantity: 1, unitPriceDollars: '' }])}>+ Add a line</button></div>
          </div>

          <div className={form.summary}>
            <label className={`${form.field} ${form.taxField}`} htmlFor="q-tax">Tax ($)<NumberField id="q-tax" step="0.01" min="0" inputMode="decimal" value={formTaxDollars} onChange={(e) => setFormTaxDollars(e.target.value)} /></label>
            <div className={form.totals} aria-live="polite">
              <div><span>Subtotal</span><span>{formatMoney(subtotalCents)}</span></div>
              <div><span>Tax</span><span>{formatMoney(taxCents)}</span></div>
              <div className={form.grand}><span>Total</span><span>{formatMoney(subtotalCents + taxCents)}</span></div>
            </div>
          </div>

          <div className={form.grid}>
            <label className={form.field} htmlFor="q-customer-note">Note for the customer<textarea id="q-customer-note" rows={3} value={formCustomerNote} onChange={(e) => setFormCustomerNote(e.target.value)} placeholder="Shown to the customer on the quote" /></label>
            <label className={form.field} htmlFor="q-internal-notes">Internal note (staff only)<textarea id="q-internal-notes" rows={3} value={formInternalNotes} onChange={(e) => setFormInternalNotes(e.target.value)} placeholder="Only your team sees this" /></label>
          </div>

          <div className={form.actions}>
            <button type="submit" className="primary" disabled={submitting || (!editingQuoteId && !formRequestId && clientName.trim().length < 2)}>{submitting ? 'Saving…' : editingQuoteId ? 'Update draft' : 'Create draft'}</button>
            <button type="button" className="secondary" onClick={resetForm}>Cancel</button>
          </div>
        </form>
      )}

      {/* Fulfill Modal */}
      {fulfillingQuote && (
        <form onSubmit={handleFulfillQuote} style={{ border: '2px solid var(--primary, #d10f1f)', padding: '1rem', borderRadius: '4px', margin: '1rem 0' }}>
          <h4>Fulfill Quotation {fulfillingQuote.referenceNumber} to Confirmed Booking</h4>
          <p>
            Total: {formatMoney(fulfillingQuote.totalCents, fulfillingQuote.currency)} | Deposit:{' '}
            {formatMoney(fulfillingQuote.depositCents, fulfillingQuote.currency)}
          </p>

          <div style={{ marginBottom: '0.75rem' }}>
            <label htmlFor="f-channel">Booking Channel</label>
            <Select id="f-channel" value={fulfillChannel} onChange={(e) => setFulfillChannel(e.target.value)}>
              <option value="online">Online</option>
              <option value="internal">Internal / Staff</option>
              <option value="phone">Phone</option>
              <option value="walk_in">Walk-in</option>
              <option value="dealership">Dealership</option>
            </Select>
          </div>

          {fulfillingQuote.depositCents > 0 && (
            <div style={{ marginBottom: '0.75rem' }}>
              <label>
                <input
                  type="checkbox"
                  checked={fulfillExempt}
                  onChange={(e) => setFulfillExempt(e.target.checked)}
                />{' '}
                Approved explicit no-deposit path configuration
              </label>
              {!fulfillExempt && (
                <div style={{ marginTop: '0.5rem' }}>
                  <label htmlFor="f-pay-id">Verified Completed Payment ID</label>
                  <input
                    id="f-pay-id"
                    required
                    value={fulfillPaymentId}
                    onChange={(e) => setFulfillPaymentId(e.target.value)}
                    placeholder="UUID of recorded payment"
                  />
                  <p style={{ fontSize: '0.8rem', color: 'var(--muted, #6b5558)', margin: '0.25rem 0' }}>
                    Enter the payment ID from the customer&apos;s settled deposit invoice (Invoices tab), or check the approved exemption if waived.
                  </p>
                </div>
              )}
            </div>
          )}

          <div style={{ marginBottom: '1rem' }}>
            <label htmlFor="f-notes">Booking Notes</label>
            <input
              id="f-notes"
              value={fulfillNotes}
              onChange={(e) => setFulfillNotes(e.target.value)}
              placeholder="Optional notes attached to the confirmed booking"
            />
          </div>

          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="submit" className="primary" disabled={fulfilling}>
              {fulfilling ? 'Converting…' : 'Confirm Booking & Fulfill Quote'}
            </button>
            <button type="button" className="secondary" onClick={() => setFulfillingQuote(null)}>
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Quotations List */}
      {loading ? (
        <p role="status">Loading quotations…</p>
      ) : filteredQuotes.length === 0 ? (
        <p>No quotations found matching the filter.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', marginTop: '1rem' }}>
          {filteredQuotes.map((q) => (
            <article
              key={q.id}
              style={{
                border: '1px solid var(--border, #f0dcde)',
                padding: '1rem',
                borderRadius: '4px',
                backgroundColor: 'var(--card-bg, #fff)',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap' }}>
                <span style={{ fontWeight: 700, fontSize: '1.1rem' }}>
                  {q.referenceNumber} · {q.status.toUpperCase()} (v{q.version})
                </span>
                <span style={{ fontWeight: 700 }}>
                  Total: {formatMoney(q.totalCents, q.currency)} (Subtotal: {formatMoney(q.subtotalCents, q.currency)} + Tax: {formatMoney(q.taxCents, q.currency)})
                </span>
              </div>

              <div style={{ fontSize: '0.85rem', color: 'var(--muted, #6b5558)', margin: '0.4rem 0' }}>
                Request: <code>{q.requestId}</code> · Customer: <code>{q.customerId}</code> · Created: {new Date(q.createdAt).toLocaleString()}
                {q.issuedAt && ` · Issued: ${new Date(q.issuedAt).toLocaleString()}`}
                {q.expiresAt && ` · Expires: ${new Date(q.expiresAt).toLocaleString()}`}
              </div>

              {q.customerNote && <p style={{ margin: '0.5rem 0', fontStyle: 'italic' }}>Note: &quot;{q.customerNote}&quot;</p>}

              {/* Line Items Table */}
              <div style={{ overflowX: 'auto', margin: '0.5rem 0' }}>
                <table style={{ width: '100%', fontSize: '0.85rem', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border, #f0dcde)', textAlign: 'left' }}>
                      <th style={{ padding: '0.3rem' }}>Description</th>
                      <th style={{ padding: '0.3rem', width: '50px' }}>Qty</th>
                      <th style={{ padding: '0.3rem', width: '90px' }}>Unit Price</th>
                      <th style={{ padding: '0.3rem', width: '90px' }}>Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {q.items.map((it) => (
                      <tr key={it.id} style={{ borderBottom: '1px solid var(--border, #f0dcde)' }}>
                        <td style={{ padding: '0.3rem' }}>{it.description}</td>
                        <td style={{ padding: '0.3rem' }}>{it.quantity}</td>
                        <td style={{ padding: '0.3rem' }}>{formatMoney(it.unitPriceCents, q.currency)}</td>
                        <td style={{ padding: '0.3rem', fontWeight: 600 }}>{formatMoney(it.lineTotalCents, q.currency)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Acceptance evidence */}
              {q.acceptance && (
                <div style={{ background: 'var(--bg-subtle, #fff1f2)', padding: '0.5rem', borderRadius: '3px', fontSize: '0.8rem', marginTop: '0.5rem' }}>
                  <strong>Customer Acceptance Evidence:</strong> Signed by actor <code>{q.acceptance.actorId}</code> at {new Date(q.acceptance.createdAt).toLocaleString()}
                  <br />
                  Content Fingerprint (SHA-256): <code>{q.acceptance.contentSha256}</code>
                </div>
              )}

              {/* Booking linkage */}
              {q.bookingId && (
                <div style={{ marginTop: '0.5rem', color: 'var(--primary, #d10f1f)', fontWeight: 600 }}>
                  Confirmed Booking ID: <code>{q.bookingId}</code>
                </div>
              )}

              {/* Actions */}
              <div style={{ marginTop: '0.75rem', display: 'flex', gap: '0.5rem' }}>
                {q.status === 'draft' && canManage && (
                  <button type="button" className="secondary" onClick={() => startEdit(q)}>
                    Edit Draft
                  </button>
                )}
                {q.status === 'draft' && canIssue && (
                  <button type="button" className="primary" onClick={() => handleIssueQuote(q)}>
                    Issue to Customer
                  </button>
                )}
                {q.status === 'accepted' && canManage && (
                  <button type="button" className="primary" onClick={() => setFulfillingQuote(q)}>
                    Fulfill to Confirmed Booking
                  </button>
                )}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
