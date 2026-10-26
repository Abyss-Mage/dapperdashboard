'use client';
import {useEffect, useState} from 'react';

type Snapshot = {totalCents: number; taxCents: number; notes: string; items?: {description: string; quantity: number; unitPriceCents: number}[]};
type Revision = {id: number; fromVersion: number; toVersion: number; createdAt: string; before: Snapshot; after: Snapshot};
const money = (value: number) => new Intl.NumberFormat('en-CA', {style: 'currency', currency: 'CAD'}).format(value / 100);
export default function InvoiceDraftHistory({base, id, version}: {base: string; id: string; version: number}) {
  const [revisions, setRevisions] = useState<Revision[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError('');
    (async () => {
      try {
        const response = await fetch(`${base}/invoices/${id}/draft-history`, {signal: controller.signal, cache: 'no-store'});
        const body = await response.json();
        if (!response.ok) throw new Error(body.error ?? 'Draft history could not be loaded.');
        setRevisions(body.items);
      } catch (failure) {if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'Draft history unavailable.');}
      finally {if (!controller.signal.aborted) setLoading(false);}
    })();
    return () => controller.abort();
  }, [base, id, version]);
  return <details style={{marginBlock: '1rem'}}>
    <summary>Draft revision history</summary>
    {loading && <p role="status">Loading draft history…</p>}
    {error && <p role="alert">{error}</p>}
    {!loading && !error && revisions.length === 0 && <p>No recorded draft edits.</p>}
    {revisions.map(revision => <details key={revision.id} style={{marginBlock: '.75rem'}}>
      <summary>Revision {revision.fromVersion} → {revision.toVersion} · {new Date(revision.createdAt).toLocaleString()} · {money(revision.before.totalCents)} → {money(revision.after.totalCents)}</summary>
      <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,220px),1fr))', gap: '1rem'}}>
        {([{label: 'Before', snapshot: revision.before}, {label: 'After', snapshot: revision.after}]).map(({label, snapshot}) => <div key={label}>
          <strong>{label}</strong>
          <ul>{snapshot.items?.map((item, i) => <li key={i}>{item.description} · {item.quantity} × {money(item.unitPriceCents)}</li>)}</ul>
          <p>Reviewed tax {money(snapshot.taxCents)} · Total {money(snapshot.totalCents)}</p>
          <p style={{whiteSpace: 'pre-wrap', overflowWrap: 'anywhere'}}>Notes: {snapshot.notes || 'None'}</p>
        </div>)}
      </div>
    </details>)}
    {revisions.length === 100 && <p>Showing the most recent 100 revisions.</p>}
  </details>;
}
