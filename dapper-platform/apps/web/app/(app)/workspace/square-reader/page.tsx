'use client';
import { useEffect, useState } from 'react';

/* Where the Square Point of Sale app sends you back after a reader payment. iOS adds a "data" parameter (JSON), Android adds com.squareup.pos.*
   parameters. Either way we pass the payment id to the server, which reads the payment from Square and records it only if it matches the request. */
export default function SquareReaderReturn() {
  const [text, setText] = useState('Checking the payment with Square…');
  const [done, setDone] = useState(false);
  const [back, setBack] = useState('/workspace');

  useEffect(() => {
    (async () => {
      let saved: { intentId: string; completeUrl: string; extraBody: Record<string, string>; back: string } | null = null;
      try { saved = JSON.parse(sessionStorage.getItem('dapper:square-reader') || 'null'); } catch { /* none */ }
      const q = new URLSearchParams(location.search);
      let paymentId = '', error = '';
      const data = q.get('data');
      if (data) {
        try { const d = JSON.parse(data); paymentId = d.transaction_id || ''; error = d.error_code || ''; } catch { error = 'bad_response'; }
      } else {
        paymentId = q.get('com.squareup.pos.SERVER_TRANSACTION_ID') || '';
        error = q.get('com.squareup.pos.ERROR_CODE') || '';
      }
      if (saved?.back) setBack(saved.back);
      if (!saved) { setText('This payment was started in a different browser tab, so it cannot be recorded here. Check Square and record it from the invoice.'); setDone(true); return; }
      if (error || !paymentId) { setText(error ? `Square did not take the payment (${error.replace(/_/g, ' ').toLowerCase()}). Nothing was charged here.` : 'Square did not send back a payment, so nothing was recorded. If the customer was charged, check the Square app.'); setDone(true); return; }
      try {
        const r = await fetch(saved.completeUrl, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...saved.extraBody, paymentId }) });
        const b = await r.json().catch(() => ({}));
        if (!r.ok) throw new Error(b.error ?? 'The payment could not be confirmed.');
        sessionStorage.removeItem('dapper:square-reader');
        setText('Paid. The Square Reader payment is confirmed and recorded on the invoice.');
      } catch (e) { setText((e instanceof Error ? e.message : 'The payment could not be confirmed.') + ' If the customer was charged, ask an administrator to reconcile it.'); }
      setDone(true);
    })();
  }, []);

  return (
    <main className="secure-workspace" style={{ maxWidth: 560 }}>
      <h1>Square Reader payment</h1>
      <p role="status">{text}</p>
      {done && <p><a href={back}>Back to the invoice</a></p>}
    </main>
  );
}
