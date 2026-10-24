'use client';

import {useEffect, useRef, useState} from 'react';

export type EditableInvoiceDraft = {
  id: string; version: number; taxCents: number; notes: string;
  items?: {description: string; quantity: number; unitPriceCents: number}[];
};

function cents(value: string): number | null {
  if (!/^\d+(\.\d{1,2})?$/.test(value.trim())) return null;
  const [whole, fraction = ''] = value.trim().split('.');
  const result = Number(whole + fraction.padEnd(2, '0'));
  return Number.isSafeInteger(result) ? result : null;
}

export default function InvoiceDraftEditor({base, invoice, onSaved, onCancel, onReload}: {
  base: string; invoice: EditableInvoiceDraft;
  onSaved: () => Promise<void>; onCancel: () => void; onReload: () => Promise<void>;
}) {
  const [lines, setLines] = useState(() => (invoice.items ?? []).map(item => ({description: item.description, quantity: String(item.quantity), price: (item.unitPriceCents / 100).toFixed(2)})));
  const [tax, setTax] = useState((invoice.taxCents / 100).toFixed(2));
  const [notes, setNotes] = useState(invoice.notes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [conflict, setConflict] = useState(false);
  const errorRef = useRef<HTMLDivElement>(null);
  const firstField = useRef<HTMLInputElement>(null);
  useEffect(() => {firstField.current?.focus();}, []);
  useEffect(() => {if (error && !busy) errorRef.current?.focus();}, [error, busy]);
  const items = lines.map(line => ({description: line.description.trim(), quantity: /^\d+$/.test(line.quantity) ? Number(line.quantity) : NaN, unitPriceCents: cents(line.price)}));
  const taxCents = cents(tax);
  const subtotal = items.reduce((sum, item) => sum + item.quantity * (item.unitPriceCents ?? NaN), 0);
  const total = subtotal + (taxCents ?? NaN);
  const valid = items.length > 0 && items.length <= 100 && items.every(item => [...item.description].length >= 2 && [...item.description].length <= 255 && Number.isSafeInteger(item.quantity) && item.quantity > 0 && item.quantity <= 2147483647 && item.unitPriceCents !== null) && Number.isSafeInteger(total) && total >= 0 && [...notes.trim()].length <= 2000;
  const money = (value: number) => Number.isSafeInteger(value) ? new Intl.NumberFormat('en-CA', {style: 'currency', currency: 'CAD'}).format(value / 100) : 'Enter valid amounts';
  function change(index: number, field: 'description' | 'quantity' | 'price', value: string) {setLines(lines.map((line, i) => i === index ? {...line, [field]: value} : line));}
  async function save(event: React.FormEvent) {
    event.preventDefault();
    if (!valid) {setError('Enter 1–100 valid line items, positive whole quantities, non-negative amounts and notes of at most 2,000 characters.'); return;}
    setBusy(true); setError(''); setConflict(false);
    try {
      const response = await fetch(`${base}/invoices/${invoice.id}/draft`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({version: invoice.version, taxCents, notes: notes.trim(), items})});
      const body = await response.json();
      if (!response.ok) {setConflict(response.status === 409); throw new Error(body.error ?? body.errors?.invoice ?? 'Draft could not be saved.');}
      await onSaved();
    } catch (failure) {setError(failure instanceof Error ? failure.message : 'Draft could not be saved.');}
    finally {setBusy(false);}
  }
  return <form onSubmit={save} aria-label="Edit invoice draft" className="panel detail-card">
    <h4>Edit invoice draft</h4>
    <p>Revision {invoice.version}. Saving replaces the draft line items and reviewed tax amount. The customer and linked work stay unchanged.</p>
    {error && <div ref={errorRef} tabIndex={-1} role="alert" className="notice-banner error">{error}</div>}
    <fieldset disabled={busy} style={{border: 0, padding: 0, minWidth: 0}}>
      {lines.map((line, index) => <div key={index} style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,140px),1fr))', gap: '.5rem', marginBottom: '1rem'}}>
        <label>Item {index + 1} description<input ref={index === 0 ? firstField : undefined} required minLength={2} maxLength={255} value={line.description} onChange={e => change(index, 'description', e.target.value)} style={{width: '100%'}} /></label>
        <label>Quantity<input required inputMode="numeric" pattern="[0-9]+" value={line.quantity} onChange={e => change(index, 'quantity', e.target.value)} style={{width: '100%'}} /></label>
        <label>Unit price (CAD)<input required inputMode="decimal" value={line.price} onChange={e => change(index, 'price', e.target.value)} style={{width: '100%'}} /></label>
        <button type="button" className="secondary" disabled={lines.length <= 1} onClick={() => setLines(lines.filter((_, i) => i !== index))}>Remove item {index + 1}</button>
      </div>)}
      <button type="button" className="secondary" disabled={lines.length >= 100} onClick={() => setLines([...lines, {description: '', quantity: '1', price: ''}])}>Add draft item</button>
      <label style={{display: 'block', marginTop: '1rem'}}>Reviewed tax (CAD)<input required inputMode="decimal" value={tax} onChange={e => setTax(e.target.value)} /></label>
      <label style={{display: 'block', marginTop: '1rem'}}>Notes<textarea value={notes} maxLength={2000} rows={3} onChange={e => setNotes(e.target.value)} style={{width: '100%'}} /></label>
      <p aria-live="polite">Subtotal {money(subtotal)} · Tax {money(taxCents ?? NaN)} · Total {money(total)}</p>
      <div style={{display: 'flex', gap: '.5rem', flexWrap: 'wrap'}}>
        <button className="primary" type="submit">{busy ? 'Saving draft…' : 'Save draft changes'}</button>
        <button className="secondary" type="button" onClick={onCancel}>Cancel editing</button>
        {conflict && <button className="secondary" type="button" onClick={async () => {if (confirm('Discard these unsaved edits and reload the current invoice?')) {setBusy(true); try {await onReload();} finally {setBusy(false);}}}}>Discard edits and reload</button>}
      </div>
    </fieldset>
  </form>;
}
