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
};
export type PrintInvoice = {
  /** 'quotation' prints a quotation: no paid or pending lines, a valid-until date and the quotation terms. */
  kind?: 'invoice' | 'quotation'; validUntil?: string | null; note?: string;
  invoiceNumber: string; status: string; currency: string; issuedAt?: string | null; createdAt?: string;
  subtotalCents: number; taxCents: number; totalCents: number; paidCents: number; dueCents: number; taxRatePercent?: number;
  discountCents?: number; couponCode?: string;
  customerName?: string;
  issuerSnapshot?: { legalName: string; address: string; taxRegistration: string } | null;
  customerSnapshot?: { name: string; email: string; phone: string } | null;
  items?: { description: string; quantity: number; unitPriceCents?: number; totalPriceCents: number }[];
};

export const DEFAULT_PRINT_TEMPLATE: PrintTemplate = {
  title: 'Invoice', accentColor: '#e4202e', showLogo: true, headerNote: '', billToLabel: 'Bill to', itemLabel: 'Item', quantityLabel: 'Qty',
  unitPriceLabel: 'Unit price', amountLabel: 'Amount', showTaxRegistration: true, showCustomerContact: true, paymentInstructions: '', footerNote: 'Thank you for choosing Dapper Detailing Pros.', paperSize: 'letter',
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
  return t.taxName ? `Tax (${t.taxName} ${pct})` : `Tax (${pct})`;
}
const issuerOf = (invoice: PrintInvoice, t: PrintTemplate) => (invoice.issuerSnapshot?.legalName ? invoice.issuerSnapshot : t.issuer?.legalName ? t.issuer : null);
const ownerLine = (invoice: PrintInvoice, t: PrintTemplate) => { const name = t.outlet === 'franchise' ? issuerOf(invoice, t)?.legalName : ''; return name ? `Independently owned and operated by ${name.replace(/\.+$/, '')}.` : ''; };

const money = (cents: number, currency = 'CAD') => `${(cents / 100).toLocaleString('en-CA', { style: 'currency', currency })}`;
const day = (iso?: string | null) => (iso ? new Date(iso).toLocaleDateString('en-CA', { year: 'numeric', month: 'long', day: 'numeric' }) : '');

const css = (t: PrintTemplate) => `
.ddInv{font:14px/1.5 'Manrope',system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;color:#17171a;background:#fff;padding:0;max-width:780px;margin:0 auto}
.ddInv *{box-sizing:border-box;background:transparent;border-radius:0;box-shadow:none;backdrop-filter:none}
.ddInv,.ddInv *{color:#17171a!important;opacity:1!important;text-shadow:none!important}
.ddInv .ddHead{display:flex;justify-content:space-between;align-items:center;gap:28px;padding:20px 24px;background:#0b0b0c;border-bottom:6px solid ${t.accentColor}}
.ddInv .ddHead,.ddInv .ddHead *{color:#fff!important}
.ddInv .ddBrand{display:flex;gap:16px;align-items:center}
.ddInv .ddBrand img{width:72px;height:72px;object-fit:contain;background:#fff;padding:5px}
.ddInv .ddIssuer p{margin:0}
.ddInv .ddIssuer .ddName{font-size:18px;line-height:1.25;font-weight:800}
.ddInv h1{margin:0;font-size:34px;line-height:1;font-weight:800;letter-spacing:.04em;text-transform:uppercase}
.ddInv .ddMeta{text-align:right}
.ddInv .ddMeta dl{display:grid;grid-template-columns:auto auto;gap:3px 14px;margin:10px 0 0;justify-content:end;font-size:13px}
.ddInv .ddMeta dt{color:#b8b5ae!important;text-align:right}
.ddInv .ddMeta dd{margin:0;font-weight:700;text-align:right}
.ddInv .ddStatus{display:inline-block;margin-top:10px;padding:3px 12px;border:1.5px solid #fff;border-radius:999px;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.08em}
.ddInv .ddStatus.paid{background:#fff;color:#0b0b0c!important}
.ddInv .ddParties{display:grid;grid-template-columns:1fr 1fr;gap:24px;margin:14px 0 18px;padding:14px 0;border-bottom:1px solid #e4e4e8}
.ddInv h2{margin:0 0 5px;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.1em;color:#6b6b74!important}
.ddInv p{margin:0 0 2px;white-space:pre-line}
.ddInv .ddNote{margin:0 0 14px;color:#33333a!important}
.ddInv .ddBanner{display:flex;justify-content:space-between;align-items:center;gap:16px;margin:0 0 18px;padding:14px 18px;border:2px solid ${t.accentColor}}
.ddInv .ddBanner b{font-size:12px;letter-spacing:.1em;text-transform:uppercase}
.ddInv .ddBanner strong{font-size:24px;font-weight:800;font-variant-numeric:tabular-nums}
.ddInv .ddBanner.paid{border-color:#17171a;background:#f1f1f3}
.ddInv .ddBanner small{display:block;font-size:12px;color:#4a4a52!important}
.ddInv table{width:100%;border-collapse:collapse;margin-top:4px}
.ddInv th{text-align:left;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.08em;background:#0b0b0c;color:#fff!important;padding:9px 10px;border:0}
.ddInv th.r{text-align:right}
.ddInv td{padding:11px 10px;border-bottom:1px solid #e4e4e8;vertical-align:top}
.ddInv tbody tr:nth-child(even) td{background:#f7f7f8}
.ddInv .r{text-align:right;font-variant-numeric:tabular-nums}
.ddInv .ddTotals{margin:16px 0 0 auto;width:min(340px,100%)}
.ddInv .ddTotals div{display:flex;justify-content:space-between;padding:4px 10px;font-variant-numeric:tabular-nums}
.ddInv .ddTotals .ddTot{border-top:1px solid #17171a;margin-top:4px;padding-top:8px;font-weight:800}
.ddInv .ddDue{background:#17171a;margin-top:8px;padding:10px!important;font-size:17px;font-weight:800}
.ddInv .ddDue,.ddInv .ddDue *{color:#fff!important}
.ddInv .ddPay,.ddInv .ddFoot{margin-top:22px;padding-top:12px;border-top:1px solid #e4e4e8}
.ddInv .ddFoot,.ddInv .ddFoot *{color:#4a4a52!important}
.ddInv .ddFoot{font-size:12.5px}
@media print{.ddInv .ddHead,.ddInv .ddBrand img,.ddInv .ddStatus.paid,.ddInv th,.ddInv .ddDue,.ddInv tbody tr:nth-child(even) td,.ddInv .ddBanner.paid{-webkit-print-color-adjust:exact;print-color-adjust:exact}}
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
  return `<!doctype html><html><head><meta charset="utf-8"><style>@page{size:${t.paperSize === 'a4' ? 'A4' : 'letter'};margin:16mm}\n${t.customCss.replace(/<\/style/gi, '')}</style></head><body>${body}</body></html>`;
}

// No allow-scripts, so nothing in a layout can run; same-origin only so this page can call print() on the frame.
const FRAME_SANDBOX = 'allow-same-origin allow-modals';

export function InvoiceDocument({ invoice, template }: { invoice: PrintInvoice; template: PrintTemplate }) {
  const t = template;
  if (t.customHtml?.trim()) return <iframe title="Invoice preview" sandbox={FRAME_SANDBOX} srcDoc={customInvoicePage(invoice, t)} style={{ width: '100%', minHeight: 900, border: 0, background: '#fff' }} />;
  const customer = invoice.customerSnapshot ?? (invoice.customerName ? { name: invoice.customerName, email: '', phone: '' } : null);
  const quote = invoice.kind === 'quotation';
  const paidInFull = !quote && invoice.totalCents > 0 && invoice.dueCents <= 0;
  const issuer = issuerOf(invoice, t);
  const owner = ownerLine(invoice, t);
  return (
    <div className="ddInv">
      <style>{css(t)}</style>
      <div className="ddHead">
        <div className="ddBrand">
          {t.showLogo && <img src={t.logoUrl || '/site/assets/logo.webp'} alt="" />}
          <div className="ddIssuer">
            {issuer && <p className="ddName">{issuer.legalName}</p>}
          </div>
        </div>
        <div className="ddMeta">
          <h1>{t.title}</h1>
          <dl>
            <dt>Number</dt><dd>{invoice.invoiceNumber}</dd>
            <dt>Currency</dt><dd>{invoice.currency}</dd>
            {(invoice.issuedAt || invoice.createdAt) && <><dt>{invoice.issuedAt ? 'Issued' : 'Created'}</dt><dd>{day(invoice.issuedAt ?? invoice.createdAt)}</dd></>}
            {quote && invoice.validUntil && <><dt>Valid until</dt><dd>{day(invoice.validUntil)}</dd></>}
          </dl>
          {!quote && <span className={'ddStatus' + (paidInFull ? ' paid' : '')}>{paidInFull ? 'Paid' : statusWord(invoice.status)}</span>}
        </div>
      </div>
      <div className="ddParties">
        <div>
          {customer && <>
            <h2>{t.billToLabel}</h2>
            <p><strong>{customer.name}</strong></p>
            {t.showCustomerContact && customer.email && <p>{customer.email}</p>}
            {t.showCustomerContact && customer.phone && <p>{customer.phone}</p>}
          </>}
        </div>
        <div>
          {issuer && <>
            <h2>From</h2>
            <p><strong>{issuer.legalName}</strong></p>
            {issuer.address && <p>{issuer.address}</p>}
            {t.showTaxRegistration && issuer.taxRegistration && <p>GST/HST: {issuer.taxRegistration}</p>}
          </>}
        </div>
      </div>
      {!quote && invoice.status === 'void' && <div className="ddBanner">
        <div><b>Void</b><small>This invoice was voided, so nothing is owed on it.</small></div>
        <strong>{money(0, invoice.currency)}</strong>
      </div>}
      {!quote && invoice.status !== 'void' && <div className={'ddBanner' + (paidInFull ? ' paid' : '')}>
        <div><b>{paidInFull ? 'Paid in full' : 'Balance due'}</b><small>{paidInFull ? 'Thank you. Nothing further is owed.' : invoice.paidCents > 0 ? `${money(invoice.paidCents, invoice.currency)} received so far.` : 'Nothing has been paid yet.'}</small></div>
        <strong>{paidInFull ? money(invoice.paidCents, invoice.currency) : money(invoice.dueCents, invoice.currency)}</strong>
      </div>}
      {t.headerNote && <p className="ddNote">{t.headerNote}</p>}
      <table>
        <thead><tr><th>{t.itemLabel}</th><th className="r">{t.quantityLabel}</th><th className="r">{t.unitPriceLabel}</th><th className="r">{t.amountLabel}</th></tr></thead>
        <tbody>
          {(invoice.items ?? []).map((it, n) => (
            <tr key={n}>
              <td>{it.description}</td>
              <td className="r">{it.quantity}</td>
              <td className="r">{money(it.unitPriceCents ?? Math.round(it.totalPriceCents / Math.max(1, it.quantity)), invoice.currency)}</td>
              <td className="r">{money(it.totalPriceCents, invoice.currency)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="ddTotals">
        <div><span>Subtotal</span><span>{money(invoice.subtotalCents, invoice.currency)}</span></div>
        {(invoice.discountCents ?? 0) > 0 && <div><span>Discount{invoice.couponCode ? ` (${invoice.couponCode})` : ''}</span><span>−{money(invoice.discountCents ?? 0, invoice.currency)}</span></div>}
        <div><span>{taxLabel(invoice, t)}</span><span>{money(invoice.taxCents, invoice.currency)}</span></div>
        <div className="ddTot"><span>Total</span><span>{money(invoice.totalCents, invoice.currency)}</span></div>
        {!quote && invoice.paidCents > 0 && <div><span>Paid</span><span>−{money(invoice.paidCents, invoice.currency)}</span></div>}
        {!quote && !paidInFull && invoice.status !== 'void' && <div className="ddDue"><span>Balance due</span><span>{money(invoice.dueCents, invoice.currency)}</span></div>}
        {quote && <div className="ddDue"><span>Quoted total</span><span>{money(invoice.totalCents, invoice.currency)}</span></div>}
      </div>
      {quote && invoice.note && <div className="ddPay"><h2>Note</h2><p>{invoice.note}</p></div>}
      {quote && t.quotationNote && <div className="ddPay"><h2>Terms</h2><p>{t.quotationNote}</p></div>}
      {!quote && t.paymentInstructions && <div className="ddPay"><h2>Payment</h2><p>{t.paymentInstructions}</p></div>}
      {(t.footerNote || owner) && <div className="ddFoot">{t.footerNote && <p>{t.footerNote}</p>}{owner && <p>{owner}</p>}</div>}
    </div>
  );
}

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
  style.textContent = `@page{size:${template.paperSize === 'a4' ? 'A4' : 'letter'};margin:16mm}
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
    html = '<!doctype html><html><head><meta charset="utf-8"><title>Invoice ' + esc(invoice.invoiceNumber) + '</title><style>@page{size:' + (template.paperSize === 'a4' ? 'A4' : 'letter') + ';margin:14mm}body{margin:0;background:#e9e9ec;font-family:system-ui,sans-serif}.ddSheet{max-width:820px;margin:24px auto;background:#fff;padding:32px;box-shadow:0 6px 30px rgba(0,0,0,.18)}' + barCss + '</style></head><body>' + toolbar + '<div class="ddSheet">' + inner + '</div></body></html>';
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
