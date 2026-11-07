'use client';
import { Smartphone } from 'lucide-react';
import { useEffect, useState } from 'react';
import glass from '../glass.module.css';

const cad = (cents: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(cents / 100);
type Prepared = { intentId: string; applicationId: string; environment: string; amountCents: number; currency: string; note: string };

/* Take the payment on the Square Reader. This opens the Square Point of Sale app on the phone or tablet the reader is paired to, with the amount filled
   in; when the customer has tapped or inserted their card the app returns here (see /workspace/square-reader), and the server reads the payment from
   Square before it records anything. It only works on a phone or tablet that has the Square Point of Sale app. */
export default function SquareReaderPayment({ prepareUrl, completeUrl, extraBody, invoiceId, dueCents }: { prepareUrl: string; completeUrl: (intentId: string) => string; extraBody?: Record<string, string>; invoiceId: string; dueCents: number }) {
  const [platform, setPlatform] = useState<'ios' | 'android' | 'other'>('other');
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  useEffect(() => { const ua = navigator.userAgent; setPlatform(/iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1) ? 'ios' : /Android/.test(ua) ? 'android' : 'other'); }, []);

  async function start() {
    setBusy(true); setError('');
    try {
      const r = await fetch(prepareUrl, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...extraBody, requestKey: `sqr-${crypto.randomUUID()}`, amountCents: dueCents }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The payment could not be started.');
      const p = b as Prepared;
      // Where the app comes back to, and what to settle when it does (kept in this browser tab only).
      sessionStorage.setItem('dapper:square-reader', JSON.stringify({ intentId: p.intentId, completeUrl: completeUrl(p.intentId), extraBody: extraBody ?? {}, back: location.pathname + location.search }));
      const callback = location.origin + '/workspace/square-reader';
      const link = platform === 'ios'
        ? 'square-commerce-v1://payment/create?data=' + encodeURIComponent(JSON.stringify({ amount_money: { amount: p.amountCents, currency_code: p.currency }, callback_url: callback, client_id: p.applicationId, version: '1.3', notes: p.note, state: p.intentId, options: { supported_tender_types: ['CREDIT_CARD'] } }))
        : `intent:#Intent;action=com.squareup.pos.action.CHARGE;package=com.squareup;S.com.squareup.pos.WEB_CALLBACK_URI=${encodeURIComponent(callback)};S.com.squareup.pos.CLIENT_ID=${p.applicationId};S.com.squareup.pos.API_VERSION=v2.0;i.com.squareup.pos.TOTAL_AMOUNT=${p.amountCents};S.com.squareup.pos.CURRENCY_CODE=${p.currency};S.com.squareup.pos.TENDER_TYPES=com.squareup.pos.TENDER_CARD;S.com.squareup.pos.NOTE=${encodeURIComponent(p.note)};S.com.squareup.pos.REQUEST_METADATA=${p.intentId};end`;
      window.location.href = link;
    } catch (e) { setError(e instanceof Error ? e.message : 'The payment could not be started.'); setBusy(false); }
  }

  return (
    <section className={glass.stage} aria-labelledby={`sqr-${invoiceId}`}><div className={glass.card}>
      <div className={glass.head}><h4 id={`sqr-${invoiceId}`} className={glass.title}><Smartphone size={17} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Pay with the Square Reader</h4></div>
      {platform === 'other'
        ? <p className={glass.lede}>Open this page on the phone or tablet that has the Square Point of Sale app and the reader paired, then press the button here.</p>
        : <>
          <p className={glass.lede}>Opens the Square Point of Sale app for {cad(dueCents)}. After the customer taps or inserts their card you come back here and the payment is recorded.</p>
          <div className={glass.actions}><button type="button" className={glass.button} disabled={busy || dueCents <= 0} onClick={() => void start()}>{busy ? 'Opening Square…' : `Charge ${cad(dueCents)} on the reader`}</button></div>
        </>}
      {error && <p className={glass.alert} role="alert">{error}</p>}
    </div></section>
  );
}
