'use client';

import { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { flushSync } from 'react-dom';

/* The printed invoice. One layout for Workspace and Master; the owner edits its wording and look in Finance & Invoices > Invoice template.
   The template is presentation only: every amount, party and number is read from the invoice. */

export type PrintTemplate = {
  title: string; accentColor: string; showLogo: boolean; headerNote: string; billToLabel: string;
  itemLabel: string; quantityLabel: string; unitPriceLabel: string; amountLabel: string;
  showTaxRegistration: boolean; showCustomerContact: boolean; paymentInstructions: string; footerNote: string; paperSize: 'letter' | 'a4';
  /** Owner-written layout (HTML with {{placeholders}}) and its CSS. Empty = the built-in layout. */
  customHtml: string; customCss: string;
  /** The franchise's own logo, when it has one; otherwise the default logo is used. */
  logoUrl?: string;
  /** Quotation wording: the terms under the total and how many days a new quote is valid for (0 = no date). */
  quotationTitle?: string; quotationNote?: string; validDays?: number;
  /** Filled from the franchise when the template is loaded: the business details it prints (so drafts and quotes show them)
      and its active tax name and rate, e.g. HST at 13. */
  issuer?: { legalName: string; address: string; taxRegistration: string } | null; taxName?: string; taxRatePercent?: number;
  /** 'franchise' outlets print the line saying who independently owns them. */
  outlet?: string;
  /** Extra lines under the address: phone, email, website, corporation number. */
  businessDetails?: string;
};
export type PrintInvoice = {
  /** 'quotation' prints a quotation: no paid or pending lines, a valid-until date and the quotation terms. */
  kind?: 'invoice' | 'quotation'; validUntil?: string | null; note?: string;
  invoiceNumber: string; status: string; currency: string; issuedAt?: string | null; createdAt?: string;
  subtotalCents: number; taxCents: number; totalCents: number; paidCents: number; dueCents: number; taxRatePercent?: number;
  discountCents?: number; couponCode?: string;
  /** When the booking is for, and how it was paid ("cash,square_card"). */
  serviceAt?: string | null; paymentMethods?: string;
  customerName?: string;
  issuerSnapshot?: { legalName: string; address: string; taxRegistration: string } | null;
  customerSnapshot?: { name: string; email: string; phone: string } | null;
  items?: { description: string; quantity: number; unitPriceCents?: number; totalPriceCents: number }[];
};

export const DEFAULT_PRINT_TEMPLATE: PrintTemplate = {
  title: 'Invoice', accentColor: '#fb1a2b', showLogo: true, headerNote: '', billToLabel: 'Bill to', itemLabel: 'Item', quantityLabel: 'Qty',
  unitPriceLabel: 'Unit price', amountLabel: 'Amount', showTaxRegistration: true, showCustomerContact: true, paymentInstructions: '', footerNote: 'Thank you for choosing us', paperSize: 'letter',
  customHtml: '', customCss: '', quotationTitle: 'Quotation', quotationNote: '', validDays: 0,
};

const STATUS_WORDS: Record<string, string> = { draft: 'Draft', issued: 'Issued', partially_paid: 'Partially paid', paid: 'Paid', void: 'Void', refunded: 'Refunded' };
const statusWord = (s: string) => STATUS_WORDS[s] ?? s.replaceAll('_', ' ');

/* The tax line names its rate: the franchise's tax name and rate when known, otherwise the rate worked out from the amounts. */
export function taxLabel(invoice: PrintInvoice, t: PrintTemplate): string {
  const base = invoice.subtotalCents - (invoice.discountCents ?? 0);
  const derived = base > 0 && invoice.taxCents > 0 ? Math.round((invoice.taxCents / base) * 10000) / 100 : 0;
  const rate = invoice.taxRatePercent ?? (derived || t.taxRatePercent || 0);
  if (!rate) return 'Tax';
  const pct = (Math.round(rate * 100) / 100).toString() + '%';
  return t.taxName ? `${t.taxName} (${pct})` : `Tax (${pct})`;
}
const issuerOf = (invoice: PrintInvoice, t: PrintTemplate) => (invoice.issuerSnapshot?.legalName ? invoice.issuerSnapshot : t.issuer?.legalName ? t.issuer : null);
const ownerLine = (invoice: PrintInvoice, t: PrintTemplate) => { const name = t.outlet === 'franchise' ? issuerOf(invoice, t)?.legalName : ''; return name ? `Independently owned and operated by ${name.replace(/\.+$/, '')}.` : ''; };

const money = (cents: number, currency = 'CAD') => `${(cents / 100).toLocaleString('en-CA', { style: 'currency', currency })}`;
const day = (iso?: string | null) => (iso ? new Date(iso).toLocaleDateString('en-CA', { year: 'numeric', month: 'long', day: 'numeric' }) : '');

const ZONE = 'America/Toronto';
const serviceDay = (iso?: string | null) => (iso ? new Date(iso).toLocaleDateString('en-CA', { timeZone: ZONE, year: 'numeric', month: 'long', day: 'numeric' }) : '');
const serviceTime = (iso?: string | null) => (iso ? new Date(iso).toLocaleTimeString('en-CA', { timeZone: ZONE, hour: 'numeric', minute: '2-digit' }) : '');
const PAYMENT_WORDS: Record<string, string> = { cash: 'Cash', square_card: 'Card', square_terminal: 'Card', bank_transfer: 'Bank transfer', cheque: 'Cheque' };
const paymentWords = (methods?: string) => [...new Set((methods ?? '').split(',').map((m) => PAYMENT_WORDS[m]).filter(Boolean))].join(', ');

const css = (t: PrintTemplate) => `
.ddInv{font:13px/1.5 Arial,Helvetica,sans-serif;background:#fff;max-width:816px;margin:0 auto}
.ddInv *{box-sizing:border-box;background:transparent;border-radius:0;box-shadow:none;backdrop-filter:none}
.ddInv,.ddInv *{color:#000!important;opacity:1!important;text-shadow:none!important}
.ddInv p{margin:0;white-space:pre-line}
.ddInv .ddHead{display:flex;justify-content:space-between;gap:24px;padding:40px 40px 30px;border-bottom:1px solid #e5e5e5}
.ddInv .ddBrand{display:flex;gap:20px;align-items:flex-start}
.ddInv .ddBrand img{height:90px;width:auto;max-width:150px;object-fit:contain;display:block}
.ddInv .ddName{font-size:20px;line-height:1.25;font-weight:700;letter-spacing:-.5px;margin-bottom:10px}
.ddInv .ddIssuer p:not(.ddName){font-size:12px;line-height:1.8}
.ddInv .ddMeta{text-align:right;flex:none}
.ddInv h1{margin:0 0 15px;font-size:38px;line-height:1.1;font-weight:700;letter-spacing:-1px;text-transform:uppercase;color:${t.accentColor}!important}
.ddInv dl{display:grid;grid-template-columns:auto auto;gap:6px 8px;margin:0;justify-content:end;font-size:12px}
.ddInv dt{color:#888!important;text-align:right}
.ddInv dd{margin:0;text-align:right}
.ddInv .ddMeta dt{text-transform:uppercase;letter-spacing:1px}
.ddInv .ddClient{display:flex;justify-content:space-between;gap:24px;padding:30px 40px;background:#fafafa;font-size:12px}
.ddInv .ddClient dl{gap:6px 15px;align-self:flex-start}
.ddInv h2{margin:0 0 10px;font-size:12px;font-weight:400;text-transform:uppercase;letter-spacing:2px;color:#888!important}
.ddInv .ddClient p{line-height:1.7}
.ddInv .ddNote{padding:24px 40px 0}
.ddInv .ddBreak{padding:30px 40px 20px}
.ddInv .ddBreak h2{font-weight:700;margin-bottom:15px}
.ddInv .ddItem{display:flex;justify-content:space-between;gap:24px;padding:20px;background:#fafafa;border:1px solid #e5e5e5;font-size:12px}
.ddInv .ddItem+.ddItem{border-top:0}
.ddInv .ddItem span:last-child{flex:none;font-variant-numeric:tabular-nums}
.ddInv .ddSum{margin:10px 40px 30px auto;width:min(340px,calc(100% - 80px));padding:20px;background:#fafafa;border:1px solid #e5e5e5;font-size:12px}
.ddInv .ddSum div{display:flex;justify-content:space-between;padding:6px 0;font-variant-numeric:tabular-nums}
.ddInv .ddSum div span:first-child{color:#666!important}
.ddInv .ddSum .ddLast{border-bottom:1px solid #ddd}
.ddInv .ddSum .ddTot{padding:12px 0 0;font-size:16px;font-weight:700}
.ddInv .ddSum .ddTot span:first-child{color:#000!important;text-transform:uppercase;letter-spacing:1px}
.ddInv .ddBoxes{display:grid;grid-template-columns:1fr 1fr;gap:20px;padding:0 40px 30px}
.ddInv .ddBox{padding:18px;text-align:center}
.ddInv .ddBox small{display:block;font-size:9px;font-weight:700;text-transform:uppercase;letter-spacing:2px;margin-bottom:4px}
.ddInv .ddBox strong{font-size:20px;font-weight:700;font-variant-numeric:tabular-nums}
.ddInv .ddPaid{background:#dcfce7;border:1px solid #86efac}
.ddInv .ddPaid small{color:#16a34a!important}
.ddInv .ddPaid strong{color:#166534!important}
.ddInv .ddOwed{background:#fef2f2;border:1px solid #fca5a5}
.ddInv .ddOwed small{color:#dc2626!important}
.ddInv .ddOwed strong{color:#991b1b!important}
.ddInv .ddVoid{margin:0 40px 30px;padding:14px 18px;border:1px solid #fca5a5;background:#fef2f2;font-weight:700;text-align:center}
.ddInv .ddPay{padding:0 40px 24px;font-size:12px}
.ddInv .ddFoot{padding:30px 40px;border-top:1px solid #e5e5e5;text-align:center}
.ddInv .ddFoot b{display:block;font-size:14px;margin-bottom:8px}
.ddInv .ddFoot p{font-size:10px;line-height:1.6;color:#888!important}
@media(max-width:600px){.ddInv .ddHead,.ddInv .ddClient,.ddInv .ddBrand{flex-direction:column}.ddInv .ddMeta{text-align:left}.ddInv dl{justify-content:start}.ddInv dt,.ddInv dd{text-align:left}.ddInv .ddBoxes{grid-template-columns:1fr}.ddInv .ddSum{width:calc(100% - 80px);margin-left:40px}}
@media print{.ddInv *{-webkit-print-color-adjust:exact;print-color-adjust:exact}}
`;

/* The placeholders a custom layout can use. Every value is escaped, so invoice text can never become markup. */
export const TEMPLATE_PLACEHOLDERS: [string, string][] = [
  ['title', 'Document title'], ['invoice_number', 'Invoice number'], ['date', 'Issued (or created) date'], ['status', 'Status'],
  ['logo_url', 'Address of the logo image'], ['business_name', 'Your legal business name'], ['business_address', 'Your address'], ['tax_registration', 'Tax registration number'],
  ['customer_name', 'Customer name'], ['customer_email', 'Customer email'], ['customer_phone', 'Customer phone'],
  ['items', 'The item rows (<tr> with description, quantity, unit price and amount)'],
  ['subtotal', 'Subtotal'], ['discount', 'Discount (a coupon), 0.00 when none'], ['tax', 'Tax'], ['tax_label', 'The tax line\'s name with its rate, e.g. Tax (HST 13%)'], ['total', 'Total'], ['paid', 'Paid (invoices)'], ['balance_due', 'Balance due (invoices)'], ['currency', 'Currency'],
  ['header_note', 'Note above the items'], ['payment_instructions', 'Payment instructions (invoices)'], ['footer_note', 'Footer'],
  ['valid_until', 'Valid until (quotations)'], ['terms', 'Quotation terms'], ['note', 'The note written on this quotation'], ['ownership_note', 'Franchisee outlets: who independently owns them (empty otherwise)'],
];
export const STARTER_HTML = [
  '<div class="invoice">',
  '  <header>',
  '    <img src="{{logo_url}}" alt="" class="logo">',
  '    <div><h1>{{title}}</h1><p>{{invoice_number}} · {{date}}</p></div>',
  '  </header>',
  '  <section class="parties">',
  '    <div><strong>{{business_name}}</strong><p>{{business_address}}</p><p>{{tax_registration}}</p></div>',
  '    <div><strong>Bill to</strong><p>{{customer_name}}</p><p>{{customer_email}}</p><p>{{customer_phone}}</p></div>',
  '  </section>',
  '  <table>',
  '    <thead><tr><th>Item</th><th>Qty</th><th>Unit price</th><th>Amount</th></tr></thead>',
  '    <tbody>{{items}}</tbody>',
  '  </table>',
  '  <div class="totals">',
  '    <p>Subtotal <span>{{subtotal}}</span></p><p>{{tax_label}} <span>{{tax}}</span></p>',
  '    <p>Total <span>{{total}}</span></p><p class="due">Balance due <span>{{balance_due}}</span></p>',
  '  </div>',
  '  <footer>{{footer_note}}</footer>',
  '</div>',
].join('\n');
export const STARTER_CSS = [
  'body{font:14px/1.45 system-ui,sans-serif;color:#111;margin:0}',
  '.invoice{max-width:760px;margin:0 auto}',
  'header{display:flex;gap:16px;align-items:center;border-bottom:3px solid #fb1a2b;padding-bottom:12px}',
  '.logo{width:64px;height:64px;object-fit:contain}',
  'h1{margin:0;color:#fb1a2b}',
  '.parties{display:grid;grid-template-columns:1fr 1fr;gap:24px;margin:20px 0}',
  '.parties p{margin:2px 0;white-space:pre-line}',
  'table{width:100%;border-collapse:collapse}',
  'th,td{text-align:left;padding:8px 0;border-bottom:1px solid #ddd}',
  'th:not(:first-child),td:not(:first-child){text-align:right}',
  '.totals{margin:14px 0 0 auto;width:320px}',
  '.totals p{display:flex;justify-content:space-between;margin:4px 0}',
  '.due{font-weight:700;border-top:2px solid #fb1a2b;padding-top:6px}',
  'footer{margin-top:24px;color:#444;font-size:12px}',
].join('\n');

const esc = (v: string) => v.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);

/* The whole page of a custom layout with the invoice filled in. It is shown and printed in a sandboxed frame, where scripts never run. */
export function customInvoicePage(invoice: PrintInvoice, t: PrintTemplate): string {
  const customer = invoice.customerSnapshot ?? (invoice.customerName ? { name: invoice.customerName, email: '', phone: '' } : null);
  const m = (c: number) => esc(money(c, invoice.currency));
  const quote = invoice.kind === 'quotation';
  const issuer = issuerOf(invoice, t);
  const values: Record<string, string> = {
    title: esc(t.title), invoice_number: esc(invoice.invoiceNumber), date: esc(day(invoice.issuedAt ?? invoice.createdAt)), status: esc(statusWord(invoice.status)),
    logo_url: `${typeof window === 'undefined' ? '' : window.location.origin}${t.logoUrl || '/site/assets/logo.webp'}`,
    business_name: esc(issuer?.legalName ?? ''), business_address: esc(issuer?.address ?? ''), tax_registration: esc(issuer?.taxRegistration ?? ''),
    customer_name: esc(customer?.name ?? ''), customer_email: esc(customer?.email ?? ''), customer_phone: esc(customer?.phone ?? ''),
    items: (invoice.items ?? []).map((it) => `<tr><td>${esc(it.description)}</td><td>${it.quantity}</td><td>${m(it.unitPriceCents ?? Math.round(it.totalPriceCents / Math.max(1, it.quantity)))}</td><td>${m(it.totalPriceCents)}</td></tr>`).join(''),
    subtotal: m(invoice.subtotalCents), discount: m(invoice.discountCents ?? 0), tax: m(invoice.taxCents), tax_label: esc(taxLabel(invoice, t)), total: m(invoice.totalCents),
    paid: quote ? '' : m(invoice.paidCents), balance_due: quote ? '' : m(invoice.dueCents), currency: esc(invoice.currency),
    header_note: esc(t.headerNote), payment_instructions: quote ? '' : esc(t.paymentInstructions), footer_note: esc(t.footerNote),
    valid_until: esc(day(invoice.validUntil)), terms: quote ? esc(t.quotationNote ?? '') : '', note: esc(invoice.note ?? ''), ownership_note: esc(ownerLine(invoice, t)),
  };
  const body = t.customHtml.replace(/\{\{\s*([a-z_]+)\s*\}\}/g, (_all, key: string) => (key in values ? values[key] : ''));
  return `<!doctype html><html><head><meta charset="utf-8"><style>@page{size:${t.paperSize === 'a4' ? 'A4' : 'letter'};margin:0}@media print{html{padding:16mm}}\n${t.customCss.replace(/<\/style/gi, '')}</style></head><body>${body}</body></html>`;
}

// No allow-scripts, so nothing in a layout can run; same-origin only so this page can call print() on the frame.
const FRAME_SANDBOX = 'allow-same-origin allow-modals';

export function InvoiceDocument({ invoice, template }: { invoice: PrintInvoice; template: PrintTemplate }) {
  const t = template;
  if (t.customHtml?.trim()) return <iframe title="Invoice preview" sandbox={FRAME_SANDBOX} srcDoc={customInvoicePage(invoice, t)} style={{ width: '100%', minHeight: 900, border: 0, background: '#fff' }} />;
  const customer = invoice.customerSnapshot ?? (invoice.customerName ? { name: invoice.customerName, email: '', phone: '' } : null);
  const quote = invoice.kind === 'quotation';
  const issuer = issuerOf(invoice, t);
  const owner = ownerLine(invoice, t);
  const facts = ([['Service Date:', serviceDay(invoice.serviceAt)], ['Time:', serviceTime(invoice.serviceAt)], ['Payment:', paymentWords(invoice.paymentMethods)], ['Coupon:', invoice.couponCode ?? '']] as [string, string][]).filter(([, v]) => v);
  return (
    <div className="ddInv">
      <style>{css(t)}</style>
      <div className="ddHead">
        <div className="ddBrand">
          {t.showLogo && <img src={t.logoUrl || '/site/assets/logo.webp'} alt="" />}
          {issuer && <div className="ddIssuer">
            <p className="ddName">{issuer.legalName}</p>
            {issuer.address && <p>{issuer.address}</p>}
            {t.businessDetails && <p>{t.businessDetails}</p>}
            {t.showTaxRegistration && issuer.taxRegistration && <p>HST: {issuer.taxRegistration}</p>}
          </div>}
        </div>
        <div className="ddMeta">
          <h1>{t.title}</h1>
          <dl>
            <dt>{quote ? 'Quote #' : 'Invoice #'}</dt><dd>{invoice.invoiceNumber}</dd>
            {(invoice.issuedAt || invoice.createdAt) && <><dt>Date</dt><dd>{day(invoice.issuedAt ?? invoice.createdAt)}</dd></>}
            {quote && invoice.validUntil && <><dt>Valid until</dt><dd>{day(invoice.validUntil)}</dd></>}
            {!quote && invoice.status === 'draft' && <><dt>Status</dt><dd>{statusWord(invoice.status)}</dd></>}
          </dl>
        </div>
      </div>
      <div className="ddClient">
        <div>
          {customer && <>
            <h2>{t.billToLabel}</h2>
            <p>Name: {customer.name}</p>
            {t.showCustomerContact && customer.email && <p>Email: {customer.email}</p>}
            {t.showCustomerContact && customer.phone && <p>Phone Number: {customer.phone}</p>}
          </>}
        </div>
        {facts.length > 0 && <dl>{facts.map(([k, v]) => <div key={k} style={{ display: 'contents' }}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>}
      </div>
      {t.headerNote && <p className="ddNote">{t.headerNote}</p>}
      <div className="ddBreak">
        <h2>Service Breakdown</h2>
        {(invoice.items ?? []).map((it, n) => (
          <div className="ddItem" key={n}>
            <span>{it.description}{it.quantity > 1 ? ` × ${it.quantity}` : ''}</span>
            <span>{money(it.totalPriceCents, invoice.currency)}</span>
          </div>
        ))}
      </div>
      <div className="ddSum">
        <div><span>Subtotal</span><span>{money(invoice.subtotalCents, invoice.currency)}</span></div>
        <div><span>Discount</span><span>- {money(invoice.discountCents ?? 0, invoice.currency)}</span></div>
        <div className="ddLast"><span>{taxLabel(invoice, t)}</span><span>{money(invoice.taxCents, invoice.currency)}</span></div>
        <div className="ddTot"><span>{quote ? 'Quoted Total' : 'Total Due'}</span><span>{money(invoice.totalCents, invoice.currency)}</span></div>
      </div>
      {!quote && invoice.status === 'void' && <p className="ddVoid">This invoice was voided, so nothing is owed on it.</p>}
      {!quote && invoice.status !== 'void' && <div className="ddBoxes">
        <div className="ddBox ddPaid"><small>Paid Amount</small><strong>{money(invoice.paidCents, invoice.currency)}</strong></div>
        <div className="ddBox ddOwed"><small>Outstanding Balance</small><strong>{money(invoice.dueCents, invoice.currency)}</strong></div>
      </div>}
      {quote && invoice.note && <div className="ddPay"><h2>Note</h2><p>{invoice.note}</p></div>}
      {quote && t.quotationNote && <div className="ddPay"><h2>Terms</h2><p>{t.quotationNote}</p></div>}
      {!quote && t.paymentInstructions && <div className="ddPay"><h2>Payment</h2><p>{t.paymentInstructions}</p></div>}
      <div className="ddFoot">
        {t.footerNote && <b>{t.footerNote}</b>}
        <p>This is a computer generated {quote ? 'quotation' : 'invoice'}. Signature is not required.</p>
        {owner && <p>{owner}</p>}
      </div>
    </div>
  );
}

// shortcut: @page margin 0 is what stops the browser printing its date, title, address and page number; a second printed page has no top margin, so give the sheet repeating spacers if long invoices become common.
/* Prints through a temporary layer so the app chrome never appears on paper: everything else in <body> is hidden while printing. */
export function printInvoiceDocument(invoice: PrintInvoice, template: PrintTemplate) {
  if (template.customHtml?.trim()) {
    const frame = document.createElement('iframe');
    frame.setAttribute('sandbox', FRAME_SANDBOX);
    frame.setAttribute('aria-hidden', 'true');
    frame.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0';
    frame.srcdoc = customInvoicePage(invoice, template);
    frame.onload = () => {
      const w = frame.contentWindow;
      if (!w) { frame.remove(); return; }
      w.addEventListener('afterprint', () => frame.remove());
      setTimeout(() => w.print(), 150);
    };
    document.body.appendChild(frame);
    return;
  }
  const host = document.createElement('div');
  host.className = 'dd-invoice-print-root';
  const style = document.createElement('style');
  style.textContent = `@page{size:${template.paperSize === 'a4' ? 'A4' : 'letter'};margin:0}
.dd-invoice-print-root{display:none}
@media print{body>*:not(.dd-invoice-print-root){display:none!important}.dd-invoice-print-root{display:block!important;background:#fff}html,body{background:#fff!important}}`;
  document.head.appendChild(style);
  document.body.appendChild(host);
  const root = createRoot(host);
  flushSync(() => root.render(<InvoiceDocument invoice={invoice} template={template} />));
  const cleanup = () => { window.removeEventListener('afterprint', cleanup); root.unmount(); host.remove(); style.remove(); };
  window.addEventListener('afterprint', cleanup);
  const logo = host.querySelector('img');
  const go = () => setTimeout(() => { window.print(); }, 50);
  if (logo && !logo.complete) { logo.addEventListener('load', go, { once: true }); logo.addEventListener('error', go, { once: true }); } else go();
}

/* The real PDF, drawn by the server and opened in a new tab to view, save or print. Call it straight from the click so the browser allows the
   new tab. A custom HTML layout cannot be drawn as a PDF, so those keep printing through the browser (ownLayout). */
export const openPdf = (url: string) => { window.open(url, '_blank', 'noopener'); };
export const ownLayout = (t: PrintTemplate) => !!t.customHtml?.trim();

/* The invoice as its own page in a new browser window, with a Print or save as PDF button. Open the window straight away from the
   click (browsers block windows opened after a wait), then fill it once the invoice has loaded. */
export function openInvoiceWindow(): Window | null {
  const w = window.open('', '_blank');
  if (!w) return null;
  w.document.open();
  w.document.write('<!doctype html><html><head><meta charset="utf-8"><title>Invoice</title></head><body style="font:16px system-ui;padding:32px">Loading the invoice…</body></html>');
  w.document.close();
  return w;
}
export function fillInvoiceWindow(w: Window, invoice: PrintInvoice, template: PrintTemplate) {
  const toolbar = '<div class="ddBar"><button onclick="window.print()">Print or save as PDF</button><button class="ddGhost" onclick="window.close()">Close</button></div>';
  const barCss = '.ddBar{position:sticky;top:0;z-index:5;display:flex;gap:10px;justify-content:center;padding:12px;background:#17171a}.ddBar button{font:600 14px system-ui;padding:9px 18px;border-radius:999px;border:0;background:#d22b2b;color:#fff;cursor:pointer}.ddBar .ddGhost{background:#3a3a40}@media print{.ddBar{display:none}body{background:#fff!important;padding:0!important}.ddSheet{box-shadow:none!important;margin:0!important;padding:0!important}}';
  let html: string;
  if (template.customHtml?.trim()) {
    html = customInvoicePage(invoice, template).replace('</style>', barCss + '</style>').replace('<body>', '<body>' + toolbar);
  } else {
    const host = document.createElement('div');
    const root = createRoot(host);
    flushSync(() => root.render(<InvoiceDocument invoice={invoice} template={template} />));
    const inner = host.innerHTML;
    root.unmount();
    html = '<!doctype html><html><head><meta charset="utf-8"><title>Invoice ' + esc(invoice.invoiceNumber) + '</title><style>@page{size:' + (template.paperSize === 'a4' ? 'A4' : 'letter') + ';margin:0}body{margin:0;background:#e9e9ec;font-family:system-ui,sans-serif}.ddSheet{max-width:820px;margin:24px auto;background:#fff;box-shadow:0 6px 30px rgba(0,0,0,.18)}' + barCss + '</style></head><body>' + toolbar + '<div class="ddSheet">' + inner + '</div></body></html>';
  }
  w.document.open();
  w.document.write(html);
  w.document.close();
  w.document.title = template.title + ' ' + invoice.invoiceNumber;
}

/* One read of a template; the built-in layout if it cannot be read. */
export function loadPrintTemplate(url: string): Promise<PrintTemplate> {
  return fetch(url, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null))
    .then((b) => (b?.template ? { ...DEFAULT_PRINT_TEMPLATE, ...b.template, logoUrl: b.logoUrl || undefined, issuer: b.issuer ?? null, taxName: b.taxName || undefined, taxRatePercent: b.taxRatePercent || undefined, outlet: b.outlet || undefined } : DEFAULT_PRINT_TEMPLATE))
    .catch(() => DEFAULT_PRINT_TEMPLATE);
}

export function usePrintTemplate(url: string): PrintTemplate {
  const [template, setTemplate] = useState<PrintTemplate>(DEFAULT_PRINT_TEMPLATE);
  useEffect(() => {
    let live = true;
    loadPrintTemplate(url).then((t) => { if (live) setTemplate(t); });
    return () => { live = false; };
  }, [url]);
  return template;
}
