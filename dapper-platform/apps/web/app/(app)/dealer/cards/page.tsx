'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Check, Plus } from 'lucide-react';
import SquareCardField from '../../workspace/SquareCardField';
import styles from '../dealer.module.css';
import { API, day, post, readJson, type Card } from '../lib';
import { askConfirm } from '../../confirm';

/* Saved cards (error404 #27/#32): the dealership adds cards with Square, picks a default and removes old ones. Only the card's brand,
   last four digits and expiry are kept here; Square keeps the card. A saved card is only used when the dealership pays with it. */
export default function DealerCardsPage() {
  const [items, setItems] = useState<Card[] | null>(null);
  const [adding, setAdding] = useState(false);
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const tokenize = useRef<(() => Promise<string>) | null>(null);
  const load = useCallback(async () => setItems((await readJson<{ items: Card[] }>(await fetch(`${API}/cards`, { cache: 'no-store' }))).items), []);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function act(fn: () => Promise<string | void>) {
    setBusy(true); setError(''); setNotice('');
    try { const m = await fn(); if (m) setNotice(m); await load(); } catch (e) { setError(e instanceof Error ? e.message : 'That did not work. Try again.'); } finally { setBusy(false); }
  }
  const add = (e: React.FormEvent) => { e.preventDefault(); void act(async () => {
    if (!tokenize.current) throw new Error('Enter the card details first.');
    const sourceId = await tokenize.current();
    await readJson(await fetch(`${API}/cards`, post({ sourceId, idempotencyKey: crypto.randomUUID() })));
    setAdding(false);
    return 'Card saved.';
  }); };

  return (
    <>
      {error && <p role="alert" className={styles.error}>{error}</p>}
      {notice && <p role="status" className={styles.ok}>{notice}</p>}
      <div className={styles.bar}>
        <p className={styles.note}>Save a card to pay invoices in one tap. We only use it when you pay with it.</p>
        {!adding && <button type="button" className={styles.btnRed} onClick={() => { setAdding(true); setNotice(''); }}><Plus size={18} aria-hidden="true" />Add payment method</button>}
      </div>
      {adding && (
        <form className={styles.payBox} onSubmit={add} style={{ marginTop: 0, marginBottom: 18 }}>
          <h3>Add a card</h3>
          <SquareCardField infoUrl={`${API}/square`} register={(fn) => { tokenize.current = fn; setReady(!!fn); }} unavailable="Card payments are not set up for this business yet." errorHint="" />
          <div className={styles.barLeft}>
            <button className={styles.btnRed} disabled={busy || !ready}>{busy ? 'Saving…' : 'Save card'}</button>
            <button type="button" className={styles.btn} disabled={busy} onClick={() => setAdding(false)}>Cancel</button>
          </div>
        </form>
      )}
      <div className={styles.panel}>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead><tr><th scope="col">Date added</th><th scope="col">Payment type</th><th scope="col">Type</th><th scope="col">Expires</th><th scope="col">Card number</th><th scope="col">Default</th><th scope="col" aria-label="Actions" /></tr></thead>
            <tbody>
              {!items && !error && <tr><td colSpan={7}>Loading…</td></tr>}
              {items?.length === 0 && <tr><td colSpan={7}>No saved cards yet.</td></tr>}
              {items?.map((c) => (
                <tr key={c.id}>
                  <td>{day(c.createdAt)}</td><td>Credit card</td><td>{c.brand.replace(/_/g, ' ') || 'Card'}</td><td>{c.expMonth} / {c.expYear}</td><td>**** {c.last4}</td>
                  <td>{c.isDefault ? <Check size={20} aria-label="Default card" /> : ''}</td>
                  <td><div className={styles.rowActions}>
                    {!c.isDefault && <button type="button" className={`${styles.btn} ${styles.small}`} disabled={busy} onClick={() => void act(async () => { await readJson(await fetch(`${API}/cards/${c.id}/default`, post(undefined, 'PUT'))); return `Card ending ${c.last4} is now the default.`; })}>Make default</button>}
                    <button type="button" className={`${styles.btn} ${styles.small}`} disabled={busy} onClick={() => void act(async () => { if (!(await askConfirm({ title: `Remove the card ending ${c.last4}?`, message: 'It can no longer be used to pay. You can add it again later.', confirmLabel: 'Remove card', tone: 'danger' }))) return; await readJson(await fetch(`${API}/cards/${c.id}`, { method: 'DELETE' })); return 'Card removed.'; })}>Remove</button>
                  </div></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
