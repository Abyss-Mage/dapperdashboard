'use client';

import { useEffect } from 'react';
import { useParams } from 'next/navigation';

/* The link in a dealership invoice email (error404 #26, owner decision 2026-11-11 "login + email link"): it opens the dealership's portal
   for up to 30 days without a password and shows the invoice it was sent for. An expired link goes to the portal's sign-in. */
export default function PayLink() {
  const { token } = useParams<{ token: string }>();
  useEffect(() => {
    fetch('/api/v1/dealer-portal/link', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ token }) })
      .then(async (r) => {
        const b = await r.json().catch(() => ({}));
        if (r.ok && b.invoiceId) window.location.replace(`/dealer/invoices/${b.invoiceId}`);
        else window.location.replace('/dealer/sign-in?expired=1');
      })
      .catch(() => window.location.replace('/dealer/sign-in?expired=1'));
  }, [token]);
  return <main style={{ minHeight: '100svh', display: 'grid', placeItems: 'center', background: '#f4f3f0', color: '#1c1c20', fontFamily: "'Manrope', system-ui, sans-serif", fontSize: 18 }} data-no-motion="">Opening your invoice…</main>;
}
