'use client';

import { useCallback, useEffect, useState } from 'react';
import { DEFAULT_PRINT_TEMPLATE, InvoiceDocument, printInvoiceDocument, STARTER_CSS, STARTER_HTML, TEMPLATE_PLACEHOLDERS, type PrintInvoice, type PrintTemplate } from './InvoicePrint';
import Select from '../GlassSelect';
import { ColorPicker } from '../Pickers';

/* Owner / Super Admin editor for the printed invoice and quotation layouts (two templates, kept apart). Wording and look only;
   amounts and parties always come from the document. */

// Clearly fictional, only used for the preview and the test print.
const SAMPLE: PrintInvoice = {
  invoiceNumber: 'SAMPLE-0001', status: 'issued', currency: 'CAD', issuedAt: new Date().toISOString(),
  subtotalCents: 26400, taxCents: 3432, totalCents: 29832, paidCents: 0, dueCents: 29832, serviceAt: new Date().toISOString(), paymentMethods: 'square_card',
  issuerSnapshot: { legalName: 'Your legal business name', address: '123 Example Street\nCity, Province A1A 1A1', taxRegistration: 'Tax number' },
  customerSnapshot: { name: 'Sample Customer', email: 'customer@example.com', phone: '555-0100' },
  items: [
    { description: 'Sample main service', quantity: 1, unitPriceCents: 19900, totalPriceCents: 19900 },
    { description: 'Sample add-on', quantity: 1, unitPriceCents: 6500, totalPriceCents: 6500 },
  ],
};
const SAMPLE_QUOTE: PrintInvoice = { ...SAMPLE, kind: 'quotation', invoiceNumber: 'QTE-SAMPLE', validUntil: new Date(Date.now() + 30 * 864e5).toISOString(), note: 'Sample note written on this quotation.' };

export default function InvoiceTemplateEditor({ organizationId, revision, locationId, locationName }: { organizationId: string; revision: number; locationId?: string; locationName?: string }) {
  // The template is shared by the whole organization unless this franchise is given its own.
  const [target, setTarget] = useState<'organization' | 'franchise'>('organization');
  // Three documents, each with its own template: invoices for brand-owned outlets, invoices for franchisee outlets, and quotations.
  const [doc, setDoc] = useState<'brand' | 'franchise' | 'quote' | 'dealer'>('brand');
  const quote = doc === 'quote';
  const dealer = doc === 'dealer';
  const file = quote ? 'quote-template' : dealer ? 'dealer-invoice-template' : 'invoice-template';
  const sharedQuery = doc === 'franchise' ? '?outlets=franchise' : '';
  const [outlet, setOutlet] = useState<'brand' | 'franchise'>('brand');
  const [hasOwn, setHasOwn] = useState(false);
  const [logoUrl, setLogoUrl] = useState('');
  const url = target === 'franchise' && locationId ? `/api/v1/organizations/${organizationId}/locations/${locationId}/${file}` : `/api/v1/organizations/${organizationId}/${file}${sharedQuery}`;
  const [form, setForm] = useState<PrintTemplate>(DEFAULT_PRINT_TEMPLATE);
  const [version, setVersion] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // The dealership template's preview is the real dealership PDF drawn by the server from these settings (error404 #22/#29).
  const [dealerPdf, setDealerPdf] = useState('');
  const samplePdf = useCallback(async (t: PrintTemplate) => {
    const q = locationId ? `?location=${encodeURIComponent(locationId)}` : '';
    const r = await fetch(`/api/v1/organizations/${organizationId}/dealer-invoice-template/sample${q}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ template: { ...t, customHtml: '', customCss: '' } }) });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'The preview could not be drawn.');
    return URL.createObjectURL(await r.blob());
  }, [organizationId, locationId]);
  useEffect(() => {
    if (!dealer || !loaded) return;
    let live = true;
    const t = setTimeout(() => { samplePdf(form).then((u) => { if (live) setDealerPdf((old) => { if (old) URL.revokeObjectURL(old); return u; }); }).catch(() => undefined); }, 500);
    return () => { live = false; clearTimeout(t); };
  }, [dealer, loaded, form, samplePdf]);

  const load = useCallback(async () => {
    const r = await fetch(url, { cache: 'no-store' });
    if (!r.ok) throw new Error('The template is unavailable or access has changed.');
    const b = await r.json();
    setForm({ ...DEFAULT_PRINT_TEMPLATE, ...b.template });
    setLogoUrl(b.logoUrl ?? '');
    if (b.outlet === 'brand' || b.outlet === 'franchise') setOutlet(b.outlet);
    const own = target === 'franchise' && b.scope === 'franchise';
    setHasOwn(own);
    setVersion(target === 'franchise' && !own ? 0 : b.version);
    setLoaded(true);
  }, [url, target]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load, revision]);
  useEffect(() => { setTarget('organization'); }, [locationId]);

  async function changeOutlet(next: 'brand' | 'franchise') {
    if (!locationId) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(`/api/v1/organizations/${organizationId}/locations/${locationId}/outlet-type`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ outlet: next }) });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The outlet type could not be changed.');
      setOutlet(next);
      setNotice(next === 'franchise' ? 'This outlet now prints with the franchisee invoice template.' : 'This outlet now prints with the brand-owned invoice template.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The outlet type could not be changed.');
    } finally { setBusy(false); }
  }

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

  const text = (key: 'title' | 'billToLabel', label: string, max: number) => (
    <label className="form-field" key={key}>{label}<input required maxLength={max} value={form[key]} disabled={busy || !loaded} onChange={(e) => set(key, e.target.value)} /></label>
  );

  return (
    <section className="panel detail-card">
      <h2>{quote ? 'Quotation template' : dealer ? 'Dealership invoice template' : doc === 'franchise' ? 'Invoice template · franchisee outlets' : 'Invoice template · brand-owned outlets'}{target === 'franchise' && locationName ? ` · ${locationName}` : ''}</h2>
      <div role="group" aria-label="Which template" style={{ display: 'flex', gap: 8, margin: '0 0 12px', flexWrap: 'wrap' }}>
        {([['brand', 'Invoice · brand-owned outlets'], ['franchise', 'Invoice · franchisee outlets'], ['quote', 'Quotation'], ['dealer', 'Dealership invoice']] as const).map(([id, label]) => (
          <button key={id} type="button" className={doc === id ? 'primary' : 'secondary'} aria-pressed={doc === id} disabled={busy} onClick={() => { if (doc !== id) { setLoaded(false); setNotice(''); setError(''); setDoc(id); } }}>{label}</button>
        ))}
      </div>
      <p>{quote ? 'How printed quotations look, separately from invoices. Until you save one, quotations use the invoice look with the title "Quotation".' : dealer ? 'How invoices to dealerships look, in the dealership layout: business details and logo, invoice number, date, due date and PO, Client Details, a table of item, description, quantity and line total, and the totals. Change the colour, wording, business details, footer and paper size; the preview below is the exact PDF dealerships receive.' : doc === 'franchise' ? 'How invoices look at franchisee outlets. Until you save one, franchisee outlets print with the brand-owned template.' : 'How invoices look at brand-owned outlets (the outlets Dapper itself owns).'} Change the title, labels, colour, notes and paper size, or write your own layout in HTML and CSS. Every amount, customer detail and number always comes from the document.</p>
      {locationId && (
        <div className="management-form" style={{ alignItems: 'end' }}>
          <label className="form-field">Which template are you editing?
            <Select value={target} disabled={busy} onChange={(e) => { setLoaded(false); setNotice(''); setError(''); setTarget(e.target.value as 'organization' | 'franchise'); }}>
              <option value="organization">{quote ? 'The shared quotation template (every outlet without its own)' : dealer ? 'The shared dealership invoice template (every outlet without its own)' : doc === 'franchise' ? 'The franchisee template (every franchisee outlet without its own)' : 'The brand-owned template (every brand-owned outlet without its own)'}</option>
              <option value="franchise">{locationName ? `${locationName} only` : 'This franchise only'}</option>
            </Select>
          </label>
          <label className="form-field">{locationName ?? 'This outlet'} is
            <Select value={outlet} disabled={busy} onChange={(e) => void changeOutlet(e.target.value as 'brand' | 'franchise')}>
              <option value="brand">Brand-owned (prints with the brand-owned invoice template)</option>
              <option value="franchise">A franchisee outlet (prints with the franchisee invoice template)</option>
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
        </div>
        <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '.5rem 0' }}><input type="checkbox" checked={form.showLogo} disabled={busy || !loaded} onChange={(e) => set('showLogo', e.target.checked)} /><span>Show the logo</span></label>
        <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '.5rem 0' }}><input type="checkbox" checked={form.showTaxRegistration} disabled={busy || !loaded} onChange={(e) => set('showTaxRegistration', e.target.checked)} /><span>Show the tax registration number</span></label>
        <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '.5rem 0' }}><input type="checkbox" checked={form.showCustomerContact} disabled={busy || !loaded} onChange={(e) => set('showCustomerContact', e.target.checked)} /><span>Show the customer&rsquo;s email and phone</span></label>
        <label className="form-field">Phone, email, website, corporation number (printed under the address)<textarea maxLength={500} value={form.businessDetails ?? ''} disabled={busy || !loaded} onChange={(e) => set('businessDetails', e.target.value)} /></label>
        <label className="form-field">Note above the items (optional)<textarea maxLength={500} value={form.headerNote} disabled={busy || !loaded} onChange={(e) => set('headerNote', e.target.value)} /></label>
        {!quote && <label className="form-field">Payment instructions (optional)<textarea maxLength={1000} value={form.paymentInstructions} disabled={busy || !loaded} onChange={(e) => set('paymentInstructions', e.target.value)} placeholder="How customers can pay, only if approved by the business" /></label>}
        {quote && <label className="form-field">Terms under the quotation total (optional)<textarea maxLength={1000} value={form.quotationNote ?? ''} disabled={busy || !loaded} onChange={(e) => set('quotationNote', e.target.value)} placeholder="For example how long the price is held and what is not included" /></label>}
        {quote && <label className="form-field" style={{ maxWidth: 320 }}>A new quotation is valid for (days, 0 = no date)<input type="number" min={0} max={365} step={1} value={form.validDays ?? 0} disabled={busy || !loaded} onChange={(e) => set('validDays', Math.max(0, Math.min(365, Math.round(Number(e.target.value) || 0))))} /></label>}
        <label className="form-field">Footer (optional)<textarea maxLength={1000} value={form.footerNote} disabled={busy || !loaded} onChange={(e) => set('footerNote', e.target.value)} placeholder="Thank-you message, terms, return policy" /></label>
        {!dealer && <>
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
        </>}
        <div style={{ display: 'flex', gap: '.5rem', flexWrap: 'wrap' }}>
          <button className="primary" disabled={busy || !loaded}>{busy ? 'Saving…' : 'Save template'}</button>
          <button type="button" className="secondary" disabled={busy || !loaded} onClick={() => { if (dealer) { samplePdf(form).then((u) => window.open(u, '_blank', 'noopener')).catch((e) => setError(e.message)); return; } printInvoiceDocument(quote ? SAMPLE_QUOTE : SAMPLE, { ...form, outlet: doc === 'franchise' ? 'franchise' : undefined }); }}>Print a sample</button>
          <button type="button" className="secondary" disabled={busy || !loaded} onClick={() => { setForm((f) => ({ ...DEFAULT_PRINT_TEMPLATE, ...(quote ? { title: 'Quotation', validDays: 30 } : {}), businessDetails: f.businessDetails })); setNotice('Built-in layout restored here. Save to keep it.'); }}>Restore built-in layout</button>
        </div>
      </form>
      <h3>Preview (sample data)</h3>
      {dealer ? (
        <div style={{ background: '#fff', borderRadius: 12, overflow: 'hidden' }}>
          {dealerPdf ? <iframe title="Dealership invoice preview" src={`${dealerPdf}#toolbar=0&navpanes=0&view=FitH`} style={{ display: 'block', width: '100%', height: 'min(1100px, 130vw)', border: 0 }} /> : <p style={{ padding: 24, margin: 0, color: '#555' }}>Drawing the preview…</p>}
        </div>
      ) : <div style={{ background: '#fff', borderRadius: 12, padding: 24, overflowX: 'auto' }}><InvoiceDocument invoice={quote ? SAMPLE_QUOTE : SAMPLE} template={{ ...form, logoUrl: target === 'franchise' ? logoUrl || undefined : undefined, outlet: doc === 'franchise' ? 'franchise' : undefined }} /></div>}
    </section>
  );
}
