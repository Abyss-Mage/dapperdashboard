'use client';

import { useCallback, useEffect, useState } from 'react';
import { DEFAULT_PRINT_TEMPLATE, InvoiceDocument, printInvoiceDocument, STARTER_CSS, STARTER_HTML, TEMPLATE_PLACEHOLDERS, type PrintInvoice, type PrintTemplate } from './InvoicePrint';
import Select from '../GlassSelect';
import { ColorPicker } from '../Pickers';

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

export default function InvoiceTemplateEditor({ organizationId, revision, locationId, locationName }: { organizationId: string; revision: number; locationId?: string; locationName?: string }) {
  // The template is shared by the whole organization unless this franchise is given its own.
  const [target, setTarget] = useState<'organization' | 'franchise'>('organization');
  const [hasOwn, setHasOwn] = useState(false);
  const [logoUrl, setLogoUrl] = useState('');
  const url = target === 'franchise' && locationId ? `/api/v1/organizations/${organizationId}/locations/${locationId}/invoice-template` : `/api/v1/organizations/${organizationId}/invoice-template`;
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
    setLogoUrl(b.logoUrl ?? '');
    const own = target === 'franchise' && b.scope === 'franchise';
    setHasOwn(own);
    setVersion(target === 'franchise' && !own ? 0 : b.version);
    setLoaded(true);
  }, [url, target]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load, revision]);
  useEffect(() => { setTarget('organization'); }, [locationId]);

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
      if (target === 'franchise') setHasOwn(true);
      setNotice('Template saved. New prints use it right away; amounts and customer details always come from the invoice.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The template could not be saved.');
    } finally { setBusy(false); }
  }

  const logoEndpoint = locationId ? `/api/v1/organizations/${organizationId}/locations/${locationId}/logo` : '';
  async function uploadLogo(file: File | undefined) {
    if (!file || !logoEndpoint) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(logoEndpoint, { method: 'PUT', headers: { 'Content-Type': file.type || 'application/octet-stream' }, body: file });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The logo could not be saved.');
      setLogoUrl(b.logoUrl ?? ''); setNotice('Logo saved for this franchise. New invoices print with it.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The logo could not be saved.'); }
    finally { setBusy(false); }
  }
  async function removeLogo() {
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(logoEndpoint, { method: 'DELETE' });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The logo could not be removed.');
      setLogoUrl(''); setNotice('Logo removed. Invoices use the default logo again.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The logo could not be removed.'); }
    finally { setBusy(false); }
  }

  async function removeOwn() {
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(url, { method: 'DELETE' });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The franchise template could not be removed.');
      setLoaded(false); await load();
      setNotice('This franchise now prints with the organization template.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The franchise template could not be removed.'); }
    finally { setBusy(false); }
  }

  const text = (key: 'title' | 'billToLabel' | 'itemLabel' | 'quantityLabel' | 'unitPriceLabel' | 'amountLabel', label: string, max: number) => (
    <label className="form-field" key={key}>{label}<input required maxLength={max} value={form[key]} disabled={busy || !loaded} onChange={(e) => set(key, e.target.value)} /></label>
  );

  return (
    <section className="panel detail-card">
      <h2>Invoice template{target === 'franchise' && locationName ? ` · ${locationName}` : ''}</h2>
      <p>How printed invoices look. Change the title, labels, colour, notes and paper size, or write your own layout in HTML and CSS. The invoice always supplies every amount, customer detail and number.</p>
      {locationId && (
        <div className="management-form" style={{ alignItems: 'end' }}>
          <label className="form-field">Which template are you editing?
            <Select value={target} disabled={busy} onChange={(e) => { setLoaded(false); setNotice(''); setError(''); setTarget(e.target.value as 'organization' | 'franchise'); }}>
              <option value="organization">The organization template (every franchise without its own)</option>
              <option value="franchise">{locationName ? `${locationName} only` : 'This franchise only'}</option>
            </Select>
          </label>
          {target === 'franchise' && <p className="help">{hasOwn ? 'This franchise has its own template.' : 'This franchise uses the organization template. Saving here gives it its own copy, started from the one below.'}</p>}
          {target === 'franchise' && (
            <div className="form-field">
              Logo for {locationName ?? 'this franchise'}
              <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
                {logoUrl ? <img src={logoUrl} alt="This franchise's logo" style={{ height: 56, maxWidth: 160, objectFit: 'contain', background: '#fff', borderRadius: 8, padding: 4 }} /> : <span className="help">Using the default logo.</span>}
                <label className="secondary" style={{ padding: '10px 16px', borderRadius: 12, cursor: 'pointer' }}>{logoUrl ? 'Replace logo' : 'Upload logo'}<input type="file" accept="image/png,image/jpeg,image/webp" hidden disabled={busy} onChange={(e) => { void uploadLogo(e.target.files?.[0]); e.target.value = ''; }} /></label>
                {logoUrl && <button type="button" className="secondary" disabled={busy} onClick={() => void removeLogo()}>Remove logo</button>}
              </div>
              <span className="help">PNG, JPEG or WebP, under 512 KB. It only shows when “Show the logo” is ticked below.</span>
            </div>
          )}
          {target === 'franchise' && hasOwn && <button type="button" className="secondary" disabled={busy} onClick={() => void removeOwn()}>Go back to the organization template</button>}
        </div>
      )}
      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      <form onSubmit={save}>
        <div className="management-form">
          {text('title', 'Document title', 40)}
          <label className="form-field">Accent colour
            <ColorPicker value={form.accentColor} disabled={busy || !loaded} onChange={(e) => set('accentColor', e.target.value)} />
          </label>
          <label className="form-field">Paper size
            <Select value={form.paperSize} disabled={busy || !loaded} onChange={(e) => set('paperSize', e.target.value as 'letter' | 'a4')}>
              <option value="letter">Letter (8.5 × 11 in)</option>
              <option value="a4">A4</option>
            </Select>
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
        <h3>Quotations</h3>
        <p>Quotations are printed in this same look. These two are their own wording.</p>
        <label className="form-field">Quotation title<input maxLength={40} value={form.quotationTitle ?? ''} disabled={busy || !loaded} onChange={(e) => set('quotationTitle', e.target.value)} placeholder="Quotation" /></label>
        <label className="form-field">Terms under the quotation total (optional)<textarea maxLength={1000} value={form.quotationNote ?? ''} disabled={busy || !loaded} onChange={(e) => set('quotationNote', e.target.value)} placeholder="For example how long the price is held and what is not included" /></label>
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
      <div style={{ background: '#fff', borderRadius: 12, padding: 24, overflowX: 'auto' }}><InvoiceDocument invoice={SAMPLE} template={{ ...form, logoUrl: target === 'franchise' ? logoUrl || undefined : undefined }} /></div>
    </section>
  );
}
