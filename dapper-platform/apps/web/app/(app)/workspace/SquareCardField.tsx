'use client';

import { useEffect, useRef, useState } from 'react';

/* Square's hosted card field that only produces a single-use token (the amount and the charge are handled by our server).
   Card numbers never touch Dapper. */

type Info = { configured: boolean; credentialsConfigured?: boolean; applicationId?: string; binding?: { environment: 'sandbox' | 'production'; squareLocationId: string } };
type SquareCard = { attach(target: string | HTMLElement): Promise<void>; tokenize(): Promise<{ status: string; token?: string; errors?: { message: string }[] }>; destroy(): Promise<void> };
type SquareGlobal = { payments(applicationId: string, locationId: string): { card(options?: object): Promise<SquareCard> } };
declare global { interface Window { Square?: SquareGlobal } }

const sdk = new Map<string, Promise<void>>();
function loadSdk(environment: string): Promise<void> {
  const src = environment === 'production' ? 'https://web.squarecdn.com/v1/square.js' : 'https://sandbox.web.squarecdn.com/v1/square.js';
  const hit = sdk.get(src);
  if (hit) return hit;
  const p = new Promise<void>((resolve, reject) => {
    const el = document.createElement('script');
    el.src = src; el.async = true;
    el.onload = () => resolve();
    el.onerror = () => { sdk.delete(src); reject(new Error('The secure card form could not be loaded. Check your connection and try again.')); };
    document.head.appendChild(el);
  });
  sdk.set(src, p);
  return p;
}

export default function SquareCardField({ infoUrl, register, unavailable = 'Card payments are not available yet. Please choose to pay on the day.', errorHint = 'You can choose to pay on the day instead.' }: { infoUrl: string; register: (tokenize: (() => Promise<string>) | null) => void; unavailable?: string; errorHint?: string }) {
  const [info, setInfo] = useState<Info | null>(null);
  const [error, setError] = useState('');
  const host = useRef<HTMLDivElement>(null);
  const registerRef = useRef(register);
  registerRef.current = register;

  useEffect(() => {
    let live = true;
    fetch(infoUrl, { cache: 'no-store' })
      .then(async (r) => { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Card payments are not available right now.'); if (live) setInfo(b as Info); })
      .catch((e) => live && setError(e.message));
    return () => { live = false; };
  }, [infoUrl]);

  const ready = !!info?.configured && !!info.credentialsConfigured && !!info.applicationId && !!info.binding;
  useEffect(() => {
    if (!ready || !info?.binding || !info.applicationId) return;
    let live = true;
    let card: SquareCard | null = null;
    (async () => {
      try {
        await loadSdk(info.binding!.environment);
        if (!live || !window.Square || !host.current) return;
        card = await window.Square.payments(info.applicationId!, info.binding!.squareLocationId).card();
        if (!live) { await card.destroy(); return; }
        await card.attach(host.current);
        const mine = card;
        registerRef.current(async () => {
          const t = await mine.tokenize();
          if (t.status !== 'OK' || !t.token) throw new Error(t.errors?.[0]?.message ?? 'Check the card details and try again.');
          return t.token;
        });
      } catch (e) { if (live) setError(e instanceof Error ? e.message : 'The card form could not start.'); }
    })();
    return () => { live = false; registerRef.current(null); void card?.destroy().catch(() => undefined); };
  }, [ready, info?.applicationId, info?.binding]);

  if (error) return <p role="alert" style={{ color: 'var(--error)', margin: 0 }}>{error} {errorHint}</p>;
  if (!info) return <p style={{ margin: 0, color: 'var(--text-muted)' }}>Loading the secure card form…</p>;
  if (!ready) return <p style={{ margin: 0, color: 'var(--text-muted)' }}>{unavailable}</p>;
  const sandbox = info.binding?.environment === 'sandbox';
  return (
    <div style={{ display: 'grid', gap: 8 }}>
      {sandbox && <p style={{ margin: 0, color: 'var(--text-muted)', fontSize: 13 }}>Test mode: no real money moves. Use card 4111 1111 1111 1111, any future date, any CVV and postal code.</p>}
      <div ref={host} style={{ minHeight: 92 }} aria-label="Secure card details" />
      <p style={{ margin: 0, color: 'var(--text-muted)', fontSize: 13 }}>Your card details go straight to Square. We never see or store your card number.</p>
    </div>
  );
}
