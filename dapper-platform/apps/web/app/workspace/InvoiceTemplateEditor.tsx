'use client';

import { useCallback, useEffect, useState } from 'react';
import { DEFAULT_PRINT_TEMPLATE, InvoiceDocument, printInvoiceDocument, STARTER_CSS, STARTER_HTML, TEMPLATE_PLACEHOLDERS, type PrintInvoice, type PrintTemplate } from './InvoicePrint';

/* Owner / Super Admin editor for the printed invoice layout. Wording and look only; amounts and parties always come from the invoice. */

// Clearly fictional, only used for the preview and the test print.
const SAMPLE: PrintInvoice = {
  invoiceNumber: 'SAMPLE-0001', status: 'issued', currency: 'CAD', issuedAt: new Date().toISOString(),
  subtotalCents: 26400, taxCents: 3432, totalCents: 29832, paidCents: 0, dueCents: 29832,
  issuerSnapshot: { legalName: 'Your legal business name', address: '123 Example Street\nCity, Province A1A 1A1', taxRegistration: 'Tax number' },
  customerSnapshot: { name: 'Sample Customer', email: 'customer@example.com', phone: '555-0100' },
  items: [
    { description: 'Sample main service', quantity: 1, unitPriceCents: 19900, totalPriceCents: 19900 },
    { description: 'Sample add-on', quantity: 1, unitPriceCents: 6500, totalPriceCents: 6500 },
  ],
};

export default function InvoiceTemplateEditor({ organizationId, revision }: { organizationId: string; revision: number }) {
  const url = `/api/v1/organizations/${organizationId}/invoice-template`;
  const [form, setForm] = useState<PrintTemplate>(DEFAULT_PRINT_TEMPLATE);
  const [version, setVersion] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const r = await fetch(url, { cache: 'no-store' });
    if (!r.ok) throw new Error('The invoice template is unavailable or access has changed.');
    const b = await r.json();
    setForm({ ...DEFAULT_PRINT_TEMPLATE, ...b.template });
    setVersion(b.version);
    setLoaded(true);
  }, [url]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load, revision]);

  const set = <K extends keyof PrintTemplate>(key: K, value: PrintTemplate[K]) => { setForm((f) => ({ ...f, [key]: value })); setNotice(''); };

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ template: form, version }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The template could not be saved. Your changes are still here.');
      setForm({ ...DEFAULT_PRINT_TEMPLATE, ...b.template });
      setVersion(b.version);
      setNotice('Template saved. New prints use it right away; amounts and customer details always come from the invoice.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The template could not be saved.');
    } finally { setBusy(false); }
  }

  const text = (key: 'title' | 'billToLabel' | 'itemLabel' | 'quantityLabel' | 'unitPriceLabel' | 'amountLabel', label: string, max: number) => (
    <label className="form-field" key={key}>{label}<input required maxLength={max} value={form[key]} disabled={busy || !loaded} onChange={(e) => set(key, e.target.value)} /></label>
  );

  return (
    <section className="panel detail-card">
      <h2>Invoice template</h2>
      <p>How printed invoices look. Change the title, labels, colour, notes and paper size, or write your own layout in HTML and CSS. The invoice always supplies every amount, customer detail and number.</p>
      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      <form onSubmit={save}>
        <div className="management-form">
          {text('title', 'Document title', 40)}
          <label className="form-field">Accent colour
            <input type="color" value={form.accentColor} disabled={busy || !loaded} onChange={(e) => set('accentColor', e.target.value)} />
          </label>
          <label className="form-field">Paper size
            <select value={form.paperSize} disabled={busy || !loaded} onChange={(e) => set('paperSize', e.target.value as 'letter' | 'a4')}>
              <option value="letter">Letter (8.5 × 11 in)</option>
              <option value="a4">A4</option>
            </select>
          </label>
          {text('billToLabel', 'Customer heading', 30)}
          {text('itemLabel', 'Item column', 30)}
          {text('quantityLabel', 'Quantity column', 20)}
          {text('unitPriceLabel', 'Unit price column', 30)}
          {text('amountLabel', 'Amount column', 30)}
        </div>
        <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '.5rem 0' }}><input type="checkbox" checked={form.showLogo} disabled={busy || !loaded} onChange={(e) => set('showLogo', e.target.checked)} /><span>Show the logo</span></label>
        <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '.5rem 0' }}><input type="checkbox" checked={form.showTaxRegistration} disabled={busy || !loaded} onChange={(e) => set('showTaxRegistration', e.target.checked)} /><span>Show the tax registration number</span></label>
        <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '.5rem 0' }}><input type="checkbox" checked={form.showCustomerContact} disabled={busy || !loaded} onChange={(e) => set('showCustomerContact', e.target.checked)} /><span>Show the customer&rsquo;s email and phone</span></label>
        <label className="form-field">Note above the items (optional)<textarea maxLength={500} value={form.headerNote} disabled={busy || !loaded} onChange={(e) => set('headerNote', e.target.value)} /></label>
        <label className="form-field">Payment instructions (optional)<textarea maxLength={1000} value={form.paymentInstructions} disabled={busy || !loaded} onChange={(e) => set('paymentInstructions', e.target.value)} placeholder="How customers can pay, only if approved by the business" /></label>
        <label className="form-field">Footer (optional)<textarea maxLength={1000} value={form.footerNote} disabled={busy || !loaded} onChange={(e) => set('footerNote', e.target.value)} placeholder="Thank-you message, terms, return policy" /></label>
        <h3>Your own layout (HTML and CSS)</h3>
        <p>Optional. Write the invoice in HTML and style it with CSS; when there is HTML here it replaces the layout above. Put <code>{'{{placeholders}}'}</code> where the invoice details go. Scripts are not allowed and never run.</p>
        <details>
          <summary>Placeholders you can use</summary>
          <ul>{TEMPLATE_PLACEHOLDERS.map(([k, d]) => <li key={k}><code>{'{{' + k + '}}'}</code> {d}</li>)}</ul>
        </details>
        <label className="form-field">HTML<textarea rows={14} spellCheck={false} maxLength={60000} style={{ fontFamily: 'ui-monospace,Consolas,monospace', fontSize: 13 }} value={form.customHtml} disabled={busy || !loaded} onChange={(e) => set('customHtml', e.target.value)} placeholder="Leave empty to use the layout above" /></label>
        <label className="form-field">CSS<textarea rows={10} spellCheck={false} maxLength={60000} style={{ fontFamily: 'ui-monospace,Consolas,monospace', fontSize: 13 }} value={form.customCss} disabled={busy || !loaded} onChange={(e) => set('customCss', e.target.value)} /></label>
        <div style={{ display: 'flex', gap: '.5rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
          <button type="button" className="secondary" disabled={busy || !loaded} onClick={() => { setForm((f) => ({ ...f, customHtml: STARTER_HTML, customCss: STARTER_CSS })); setNotice('A starting layout was filled in. Edit it, then save.'); }}>Start from an example</button>
          <button type="button" className="secondary" disabled={busy || !loaded || !form.customHtml} onClick={() => { setForm((f) => ({ ...f, customHtml: '', customCss: '' })); setNotice('Your own layout was cleared here. Save to go back to the layout above.'); }}>Stop using my own layout</button>
        </div>
        <div style={{ display: 'flex', gap: '.5rem', flexWrap: 'wrap' }}>
          <button className="primary" disabled={busy || !loaded}>{busy ? 'Saving…' : 'Save template'}</button>
          <button type="button" className="secondary" disabled={busy || !loaded} onClick={() => printInvoiceDocument(SAMPLE, form)}>Print a sample</button>
          <button type="button" className="secondary" disabled={busy || !loaded} onClick={() => { setForm(DEFAULT_PRINT_TEMPLATE); setNotice('Built-in layout restored here. Save to keep it.'); }}>Restore built-in layout</button>
        </div>
      </form>
      <h3>Preview (sample data)</h3>
      <div style={{ background: '#fff', borderRadius: 12, padding: 24, overflowX: 'auto' }}><InvoiceDocument invoice={SAMPLE} template={form} /></div>
    </section>
  );
}
