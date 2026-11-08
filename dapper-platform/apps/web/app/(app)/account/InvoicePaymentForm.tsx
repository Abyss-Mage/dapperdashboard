'use client';

import { useRef, useState } from 'react';
import SquareCardField from '../workspace/SquareCardField';

type InvoicePaymentFormProps = {
  customerId: string;
  invoiceId: string;
  invoiceNumber: string;
  dueCents: number;
  currency: string;
  onPaymentSuccess: () => void;
};

function amount(cents: number, currency: string) {
  if (!Number.isSafeInteger(cents)) return '$0.00';
  try {
    return new Intl.NumberFormat('en-CA', { style: 'currency', currency }).format(cents / 100);
  } catch {
    return '$0.00';
  }
}

export default function InvoicePaymentForm({
  customerId,
  invoiceId,
  invoiceNumber,
  dueCents,
  currency,
  onPaymentSuccess,
}: InvoicePaymentFormProps) {
  const tokenize = useRef<(() => Promise<string>) | null>(null);
  const [cardReady, setCardReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [showForm, setShowForm] = useState(false);

  async function handlePay(event: React.FormEvent) {
    event.preventDefault();
    if (!tokenize.current) return;
    setBusy(true);
    setError('');
    setNotice('');

    try {
      const sourceId = await tokenize.current();
      const res = await fetch(`/api/v1/portal/accounts/${encodeURIComponent(customerId)}/invoices/${encodeURIComponent(invoiceId)}/pay`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sourceId, amountCents: dueCents, idempotencyKey: crypto.randomUUID() }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || 'Invoice payment failed');
      }

      const data = await res.json();
      setNotice(`Payment of ${amount(data.payment.amountCents, currency)} processed successfully!`);
      setShowForm(false);
      onPaymentSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Payment could not be completed.');
    } finally {
      setBusy(false);
    }
  }

  if (dueCents <= 0) return null;

  return (
    <div style={{ marginTop: '0.5rem', background: 'var(--bg-subtle, #fff1f2)', padding: '0.6rem', borderRadius: '4px' }}>
      {notice && <p role="status" style={{ color: 'var(--success, #15804f)', fontWeight: 600, margin: '0 0 0.4rem' }}>{notice}</p>}
      {error && <p role="alert" className="field-error" style={{ margin: '0 0 0.4rem' }}>{error}</p>}

      {!showForm ? (
        <button
          type="button"
          className="secondary"
          onClick={() => setShowForm(true)}
          style={{ fontSize: '0.85rem', padding: '0.3rem 0.6rem' }}
        >
          Pay Balance ({amount(dueCents, currency)}) Online
        </button>
      ) : (
        <form onSubmit={handlePay} style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 600 }}>
            Pay Invoice #{invoiceNumber} Balance: {amount(dueCents, currency)}
          </div>
          <SquareCardField
            infoUrl={`/api/v1/portal/accounts/${encodeURIComponent(customerId)}/square`}
            register={(fn) => { tokenize.current = fn; setCardReady(!!fn); }}
          />
          <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
            <button
              type="submit"
              className="primary"
              disabled={busy || !cardReady}
              style={{ padding: '0.35rem 0.75rem', fontSize: '0.85rem' }}
            >
              {busy ? 'Processing…' : `Pay ${amount(dueCents, currency)}`}
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => { setShowForm(false); setError(''); }}
              disabled={busy}
              style={{ padding: '0.35rem 0.6rem', fontSize: '0.85rem' }}
            >
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
