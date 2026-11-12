'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Download } from 'lucide-react';
import styles from './dealer.module.css';
import { API, day, dueDay, invoiceDay, money, readJson, stateOf, type Summary } from './lib';

/* The dealership's invoices (error404 #30, as in the owner's example): tick invoices to download them together, Pay now for the one
   ticked, filter by Paid / Unpaid / Past due, and page through 10, 25 or 50 at a time. */
export default function DealerInvoicesPage() {
  const router = useRouter();
  const [items, setItems] = useState<Summary[] | null>(null);
  const [error, setError] = useState('');
  const [show, setShow] = useState({ paid: false, unpaid: false, late: false });
  const [perPage, setPerPage] = useState(10);
  const [page, setPage] = useState(1);
  const [picked, setPicked] = useState<Set<string>>(new Set());

  useEffect(() => { fetch(`${API}/invoices`, { cache: 'no-store' }).then((r) => readJson<{ items: Summary[] }>(r)).then((b) => setItems(b.items)).catch((e) => setError(e.message)); }, []);

  const shown = useMemo(() => (items ?? []).filter((i) => {
    if (!show.paid && !show.unpaid && !show.late) return true;
    const t = stateOf(i).tone;
    return (show.paid && t === 'paid') || (show.unpaid && t === 'unpaid') || (show.late && t === 'late');
  }), [items, show]);
  const pages = Math.max(1, Math.ceil(shown.length / perPage));
  const current = Math.min(page, pages);
  const rows = shown.slice((current - 1) * perPage, current * perPage);
  const allOnPage = rows.length > 0 && rows.every((r) => picked.has(r.id));
  const toggle = (id: string) => setPicked((p) => { const n = new Set(p); if (n.has(id)) n.delete(id); else n.add(id); return n; });
  const pickedOpen = (items ?? []).filter((i) => picked.has(i.id) && i.dueCents > 0 && i.status !== 'void');

  return (
    <>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      <div className={styles.bar}>
        <div className={styles.barLeft}>
          <a className={styles.btnRed} href={`${API}/zip?ids=${[...picked].join(',')}`} aria-disabled={picked.size === 0} onClick={(e) => { if (!picked.size) e.preventDefault(); }}><Download size={17} aria-hidden="true" />Download{picked.size ? ` (${picked.size})` : ''}</a>
          <button type="button" className={styles.btnRed} disabled={pickedOpen.length === 0} onClick={() => router.push(pickedOpen.length === 1 ? `/dealer/invoices/${pickedOpen[0].id}?pay=1` : `/dealer/pay?ids=${pickedOpen.map((i) => i.id).join(',')}`)}>Pay now{pickedOpen.length > 1 ? ` (${pickedOpen.length})` : ''}</button>
          <label className={styles.field} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>Per page
            <select value={perPage} onChange={(e) => { setPerPage(Number(e.target.value)); setPage(1); }}>{[10, 25, 50].map((n) => <option key={n} value={n}>{n}</option>)}</select>
          </label>
        </div>
        <div className={styles.barLeft} role="group" aria-label="Show">
          <label className={styles.check}><input type="checkbox" checked={show.paid} onChange={(e) => { setShow({ ...show, paid: e.target.checked }); setPage(1); }} />Paid</label>
          <label className={styles.check}><input type="checkbox" checked={show.unpaid} onChange={(e) => { setShow({ ...show, unpaid: e.target.checked }); setPage(1); }} />Unpaid</label>
          <label className={styles.check}><input type="checkbox" checked={show.late} onChange={(e) => { setShow({ ...show, late: e.target.checked }); setPage(1); }} />Past due</label>
        </div>
      </div>

      <div className={styles.panel}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr>
              <th scope="col"><input type="checkbox" aria-label="Tick every invoice on this page" checked={allOnPage} onChange={() => setPicked((p) => { const n = new Set(p); rows.forEach((r) => (allOnPage ? n.delete(r.id) : n.add(r.id))); return n; })} /></th>
              <th scope="col">Invoice number</th><th scope="col">PO number</th><th scope="col">Invoice date</th><th scope="col" className={styles.num}>Amount</th><th scope="col" className={styles.num}>Balance</th><th scope="col">Due date</th><th scope="col">Status</th><th scope="col" aria-label="Actions" />
            </tr></thead>
            <tbody>
              {!items && !error && <tr><td colSpan={9}>Loading your invoices…</td></tr>}
              {items && rows.length === 0 && <tr><td colSpan={9}>No invoices to show.</td></tr>}
              {rows.map((i) => {
                const s = stateOf(i);
                const open = i.dueCents > 0 && i.status !== 'void';
                return (
                  <tr key={i.id}>
                    <td><input type="checkbox" aria-label={`Tick invoice ${i.invoiceNumber}`} checked={picked.has(i.id)} onChange={() => toggle(i.id)} /></td>
                    <td><Link className={styles.link} href={`/dealer/invoices/${i.id}`}>{i.invoiceNumber}</Link></td>
                    <td>{i.poNumber || '—'}</td>
                    <td>{day(invoiceDay(i))}</td>
                    <td className={styles.num}>{money(i.totalCents)}</td>
                    <td className={styles.num}>{money(i.dueCents)}</td>
                    <td>{day(dueDay(i))}</td>
                    <td><span className={`${styles.pill} ${styles[s.tone]}`}>{s.label}</span></td>
                    <td><div className={styles.rowActions}>
                      {open && <Link className={`${styles.btnRed} ${styles.small}`} href={`/dealer/invoices/${i.id}?pay=1`}>Pay now</Link>}
                      <Link className={`${styles.btn} ${styles.small}`} href={`/dealer/invoices/${i.id}`}>View</Link>
                    </div></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
      {items && shown.length > 0 && (
        <div className={styles.pager}>
          <span>Showing {(current - 1) * perPage + 1} to {Math.min(current * perPage, shown.length)} of {shown.length}</span>
          <div>
            <button type="button" className={`${styles.btn} ${styles.small}`} disabled={current <= 1} onClick={() => setPage(current - 1)}>Previous</button>
            <span style={{ alignSelf: 'center', padding: '0 8px' }}>Page {current} of {pages}</span>
            <button type="button" className={`${styles.btn} ${styles.small}`} disabled={current >= pages} onClick={() => setPage(current + 1)}>Next</button>
          </div>
        </div>
      )}
    </>
  );
}
