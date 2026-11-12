'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { Download } from 'lucide-react';
import styles from '../dealer.module.css';
import { API, day, METHOD, money, readJson, stateOf, type Payment } from '../lib';

/* The statement (error404 #31, as in the owner's example): From / To dates, status, Show payments and Show aging, the invoices with
   what is still owed, and Download as a PDF. */
type Row = { invoiceId: string; invoiceNumber: string; invoiceDate: string; dueDate: string; status: string; overdue: boolean; totalCents: number; balanceCents: number };
type Statement = { rows: Row[]; payments: Payment[]; balanceCents: number; billedCents: number; aging: { current: number; days1to30: number; days31to60: number; days61to90: number; over90: number } };

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

export default function DealerStatementPage() {
  const now = new Date();
  const [f, setF] = useState({ from: `${now.getFullYear()}-01-01`, to: iso(now), status: 'all', payments: false, aging: false });
  const [st, setSt] = useState<Statement | null>(null);
  const [error, setError] = useState('');
  const query = useMemo(() => new URLSearchParams({ from: f.from, to: f.to, status: f.status, payments: f.payments ? '1' : '0', aging: f.aging ? '1' : '0' }).toString(), [f]);
  useEffect(() => {
    if (!f.from || !f.to) return;
    setError('');
    fetch(`${API}/statement?${query}`, { cache: 'no-store' }).then((r) => readJson<Statement>(r)).then(setSt).catch((e) => setError(e.message));
  }, [query, f.from, f.to]);

  return (
    <>
      <div className={styles.bar}>
        <div className={styles.barLeft}>
          <label className={styles.field}>From<input type="date" value={f.from} max={f.to} onChange={(e) => setF({ ...f, from: e.target.value })} /></label>
          <label className={styles.field}>To<input type="date" value={f.to} min={f.from} onChange={(e) => setF({ ...f, to: e.target.value })} /></label>
          <label className={styles.field}>Status<select value={f.status} onChange={(e) => setF({ ...f, status: e.target.value })}><option value="all">All</option><option value="unpaid">Unpaid</option><option value="overdue">Past due</option><option value="paid">Paid</option></select></label>
          <label className={styles.check}><input type="checkbox" checked={f.payments} onChange={(e) => setF({ ...f, payments: e.target.checked })} />Show payments</label>
          <label className={styles.check}><input type="checkbox" checked={f.aging} onChange={(e) => setF({ ...f, aging: e.target.checked })} />Show aging</label>
        </div>
        <a className={styles.btnRed} href={`${API}/statement/pdf?${query}`} target="_blank" rel="noopener" aria-disabled={!st}><Download size={18} aria-hidden="true" />Download</a>
      </div>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      {st && <>
        <dl className={styles.facts} style={{ marginBottom: 16 }}>
          <div><dt className={styles.note}>Statement date</dt><dd style={{ margin: '4px 0 0', fontSize: 18, fontWeight: 750 }}>{day(f.from)} - {day(f.to)}</dd></div>
          <div><dt className={styles.note}>Billed</dt><dd style={{ margin: '4px 0 0', fontSize: 18, fontWeight: 750 }}>{money(st.billedCents)}</dd></div>
          <div><dt className={styles.note}>Balance due</dt><dd style={{ margin: '4px 0 0', fontSize: 18, fontWeight: 750 }}>{money(st.balanceCents)}</dd></div>
        </dl>
        <div className={styles.panel}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th scope="col">Invoice number</th><th scope="col">Invoice date</th><th scope="col">Due date</th><th scope="col">Status</th><th scope="col" className={styles.num}>Total</th><th scope="col" className={styles.num}>Balance</th></tr></thead>
              <tbody>
                {st.rows.length === 0 && <tr><td colSpan={6}>No invoices in these dates.</td></tr>}
                {st.rows.map((r) => { const s = stateOf(r); return (
                  <tr key={r.invoiceId}><td><Link className={styles.link} href={`/dealer/invoices/${r.invoiceId}`}>{r.invoiceNumber}</Link></td><td>{day(r.invoiceDate)}</td><td>{day(r.dueDate)}</td><td><span className={`${styles.pill} ${styles[s.tone]}`}>{s.label}</span></td><td className={styles.num}>{money(r.totalCents)}</td><td className={styles.num}>{money(r.balanceCents)}</td></tr>
                ); })}
              </tbody>
            </table>
          </div>
        </div>
        {f.payments && (
          <div className={styles.panel} style={{ marginTop: 18 }}>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead><tr><th scope="col">Payment date</th><th scope="col">Invoice</th><th scope="col">Method</th><th scope="col" className={styles.num}>Amount</th></tr></thead>
                <tbody>
                  {st.payments.length === 0 && <tr><td colSpan={4}>No payments in these dates.</td></tr>}
                  {st.payments.map((p) => <tr key={p.id}><td>{day(p.recordedAt)}</td><td>{p.invoiceNumber}</td><td>{METHOD[p.method] ?? p.method}</td><td className={styles.num}>{money(p.amountCents)}</td></tr>)}
                </tbody>
              </table>
            </div>
          </div>
        )}
        {f.aging && (
          <div className={styles.aging} aria-label="Aging of what is unpaid">
            <div><span>Current</span><strong>{money(st.aging.current)}</strong></div>
            <div><span>1-30 days</span><strong>{money(st.aging.days1to30)}</strong></div>
            <div><span>31-60 days</span><strong>{money(st.aging.days31to60)}</strong></div>
            <div><span>61-90 days</span><strong>{money(st.aging.days61to90)}</strong></div>
            <div><span>Over 90 days</span><strong>{money(st.aging.over90)}</strong></div>
          </div>
        )}
      </>}
    </>
  );
}
