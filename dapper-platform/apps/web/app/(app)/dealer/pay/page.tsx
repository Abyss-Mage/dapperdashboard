'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { ArrowLeft, CreditCard } from 'lucide-react';
import SquareCardField from '../../workspace/SquareCardField';
import styles from '../dealer.module.css';
import { API, day, dueDay, money, post, readJson, type Card, type Summary } from '../lib';

/* Pay several invoices at once (error404 #51): the ticked invoices, their total, and one card. Each invoice is charged on its own;
   a new card is saved first, since Square lets a new card be charged only once. */
type Result = { invoiceId: string; invoiceNumber: string; amountCents: number; paid: boolean; error?: string };

export default function DealerPayMany() {
  const [items, setItems] = useState<Summary[] | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [choice, setChoice] = useState('new');
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const [results, setResults] = useState<Result[] | null>(null);
  const tokenize = useRef<(() => Promise<string>) | null>(null);
  const key = useRef('');

  useEffect(() => {
    const ids = new Set((new URLSearchParams(window.location.search).get('ids') ?? '').split(',').filter(Boolean));
    Promise.all([fetch(`${API}/invoices`, { cache: 'no-store' }).then((r) => readJson<{ items: Summary[] }>(r)), fetch(`${API}/cards`, { cache: 'no-store' }).then((r) => readJson<{ items: Card[] }>(r)).catch(() => ({ items: [] as Card[] }))])
      .then(([inv, c]) => {
        setItems(inv.items.filter((i) => ids.has(i.id) && i.dueCents > 0 && i.status !== 'void'));
        setCards(c.items);
        const def = c.items.find((x) => x.isDefault) ?? c.items[0];
        setChoice(def ? def.id : 'new');
      })
      .catch((e) => setError(e.message));
  }, []);

  const total = (items ?? []).reduce((t, i) => t + i.dueCents, 0);
  async function pay(e: React.FormEvent) {
    e.preventDefault();
    if (!items?.length) return;
    setBusy(true); setError('');
    key.current ||= crypto.randomUUID();
    try {
      const body: Record<string, unknown> = { invoiceIds: items.map((i) => i.id), idempotencyKey: key.current };
      if (choice === 'new') {
        if (!tokenize.current) throw new Error('Enter the card details first.');
        body.sourceId = await tokenize.current();
      } else body.cardId = choice;
      const res = await fetch(`${API}/pay-many`, post(body));
      if (res.status >= 400 && res.status < 500) key.current = '';
      const r = await readJson<{ results: Result[]; paidCents: number }>(res);
      setResults(r.results); key.current = '';
    } catch (err) { setError(err instanceof Error ? err.message : 'The payment could not be completed.'); } finally { setBusy(false); }
  }

  if (results) return (
    <>
      <div className={`${styles.panel} ${styles.pad}`}>
        <h2 style={{ marginTop: 0 }}>{results.every((r) => r.paid) ? 'Thank you. Your payment was received.' : 'Some invoices were not paid'}</h2>
        <ul style={{ display: 'grid', gap: 8, paddingLeft: 18 }}>
          {results.map((r) => <li key={r.invoiceId}><strong>{r.invoiceNumber}</strong> · {money(r.amountCents)} · {r.paid ? 'Paid' : r.error}</li>)}
        </ul>
        {results.length < (items?.length ?? 0) && <p className={styles.note}>The remaining invoices were not charged.</p>}
        <Link className={styles.btn} href="/dealer">Back to invoices</Link>
      </div>
    </>
  );
  return (
    <>
      <p style={{ margin: '0 0 14px' }}><Link className={styles.link} href="/dealer"><ArrowLeft size={15} aria-hidden="true" style={{ verticalAlign: '-2px' }} /> All invoices</Link></p>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      {!items && !error && <p className={styles.note}>Loading…</p>}
      {items && items.length === 0 && <p className={styles.note}>None of the chosen invoices has anything left to pay.</p>}
      {items && items.length > 0 && <>
        <div className={styles.panel}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th scope="col">Invoice number</th><th scope="col">Due date</th><th scope="col" className={styles.num}>Balance</th></tr></thead>
              <tbody>{items.map((i) => <tr key={i.id}><td>{i.invoiceNumber}</td><td>{day(dueDay(i))}</td><td className={styles.num}>{money(i.dueCents)}</td></tr>)}</tbody>
            </table>
          </div>
        </div>
        <div className={styles.sums}><div className={styles.big}><span>Total</span><span>{money(total)}</span></div></div>
        <form className={styles.payBox} onSubmit={pay}>
          <h3>Pay {money(total)} for {items.length} invoice{items.length === 1 ? '' : 's'}</h3>
          {cards.map((c) => (
            <label key={c.id} className={styles.cardChoice}><input type="radio" name="card" checked={choice === c.id} onChange={() => setChoice(c.id)} />{c.brand || 'Card'} ending {c.last4} · expires {c.expMonth}/{c.expYear}{c.isDefault ? ' · default' : ''}</label>
          ))}
          <label className={styles.cardChoice}><input type="radio" name="card" checked={choice === 'new'} onChange={() => setChoice('new')} />{cards.length ? 'Use a new card' : 'Card details'}</label>
          {choice === 'new' && <>
            <SquareCardField infoUrl={`${API}/square`} register={(fn) => { tokenize.current = fn; setReady(!!fn); }} unavailable="Card payments are not set up for this business yet. Contact them to pay another way." errorHint="Contact the business to pay another way." />
            {items.length > 1 && <p className={styles.note}>The card is saved to your account so each invoice can be charged from it. You can remove it under Payment methods.</p>}
          </>}
          <div className={styles.barLeft}>
            <button className={styles.btnRed} disabled={busy || (choice === 'new' && !ready)}><CreditCard size={18} aria-hidden="true" />{busy ? 'Processing…' : `Pay ${money(total)}`}</button>
            <Link className={styles.btn} href="/dealer">Cancel</Link>
          </div>
        </form>
      </>}
    </>
  );
}
