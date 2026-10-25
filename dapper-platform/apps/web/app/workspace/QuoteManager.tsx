'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import Select from '../GlassSelect';

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

  function resetForm() {
    setIsFormOpen(false);
    setEditingQuoteId(null);
    setEditingVersion(1);
    setFormRequestId('');
    setFormCustomerId('');
    setFormVehicleId('');
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
          internalNotes: formInternalNotes,
          customerNote: formCustomerNote,
          taxCents,
          depositCents,
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
    if (!confirm(`Issue quotation ${quote.referenceNumber} to customer? Price amounts will be frozen.`)) {
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
            <button type="button" className="primary" onClick={() => setIsFormOpen(true)}>
              + New Quotation
            </button>
          )}
        </div>
      </div>

      <p style={{ fontSize: '0.9rem', color: 'var(--muted, #6b5558)' }}>
        Staff quotation pipeline attached to customer booking requests. Itemized revisions are frozen upon issuance and immutably accepted by customers.
      </p>

      {notice && <p role="status" style={{ color: 'var(--success, #15804f)', fontWeight: 600 }}>{notice}</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1} className="field-error">{error}</p>}

      {/* Create / Edit Form Modal */}
      {isFormOpen && (
        <form onSubmit={handleSaveQuote} style={{ border: '1px solid var(--border, #f0dcde)', padding: '1rem', borderRadius: '4px', margin: '1rem 0' }}>
          <h4>{editingQuoteId ? `Edit Draft (${editingVersion})` : 'New Quotation Draft'}</h4>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '0.75rem', marginBottom: '1rem' }}>
            <div>
              <label htmlFor="q-req-id">Booking Request ID *</label>
              <input
                id="q-req-id"
                required
                disabled={!!editingQuoteId}
                value={formRequestId}
                onChange={(e) => setFormRequestId(e.target.value)}
                placeholder="UUID of customer request"
              />
            </div>
            <div>
              <label htmlFor="q-cust-id">Customer ID *</label>
              <input
                id="q-cust-id"
                required
                disabled={!!editingQuoteId}
                value={formCustomerId}
                onChange={(e) => setFormCustomerId(e.target.value)}
                placeholder="Customer UUID"
              />
            </div>
            <div>
              <label htmlFor="q-veh-id">Vehicle / Equipment ID *</label>
              <input
                id="q-veh-id"
                required
                disabled={!!editingQuoteId}
                value={formVehicleId}
                onChange={(e) => setFormVehicleId(e.target.value)}
                placeholder="Vehicle UUID"
              />
            </div>
            <div>
              <label htmlFor="q-expires">Expiry (Optional)</label>
              <input
                id="q-expires"
                type="datetime-local"
                value={formExpiresAt}
                onChange={(e) => setFormExpiresAt(e.target.value)}
              />
            </div>
          </div>

          <h5>Line Items</h5>
          {lines.map((line, idx) => (
            <div key={idx} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem', alignItems: 'center' }}>
              <input
                style={{ flex: 3 }}
                required
                placeholder="Service or part description"
                value={line.description}
                onChange={(e) => {
                  const updated = [...lines];
                  updated[idx].description = e.target.value;
                  setLines(updated);
                }}
              />
              <input
                style={{ flex: 1, minWidth: '60px' }}
                type="number"
                min="1"
                placeholder="Qty"
                value={line.quantity}
                onChange={(e) => {
                  const updated = [...lines];
                  updated[idx].quantity = parseInt(e.target.value, 10) || 1;
                  setLines(updated);
                }}
              />
              <input
                style={{ flex: 1.5, minWidth: '90px' }}
                type="number"
                step="0.01"
                min="0"
                placeholder="Unit Price ($)"
                value={line.unitPriceDollars}
                onChange={(e) => {
                  const updated = [...lines];
                  updated[idx].unitPriceDollars = e.target.value;
                  setLines(updated);
                }}
              />
              {lines.length > 1 && (
                <button
                  type="button"
                  className="secondary"
                  onClick={() => setLines(lines.filter((_, i) => i !== idx))}
                  style={{ padding: '0.3rem 0.5rem' }}
                >
                  ✕
                </button>
              )}
            </div>
          ))}

          <button
            type="button"
            className="secondary"
            style={{ marginBottom: '1rem', fontSize: '0.85rem' }}
            onClick={() => setLines([...lines, { description: '', quantity: 1, unitPriceDollars: '0.00' }])}
          >
            + Add Line Item
          </button>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
            <div>
              <label htmlFor="q-tax">Tax ($)</label>
              <input
                id="q-tax"
                type="number"
                step="0.01"
                min="0"
                value={formTaxDollars}
                onChange={(e) => setFormTaxDollars(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="q-deposit">Deposit Required ($)</label>
              <input
                id="q-deposit"
                type="number"
                step="0.01"
                min="0"
                value={formDepositDollars}
                onChange={(e) => setFormDepositDollars(e.target.value)}
              />
            </div>
          </div>

          <div style={{ marginBottom: '1rem' }}>
            <label htmlFor="q-customer-note">Customer Note (Public)</label>
            <textarea
              id="q-customer-note"
              rows={2}
              value={formCustomerNote}
              onChange={(e) => setFormCustomerNote(e.target.value)}
              placeholder="Notes presented to customer on quote"
            />
          </div>

          <div style={{ marginBottom: '1rem' }}>
            <label htmlFor="q-internal-notes">Internal Notes (Staff Only)</label>
            <textarea
              id="q-internal-notes"
              rows={2}
              value={formInternalNotes}
              onChange={(e) => setFormInternalNotes(e.target.value)}
              placeholder="Internal staff notes"
            />
          </div>

          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="submit" className="primary" disabled={submitting}>
              {submitting ? 'Saving…' : editingQuoteId ? 'Update Draft' : 'Create Draft'}
            </button>
            <button type="button" className="secondary" onClick={resetForm}>
              Cancel
            </button>
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
