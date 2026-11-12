'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import styles from '../dealer.module.css';
import { API, day, METHOD, money, readJson, type Payment } from '../lib';

/* Payments the dealership has made on its invoices, newest first. */
export default function DealerPaymentsPage() {
  const [items, setItems] = useState<Payment[] | null>(null);
  const [error, setError] = useState('');
  useEffect(() => { fetch(`${API}/payments`, { cache: 'no-store' }).then((r) => readJson<{ items: Payment[] }>(r)).then((b) => setItems(b.items)).catch((e) => setError(e.message)); }, []);
  return (
    <>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      <div className={styles.panel}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th scope="col">Date</th><th scope="col">Invoice</th><th scope="col">Method</th><th scope="col">Status</th><th scope="col" className={styles.num}>Amount</th></tr></thead>
            <tbody>
              {!items && !error && <tr><td colSpan={5}>Loading…</td></tr>}
              {items?.length === 0 && <tr><td colSpan={5}>No payments yet.</td></tr>}
              {items?.map((p) => (
                <tr key={p.id}>
                  <td>{day(p.recordedAt)}</td>
                  <td><Link className={styles.link} href={`/dealer/invoices/${p.invoiceId}`}>{p.invoiceNumber}</Link></td>
                  <td>{METHOD[p.method] ?? p.method}</td>
                  <td><span className={`${styles.pill} ${p.status === 'refunded' ? styles.void : styles.paid}`}>{p.status === 'refunded' ? 'Refunded' : 'Received'}</span></td>
                  <td className={styles.num}>{money(p.amountCents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
