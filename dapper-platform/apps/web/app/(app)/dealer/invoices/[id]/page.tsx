'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { ArrowLeft, CreditCard, FileText } from 'lucide-react';
import SquareCardField from '../../../workspace/SquareCardField';
import styles from '../../dealer.module.css';
import { API, day, dueDay, invoiceDay, money, post, readJson, stateOf, type Card, type Invoice } from '../../lib';

/* One invoice in the dealership portal (error404 #26): the breakdown, Download PDF, and Pay by card with a saved card or a new one
   (which can be saved for next time). */
export default function DealerInvoicePage() {
  const { id } = useParams<{ id: string }>();
  const [inv, setInv] = useState<Invoice | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [paying, setPaying] = useState(false);
  const [choice, setChoice] = useState('new');
  const [save, setSave] = useState(true);
  const [cardReady, setCardReady] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [done, setDone] = useState('');
  const tokenize = useRef<(() => Promise<string>) | null>(null);
  const key = useRef('');

  const load = useCallback(async () => {
    const [i, c] = await Promise.all([fetch(`${API}/invoices/${id}`, { cache: 'no-store' }).then((r) => readJson<Invoice>(r)), fetch(`${API}/cards`, { cache: 'no-store' }).then((r) => readJson<{ items: Card[] }>(r)).catch(() => ({ items: [] as Card[] }))]);
    setInv(i); setCards(c.items);
    const def = c.items.find((x) => x.isDefault) ?? c.items[0];
    setChoice(def ? def.id : 'new');
    if (new URLSearchParams(window.location.search).get('pay') === '1' && i.dueCents > 0) setPaying(true);
  }, [id]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function pay(e: React.FormEvent) {
    e.preventDefault();
    if (!inv) return;
    setBusy(true); setError('');
    key.current ||= crypto.randomUUID(); // one key per attempt, so a double tap never charges twice
    try {
      const body: Record<string, unknown> = { amountCents: inv.dueCents, idempotencyKey: key.current };
      if (choice === 'new') {
        if (!tokenize.current) throw new Error('Enter the card details first.');
        body.sourceId = await tokenize.current();
        body.saveCard = save;
      } else body.cardId = choice;
      const res = await fetch(`${API}/invoices/${inv.id}/pay`, post(body));
      // A 4xx is a definite answer (nothing charged): the next try is a new attempt. After a 5xx the same key retries safely.
      if (res.status >= 400 && res.status < 500) key.current = '';
      const r = await readJson<{ payment: { amountCents: number }; invoice: Invoice; cardNotSaved?: boolean; cardNotSavedReason?: string }>(res);
      setInv(r.invoice); setPaying(false); key.current = '';
      setDone(`Thank you. Your payment of ${money(r.payment.amountCents)} was received.${r.cardNotSaved ? ` Your card was not saved for next time: ${r.cardNotSavedReason || 'Square did not accept it.'}` : ''}`);
      if (choice === 'new' && save) await load();
    } catch (err) { setError(err instanceof Error ? err.message : 'The payment could not be completed.'); } finally { setBusy(false); }
  }

  if (!inv) return error ? <p role="alert" className={styles.error}>{error}</p> : <p className={styles.note}>Loading the invoice…</p>;
  const s = stateOf(inv);
  const open = inv.dueCents > 0 && inv.status !== 'void';
  return (
    <>
      <p style={{ margin: '0 0 14px' }}><Link className={styles.link} href="/dealer"><ArrowLeft size={15} aria-hidden="true" style={{ verticalAlign: '-2px' }} /> All invoices</Link></p>
      {done && <p role="status" className={styles.ok}>{done}</p>}
      <div className={`${styles.panel} ${styles.pad}`}>
        <div className={styles.invHead}>
          <div><h2>Invoice {inv.invoiceNumber}</h2><p className={styles.note}>For {inv.dealershipName}</p></div>
          <span className={`${styles.pill} ${styles[s.tone]}`} style={{ fontSize: 15 }}>{s.label}</span>
        </div>
        <dl className={styles.facts}>
          <div><dt>Invoice date</dt><dd>{day(invoiceDay(inv))}</dd></div>
          <div><dt>Due date</dt><dd>{day(dueDay(inv))}</dd></div>
          <div><dt>PO number</dt><dd>{inv.poNumber || '—'}</dd></div>
          <div><dt>Balance</dt><dd>{money(inv.dueCents)}</dd></div>
        </dl>
        <div className={styles.panel}>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead><tr><th scope="col">Item No.</th><th scope="col">Description</th><th scope="col" className={styles.num}>Qty</th><th scope="col" className={styles.num}>Line total</th></tr></thead>
              <tbody>{inv.lines.map((l, i) => (
                <tr key={i}><td className={styles.wrap}>{l.description}</td><td className={styles.wrap}>{[l.detail, l.vin && `VIN ${l.vin}`, l.stockNumber && `Stock ${l.stockNumber}`].filter(Boolean).join(' · ') || '—'}</td><td className={styles.num}>{l.quantity}</td><td className={styles.num}>{money(l.unitPriceCents * l.quantity)}</td></tr>
              ))}</tbody>
            </table>
          </div>
        </div>
        <div className={styles.sums}>
          <div><span>Subtotal</span><span>{money(inv.subtotalCents)}</span></div>
          <div><span>Discount{inv.discountPercent ? ` (${inv.discountPercent}%)` : ''}</span><span>{money(inv.discountCents)}</span></div>
          <div><span>Tax</span><span>{money(inv.taxCents)}</span></div>
          <div className={styles.big}><span>Grand total</span><span>{money(inv.totalCents)}</span></div>
          {inv.paidCents > 0 && <div><span>Paid</span><span>- {money(inv.paidCents)}</span></div>}
          {inv.paidCents > 0 && <div className={styles.big}><span>Balance due</span><span>{money(inv.dueCents)}</span></div>}
        </div>
        {inv.notes && <p className={styles.note} style={{ marginTop: 16, whiteSpace: 'pre-wrap' }}>{inv.notes}</p>}
        <div className={styles.barLeft} style={{ marginTop: 22 }}>
          <a className={styles.btn} href={`${API}/invoices/${inv.id}/pdf`} target="_blank" rel="noopener"><FileText size={18} aria-hidden="true" />Download PDF</a>
          {open && !paying && <button type="button" className={styles.btnRed} onClick={() => { setPaying(true); setDone(''); }}><CreditCard size={18} aria-hidden="true" />Pay {money(inv.dueCents)} by card</button>}
        </div>
      </div>

      {paying && open && (
        <form className={styles.payBox} onSubmit={pay}>
          <h3>Pay {money(inv.dueCents)}</h3>
          {error && <p role="alert" className={styles.error}>{error}</p>}
          {cards.map((c) => (
            <label key={c.id} className={styles.cardChoice}><input type="radio" name="card" checked={choice === c.id} onChange={() => setChoice(c.id)} />{c.brand || 'Card'} ending {c.last4} · expires {c.expMonth}/{c.expYear}{c.isDefault ? ' · default' : ''}</label>
          ))}
          <label className={styles.cardChoice}><input type="radio" name="card" checked={choice === 'new'} onChange={() => setChoice('new')} />{cards.length ? 'Use a new card' : 'Card details'}</label>
          {choice === 'new' && <>
            <SquareCardField infoUrl={`${API}/square`} register={(fn) => { tokenize.current = fn; setCardReady(!!fn); }} unavailable="Card payments are not set up for this business yet. Contact them to pay another way." errorHint="Contact the business to pay another way." />
            <label className={styles.check}><input type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />Save this card for next time</label>
          </>}
          <div className={styles.barLeft}>
            <button className={styles.btnRed} disabled={busy || (choice === 'new' && !cardReady)}>{busy ? 'Processing…' : `Pay ${money(inv.dueCents)}`}</button>
            <button type="button" className={styles.btn} disabled={busy} onClick={() => { setPaying(false); setError(''); }}>Cancel</button>
          </div>
        </form>
      )}
    </>
  );
}
