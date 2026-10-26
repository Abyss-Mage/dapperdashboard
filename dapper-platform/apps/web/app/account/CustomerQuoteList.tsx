'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

type QuoteItem = {
  id: string;
  sortOrder: number;
  description: string;
  quantity: number;
  unitPriceCents: number;
  lineTotalCents: number;
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
  referenceNumber: string;
  status: 'issued' | 'accepted' | 'declined' | 'fulfilled' | 'expired';
  version: number;
  currency: string;
  subtotalCents: number;
  taxCents: number;
  depositCents: number;
  totalCents: number;
  customerNote?: string;
  expiresAt?: string;
  issuedAt?: string;
  decidedAt?: string;
  bookingId?: string;
  createdAt: string;
  items: QuoteItem[];
  acceptance?: QuoteAcceptance;
};

function formatMoney(cents: number, currency: string = 'CAD'): string {
  if (!Number.isSafeInteger(cents)) return '$0.00';
  return new Intl.NumberFormat('en-CA', { style: 'currency', currency }).format(cents / 100);
}

export default function CustomerQuoteList({
  customerId,
  revision = 0,
}: {
  customerId: string;
  revision?: number;
}) {
  const [quotes, setQuotes] = useState<Quote[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [actingOnQuoteId, setActingOnQuoteId] = useState<string | null>(null);
  const [termsAgreed, setTermsAgreed] = useState<{ [quoteId: string]: boolean }>({});
  const [declineReason, setDeclineReason] = useState<{ [quoteId: string]: string }>({});
  const [showDeclineBox, setShowDeclineBox] = useState<{ [quoteId: string]: boolean }>({});
  const [depositSourceId, setDepositSourceId] = useState<{ [quoteId: string]: string }>({});

  const errorRef = useRef<HTMLParagraphElement>(null);
  const endpoint = `/api/v1/portal/accounts/${customerId}/quotes`;

  const loadQuotes = useCallback(async () => {
    if (!customerId) return;
    setLoading(true);
    setError('');
    try {
      const res = await fetch(endpoint, { cache: 'no-store' });
      if (!res.ok) throw new Error('Quotations unavailable or access changed.');
      const data = await res.json();
      setQuotes(data.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Quotes unavailable.');
    } finally {
      setLoading(false);
    }
  }, [endpoint, customerId]);

  useEffect(() => {
    loadQuotes();
  }, [loadQuotes, revision]);

  useEffect(() => {
    if (error && !loading) errorRef.current?.focus();
  }, [error, loading]);

  async function handleAccept(quote: Quote) {
    if (!termsAgreed[quote.id]) {
      setError('Please review and check the terms agreement box before accepting.');
      return;
    }
    setActingOnQuoteId(quote.id);
    setError('');
    setNotice('');

    try {
      const acceptanceKey = `cust-accept-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
      const res = await fetch(`${endpoint}/${quote.id}/accept`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          acceptanceKey,
          termsReviewed: true,
        }),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Quotation acceptance failed');
      }
      const acceptedData: Quote = await res.json();
      setNotice(
        `Quotation ${quote.referenceNumber} accepted! SHA-256 fingerprint: ${acceptedData.acceptance?.contentSha256 || 'recorded'}`
      );
      loadQuotes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Acceptance failed');
    } finally {
      setActingOnQuoteId(null);
    }
  }

  async function handleDecline(quote: Quote) {
    setActingOnQuoteId(quote.id);
    setError('');
    setNotice('');

    try {
      const acceptanceKey = `cust-decline-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
      const res = await fetch(`${endpoint}/${quote.id}/decline`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          acceptanceKey,
          note: declineReason[quote.id] || 'Customer declined quote',
        }),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Quotation decline failed');
      }
      setNotice(`Quotation ${quote.referenceNumber} was declined.`);
      loadQuotes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Decline failed');
    } finally {
      setActingOnQuoteId(null);
    }
  }

  async function handlePayDeposit(quote: Quote) {
    const sourceId = (depositSourceId[quote.id] || '').trim();
    if (!sourceId) {
      setError('Please provide a valid card payment nonce or token.');
      return;
    }
    setActingOnQuoteId(quote.id);
    setError('');
    setNotice('');

    try {
      const idempotencyKey = `cust-dep-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
      const res = await fetch(`${endpoint}/${quote.id}/pay-deposit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sourceId,
          idempotencyKey,
        }),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || 'Deposit payment failed');
      }
      const data = await res.json();
      setNotice(
        `Deposit of ${formatMoney(data.amountCents, quote.currency)} paid successfully! ${data.receiptUrl ? `Receipt: ${data.receiptUrl}` : ''}`
      );
      loadQuotes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Deposit payment failed');
    } finally {
      setActingOnQuoteId(null);
    }
  }

  if (loading) return <p role="status">Loading quotations…</p>;

  return (
    <section className="panel detail-card" style={{ marginTop: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h3>Your Quotations</h3>
        <button type="button" className="secondary" onClick={loadQuotes} disabled={loading}>
          Refresh Quotations
        </button>
      </div>

      <p style={{ fontSize: '0.9rem', color: 'var(--muted, #6b5558)' }}>
        Itemized service quotes issued for your booking requests. Acceptance records an immutable cryptographic agreement before booking confirmation.
      </p>

      {notice && <p role="status" style={{ color: 'var(--success, #15804f)', fontWeight: 600 }}>{notice}</p>}
      {error && <p ref={errorRef} role="alert" tabIndex={-1} className="field-error">{error}</p>}

      {quotes.length === 0 ? (
        <p>No quotations issued for this account yet.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', marginTop: '1rem' }}>
          {quotes.map((q) => {
            const isIssued = q.status === 'issued';
            const isAccepted = q.status === 'accepted';
            const isFulfilled = q.status === 'fulfilled';
            const isDeclined = q.status === 'declined';
            const isActing = actingOnQuoteId === q.id;

            return (
              <article
                key={q.id}
                style={{
                  border: isAccepted || isFulfilled ? '2px solid var(--success, #15804f)' : '1px solid var(--border, #f0dcde)',
                  borderRadius: '4px',
                  padding: '1rem',
                  backgroundColor: 'var(--card-bg, #fff)',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap' }}>
                  <span style={{ fontWeight: 700, fontSize: '1.1rem' }}>
                    Quote #{q.referenceNumber}
                  </span>
                  <span
                    style={{
                      padding: '0.2rem 0.6rem',
                      borderRadius: '3px',
                      fontSize: '0.85rem',
                      fontWeight: 700,
                      backgroundColor:
                        isFulfilled || isAccepted
                          ? '#eaf6ee'
                          : isDeclined
                          ? '#ffe3e5'
                          : '#ece6e7',
                      color:
                        isFulfilled || isAccepted
                          ? '#15804f'
                          : isDeclined
                          ? '#a30b18'
                          : '#d10f1f',
                    }}
                  >
                    {q.status.toUpperCase()}
                  </span>
                </div>

                <div style={{ fontSize: '0.85rem', color: 'var(--muted, #6b5558)', margin: '0.4rem 0' }}>
                  Issued: {q.issuedAt ? new Date(q.issuedAt).toLocaleString() : 'Pending'}
                  {q.expiresAt && ` · Valid until: ${new Date(q.expiresAt).toLocaleString()}`}
                </div>

                {q.customerNote && (
                  <p style={{ margin: '0.5rem 0', fontStyle: 'italic', background: 'var(--bg-subtle, #fff1f2)', padding: '0.5rem', borderRadius: '3px' }}>
                    Note from Dapper: &quot;{q.customerNote}&quot;
                  </p>
                )}

                {/* Items Breakdown */}
                <div style={{ overflowX: 'auto', margin: '0.75rem 0' }}>
                  <table style={{ width: '100%', fontSize: '0.9rem', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--border, #f0dcde)', textAlign: 'left' }}>
                        <th style={{ padding: '0.4rem' }}>Item</th>
                        <th style={{ padding: '0.4rem', width: '50px' }}>Qty</th>
                        <th style={{ padding: '0.4rem', width: '100px' }}>Unit Price</th>
                        <th style={{ padding: '0.4rem', width: '100px' }}>Total</th>
                      </tr>
                    </thead>
                    <tbody>
                      {q.items.map((it) => (
                        <tr key={it.id} style={{ borderBottom: '1px solid var(--border, #f0dcde)' }}>
                          <td style={{ padding: '0.4rem' }}>{it.description}</td>
                          <td style={{ padding: '0.4rem' }}>{it.quantity}</td>
                          <td style={{ padding: '0.4rem' }}>{formatMoney(it.unitPriceCents, q.currency)}</td>
                          <td style={{ padding: '0.4rem', fontWeight: 600 }}>{formatMoney(it.lineTotalCents, q.currency)}</td>
                        </tr>
                      ))}
                    </tbody>
                    <tfoot>
                      <tr>
                        <td colSpan={3} style={{ textAlign: 'right', padding: '0.4rem' }}>Subtotal:</td>
                        <td style={{ padding: '0.4rem' }}>{formatMoney(q.subtotalCents, q.currency)}</td>
                      </tr>
                      <tr>
                        <td colSpan={3} style={{ textAlign: 'right', padding: '0.4rem' }}>Tax:</td>
                        <td style={{ padding: '0.4rem' }}>{formatMoney(q.taxCents, q.currency)}</td>
                      </tr>
                      <tr style={{ fontWeight: 700, fontSize: '1rem' }}>
                        <td colSpan={3} style={{ textAlign: 'right', padding: '0.4rem' }}>Total:</td>
                        <td style={{ padding: '0.4rem' }}>{formatMoney(q.totalCents, q.currency)}</td>
                      </tr>
                      {q.depositCents > 0 && (
                        <tr style={{ color: 'var(--primary, #d10f1f)', fontWeight: 600 }}>
                          <td colSpan={3} style={{ textAlign: 'right', padding: '0.4rem' }}>Deposit Required:</td>
                          <td style={{ padding: '0.4rem' }}>{formatMoney(q.depositCents, q.currency)}</td>
                        </tr>
                      )}
                    </tfoot>
                  </table>
                </div>

                {/* Acceptance Evidence */}
                {q.acceptance && (
                  <div style={{ background: '#eaf6ee', padding: '0.75rem', borderRadius: '4px', fontSize: '0.85rem', margin: '0.5rem 0' }}>
                    <p style={{ margin: 0, fontWeight: 700, color: '#15804f' }}>✓ Cryptographic Acceptance Recorded</p>
                    <p style={{ margin: '0.2rem 0', wordBreak: 'break-all' }}>
                      SHA-256 Content Hash: <code>{q.acceptance.contentSha256}</code>
                    </p>
                    <p style={{ margin: 0, color: 'var(--muted, #6b5558)' }}>
                      Accepted at {new Date(q.acceptance.createdAt).toLocaleString()}
                    </p>
                  </div>
                )}

                {/* Fulfilled Booking Link */}
                {q.bookingId && (
                  <div style={{ background: '#ece6e7', padding: '0.75rem', borderRadius: '4px', margin: '0.5rem 0' }}>
                    <strong>Booking Confirmed:</strong> Reference <code>{q.bookingId}</code>
                  </div>
                )}

                {/* Deposit Guidance and Payment for Accepted Quotes */}
                {isAccepted && !q.bookingId && q.depositCents > 0 && (
                  <div style={{ background: '#fff1d0', border: '1px solid #e8a200', padding: '0.85rem', borderRadius: '4px', fontSize: '0.9rem', margin: '0.75rem 0' }}>
                    <div style={{ fontWeight: 700, color: '#6b4400', marginBottom: '0.35rem' }}>
                      Deposit Payment Required ({formatMoney(q.depositCents, q.currency)})
                    </div>
                    <p style={{ margin: '0.2rem 0 0.6rem', color: 'var(--foreground, #1a0b0d)' }}>
                      To confirm your booking, a deposit of {formatMoney(q.depositCents, q.currency)} is required. Settle online below via credit card or pay in-person at drop-off.
                    </p>
                    <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
                      <input
                        type="text"
                        placeholder="Square card nonce (e.g. cnon:card-nonce-ok)"
                        value={depositSourceId[q.id] || ''}
                        onChange={(e) => setDepositSourceId((prev) => ({ ...prev, [q.id]: e.target.value }))}
                        disabled={isActing}
                        style={{ minWidth: '260px', padding: '0.4rem 0.6rem', fontSize: '0.85rem' }}
                      />
                      <button
                        type="button"
                        className="primary"
                        onClick={() => handlePayDeposit(q)}
                        disabled={isActing || !(depositSourceId[q.id] || '').trim()}
                        style={{ padding: '0.4rem 0.8rem', fontSize: '0.85rem' }}
                      >
                        {isActing ? 'Processing…' : `Pay Deposit (${formatMoney(q.depositCents, q.currency)})`}
                      </button>
                    </div>
                  </div>
                )}

                {/* Actions for Issued Quotes */}
                {isIssued && (
                  <div style={{ marginTop: '1rem', borderTop: '1px solid var(--border, #f0dcde)', paddingTop: '0.75rem' }}>
                    <label style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginBottom: '0.75rem', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={!!termsAgreed[q.id]}
                        onChange={(e) => setTermsAgreed({ ...termsAgreed, [q.id]: e.target.checked })}
                      />
                      <span style={{ fontSize: '0.9rem' }}>
                        I have reviewed and agree to the itemized quotation total of {formatMoney(q.totalCents, q.currency)}.
                      </span>
                    </label>

                    <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                      <button
                        type="button"
                        className="primary"
                        disabled={isActing || !termsAgreed[q.id]}
                        onClick={() => handleAccept(q)}
                      >
                        {isActing ? 'Recording Acceptance…' : 'Accept Quotation'}
                      </button>

                      {!showDeclineBox[q.id] ? (
                        <button
                          type="button"
                          className="secondary"
                          onClick={() => setShowDeclineBox({ ...showDeclineBox, [q.id]: true })}
                        >
                          Decline
                        </button>
                      ) : (
                        <div style={{ display: 'flex', gap: '0.5rem', width: '100%', marginTop: '0.5rem' }}>
                          <input
                            placeholder="Reason for declining (optional)"
                            value={declineReason[q.id] || ''}
                            onChange={(e) => setDeclineReason({ ...declineReason, [q.id]: e.target.value })}
                            style={{ flex: 1 }}
                          />
                          <button
                            type="button"
                            className="secondary"
                            style={{ color: '#a30b18' }}
                            disabled={isActing}
                            onClick={() => handleDecline(q)}
                          >
                            Confirm Decline
                          </button>
                          <button
                            type="button"
                            className="secondary"
                            onClick={() => setShowDeclineBox({ ...showDeclineBox, [q.id]: false })}
                          >
                            Cancel
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
