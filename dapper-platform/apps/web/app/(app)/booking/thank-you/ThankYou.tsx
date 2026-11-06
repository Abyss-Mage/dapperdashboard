'use client';

import { useEffect, useState } from 'react';
import { CalendarCheck } from 'lucide-react';
import styles from '../booking.module.css';

type Booked = { bookingId: string; invoiceNumber: string; totalCents: number; paidCents: number; paymentPending?: boolean; serviceName: string; pack: string; start: string; name: string; email: string; phone: string; address: string; notes: string };
type Win = Window & { dataLayer?: unknown[]; fbq?: (...a: unknown[]) => void };

const money = (c: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);

/* The confirmation, with everything the customer booked. Reporting happens here, once per booking: a paid-in-full booking is a Google tag
   (dataLayer) "purchase" and a Meta Pixel "Purchase"; one to be paid on the day is a "generate_lead" and a "Lead". Each carries the booking's value in CAD. The details come from the
   booking just made in this browser; the page address carries only the booking number. */
export default function ThankYou() {
  const [b, setB] = useState<Booked | null | undefined>();

  useEffect(() => {
    let got: Booked | null = null;
    try { got = JSON.parse(sessionStorage.getItem('dapper:booked') || localStorage.getItem('dapper:booked') || 'null'); } catch { /* fall through */ }
    // The spare copy exists only to get here; it is not left on a shared computer.
    try { localStorage.removeItem('dapper:booked'); } catch { /* nothing to remove */ }
    const id = new URLSearchParams(location.search).get('b') || '';
    const mine = got && (!id || got.bookingId === id) ? got : null;
    setB(mine);
    if (!mine || !id) return;
    const flag = 'dapper:reported:' + id;
    try { if (sessionStorage.getItem(flag)) return; sessionStorage.setItem(flag, '1'); } catch { /* report anyway */ }
    const w = window as Win;
    const value = Math.round(mine.totalCents) / 100;
    w.dataLayer = w.dataLayer || [];
    // Only money that has been paid is a purchase. A booking to be paid on the day is a lead, with the same value, so ad reports show real revenue.
    const paid = mine.paidCents >= mine.totalCents;
    const item = { item_id: mine.bookingId, item_name: mine.pack || mine.serviceName, price: value, quantity: 1 };
    w.dataLayer.push(paid
      ? { event: 'purchase', ecommerce: { transaction_id: mine.invoiceNumber || id, value, currency: 'CAD', items: [item] } }
      : { event: 'generate_lead', value, currency: 'CAD', transaction_id: mine.invoiceNumber || id, items: [item] });
    // The Meta Pixel is loaded by the page's tag loader a moment after the page opens, so wait for it (up to ten seconds).
    let tries = 0;
    const t = setInterval(() => {
      tries++;
      if (w.fbq) { clearInterval(t); w.fbq('track', paid ? 'Purchase' : 'Lead', { value, currency: 'CAD', content_name: mine.pack || mine.serviceName, content_type: 'product' }, { eventID: id }); }
      else if (tries > 40) clearInterval(t);
    }, 250);
    return () => clearInterval(t);
  }, []);

  const when = b?.start ? new Date(b.start).toLocaleString('en-CA', { weekday: 'long', month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit' }) : '';
  const rows: [string, string][] = b ? ([
    ['Service', b.pack || b.serviceName],
    ['When', when],
    ['Where', b.address ? b.address : 'At the Dapper shop'],
    ['Name', b.name],
    ['Phone', b.phone],
    ['Email', b.email],
    ['Reference', b.invoiceNumber],
    ['Total, tax included', money(b.totalCents)],
    ['Payment', b.paidCents >= b.totalCents ? `Paid in full online (${money(b.paidCents)})` : b.paymentPending ? 'Card payment is being confirmed. The Dapper team will follow up if anything is needed.' : 'Pay on the day'],
  ] as [string, string][]).filter((r) => !!r[1]) : [];

  return (
    <main className={styles.thanks}>
      <CalendarCheck size={40} aria-hidden="true" />
      <p className={styles.kicker}>Dapper Detailing Pros</p>
      <h1>Thank you. <em>You are booked.</em></h1>
      {b === undefined ? <p className={styles.lede}>Your booking is confirmed. Loading your booking summary…</p> : b ? <>
        <p className={styles.lede}>Your booking is confirmed and we are holding your time. {b.email ? `A confirmation goes to ${b.email}.` : ''} If anything needs to change, a team member will contact you.</p>
        <dl className={styles.recap}>{rows.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
      </> : <p className={styles.lede}>Your booking is confirmed and we are holding your time. We will be in touch by email or phone with anything that needs your attention.</p>}
      <div className={styles.thanksAct}>
        <a className={styles.btn} href="/account">View my account</a>
        <a className={styles.ghost} href="/">Back to the home page</a>
      </div>
    </main>
  );
}
