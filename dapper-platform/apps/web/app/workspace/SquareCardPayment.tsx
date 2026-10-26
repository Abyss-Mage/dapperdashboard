'use client';
import { CreditCard } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import glass from '../glass.module.css';

type Binding = { configured: boolean; credentialsConfigured?: boolean; applicationId?: string; binding?: { environment: 'sandbox' | 'production'; merchantId: string; squareLocationId: string; version: number } };
type SquareCard = { attach(target: string | HTMLElement): Promise<void>; tokenize(): Promise<{ status: string; token?: string; errors?: { message: string }[] }>; destroy(): Promise<void> };
type SquareGlobal = { payments(applicationId: string, locationId: string): { card(options?: object): Promise<SquareCard> } };
declare global { interface Window { Square?: SquareGlobal } }

const sdkLoads = new Map<string, Promise<void>>();
function loadSdk(environment: string): Promise<void> {
  const src = environment === 'production' ? 'https://web.squarecdn.com/v1/square.js' : 'https://sandbox.web.squarecdn.com/v1/square.js';
  const cached = sdkLoads.get(src); if (cached) return cached;
  const promise = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script'); script.src = src; script.async = true;
    script.onload = () => resolve(); script.onerror = () => { sdkLoads.delete(src); reject(new Error('Square’s secure card form could not be loaded. Check the connection and try again.')); };
    document.head.appendChild(script);
  });
  sdkLoads.set(src, promise); return promise;
}
const cad = (cents: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(cents / 100);
const toCents = (text: string) => /^\d{1,7}(\.\d{1,2})?$/.test(text.trim()) ? Math.round(parseFloat(text) * 100) : NaN;

/* Card details are typed into Square's hosted field and exchanged for a single-use token in the browser.
   Our server only ever receives that token. The charge goes through the durable, merchant-bound intent route. */
export default function SquareCardPayment({ infoUrl, payUrl, extraBody, invoiceId, dueCents, onPaid }: { infoUrl: string; payUrl: string; extraBody?: Record<string, string>; invoiceId: string; dueCents: number; onPaid: () => Promise<void> }) {
  const [info, setInfo] = useState<Binding | null>(null), [fieldReady, setFieldReady] = useState(false), [amount, setAmount] = useState((dueCents / 100).toFixed(2));
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState(''), [locked, setLocked] = useState('');
  const cardRef = useRef<SquareCard | null>(null), host = useRef<HTMLDivElement>(null), attempt = useRef<{ key: string; cents: number } | null>(null);

  useEffect(() => { setAmount((dueCents / 100).toFixed(2)); }, [dueCents, invoiceId]);
  useEffect(() => {
    let live = true; setInfo(null);
    fetch(infoUrl, { cache: 'no-store' }).then(async r => { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Card payment settings are unavailable.'); if (live) setInfo(b as Binding); })
      .catch(e => { if (live) setError(e.message); });
    return () => { live = false; };
  }, [infoUrl]);

  const ready = !!info?.configured && !!info.credentialsConfigured && !!info.applicationId && !!info.binding;
  useEffect(() => {
    if (!ready || !info?.binding || !info.applicationId) return;
    let live = true, card: SquareCard | null = null; setFieldReady(false);
    (async () => {
      try {
        await loadSdk(info.binding!.environment);
        if (!live || !window.Square || !host.current) return;
        const ink = getComputedStyle(host.current).getPropertyValue('--text').trim() || '#1a0b0d';
        card = await window.Square.payments(info.applicationId!, info.binding!.squareLocationId).card({ style: { '.input-container': { borderRadius: '14px', borderColor: 'rgba(120,100,104,.35)' }, input: { color: ink, fontSize: '16px' }, 'input::placeholder': { color: '#8a7377' } } });
        if (!live) { await card.destroy(); return; }
        await card.attach(host.current); cardRef.current = card; setFieldReady(true);
      } catch (e) { if (live) setError(e instanceof Error ? e.message : 'The card form could not start.'); }
    })();
    return () => { live = false; cardRef.current = null; setFieldReady(false); void card?.destroy().catch(() => undefined); };
  }, [ready, info?.applicationId, info?.binding?.environment, info?.binding?.squareLocationId, info?.binding]);

  const cents = toCents(amount), valid = Number.isFinite(cents) && cents > 0 && cents <= dueCents;
  const submit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault(); if (!valid || !cardRef.current || busy || locked) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const token = await cardRef.current.tokenize();
      if (token.status !== 'OK' || !token.token) throw new Error(token.errors?.[0]?.message ?? 'Check the card details and try again.');
      if (!attempt.current || attempt.current.cents !== cents) attempt.current = { key: `sq-${crypto.randomUUID()}`, cents };
      const r = await fetch(payUrl, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...extraBody, requestKey: attempt.current.key, sourceId: token.token, amountCents: cents }) });
      const body = await r.json().catch(() => ({}));
      if (r.status === 502) { setLocked(body.intentId ?? 'unknown'); throw new Error(body.error ?? 'The payment could not be confirmed.'); }
      if (!r.ok) { attempt.current = null; throw new Error(body.error ?? body.errors?.amountCents ?? 'The card payment could not be completed.'); }
      attempt.current = null; setNotice(`Card payment of ${cad(cents)} confirmed by Square and recorded.`); await onPaid();
    } catch (err) { setError(err instanceof Error ? err.message : 'The card payment could not be completed.'); }
    finally { setBusy(false); }
  }, [busy, cents, extraBody, locked, onPaid, payUrl, valid]);

  const sandbox = info?.binding?.environment === 'sandbox';
  let body: React.ReactNode;
  if (!info && !error) body = <p className={glass.lede}>Checking card payment settings…</p>;
  else if (info && !info.configured) body = <p className={glass.lede}>Card payments are not set up for this location. An administrator can add the Square merchant in Finance settings.</p>;
  else if (info && (!info.credentialsConfigured || !info.applicationId)) body = <p className={glass.lede}>Square is saved for this location, but the server is missing its credentials or application ID, so card entry is unavailable.</p>;
  else if (info) body = <form onSubmit={submit} style={{ display: 'grid', gap: 14 }}>
    {sandbox && <p className={`${glass.notice} ${glass.warn}`} role="note">Sandbox: no real money moves. Use test card 4111 1111 1111 1111, any future date, any CVV and postal code.</p>}
    <label className={glass.field}>Amount (CAD)<input inputMode="decimal" required disabled={busy || !!locked} value={amount} onChange={e => setAmount(e.target.value)} aria-invalid={!valid} /></label>
    {!valid && <p className={glass.alert} role="alert">Enter an amount from $0.01 up to the balance due ({cad(dueCents)}).</p>}
    <div><span className={glass.field}>Card</span><div ref={host} style={{ minHeight: 92, marginTop: 6 }} aria-label="Secure card details" /></div>
    <p className={glass.lede}>Card details go straight to Square. Dapper never sees or stores the card number.</p>
    {locked && <p className={glass.alert} role="alert">Do not charge again. Ask an administrator to reconcile attempt <code>{locked}</code>, then <button type="button" className={glass.ghost} style={{ minHeight: 34, padding: '0 12px' }} onClick={() => { setLocked(''); attempt.current = null; setError(''); }}>start a new payment</button> only once they confirm no charge was made.</p>}
    <div className={glass.actions}><button className={glass.button} disabled={busy || !fieldReady || !valid || !!locked}>{busy ? 'Charging…' : `Charge ${valid ? cad(cents) : 'card'}`}</button></div>
  </form>;
  return <section className={glass.stage} aria-labelledby={`sq-${invoiceId}`}><div className={glass.card}>
    <div className={glass.head}><h4 id={`sq-${invoiceId}`} className={glass.title}><CreditCard size={17} aria-hidden="true" style={{ verticalAlign: '-3px', marginRight: 6 }} />Pay by card</h4>{sandbox && <span className={`${glass.pill} ${glass.pillWarn}`}>Sandbox</span>}</div>
    {body}{error && !locked && <p className={glass.alert} role="alert">{error}</p>}{locked && error && <p className={glass.alert} role="alert">{error}</p>}{notice && <p className={glass.notice} role="status">{notice}</p>}
  </div></section>;
}
