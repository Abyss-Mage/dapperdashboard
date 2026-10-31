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
};
export type PrintInvoice = {
  invoiceNumber: string; status: string; currency: string; issuedAt?: string | null; createdAt?: string;
  subtotalCents: number; taxCents: number; totalCents: number; paidCents: number; dueCents: number;
  customerName?: string;
  issuerSnapshot?: { legalName: string; address: string; taxRegistration: string } | null;
  customerSnapshot?: { name: string; email: string; phone: string } | null;
  items?: { description: string; quantity: number; unitPriceCents?: number; totalPriceCents: number }[];
};

export const DEFAULT_PRINT_TEMPLATE: PrintTemplate = {
  title: 'Invoice', accentColor: '#d22b2b', showLogo: true, headerNote: '', billToLabel: 'Bill to', itemLabel: 'Item', quantityLabel: 'Qty',
  unitPriceLabel: 'Unit price', amountLabel: 'Amount', showTaxRegistration: true, showCustomerContact: true, paymentInstructions: '', footerNote: '', paperSize: 'letter',
  customHtml: '', customCss: '',
};

const money = (cents: number, currency = 'CAD') => `${(cents / 100).toLocaleString('en-CA', { style: 'currency', currency })}`;
const day = (iso?: string | null) => (iso ? new Date(iso).toLocaleDateString('en-CA', { year: 'numeric', month: 'long', day: 'numeric' }) : '');

const css = (t: PrintTemplate) => `
.ddInv{font:14px/1.5 'Manrope',system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;color:#17171a;background:#fff;padding:0;max-width:780px;margin:0 auto;border-top:8px solid ${t.accentColor}}
.ddInv *{box-sizing:border-box;background:transparent;border-radius:0;box-shadow:none;backdrop-filter:none}
.ddInv,.ddInv *{color:#17171a!important;opacity:1!important;text-shadow:none!important}
.ddInv .ddHead{display:flex;justify-content:space-between;align-items:flex-start;gap:28px;padding:22px 0 18px}
.ddInv .ddBrand{display:flex;gap:14px;align-items:flex-start}
.ddInv .ddBrand img{width:72px;height:72px;object-fit:contain}
.ddInv .ddIssuer p{font-size:13px;color:#4a4a52!important;margin:0}
.ddInv .ddIssuer .ddName{font-size:17px;font-weight:800;color:#17171a!important}
.ddInv h1{margin:0;font-size:34px;line-height:1;font-weight:800;letter-spacing:.04em;text-transform:uppercase;color:${t.accentColor}!important}
.ddInv .ddMeta{text-align:right}
.ddInv .ddMeta dl{display:grid;grid-template-columns:auto auto;gap:3px 14px;margin:10px 0 0;justify-content:end;font-size:13px}
.ddInv .ddMeta dt{color:#6b6b74!important;text-align:right}
.ddInv .ddMeta dd{margin:0;font-weight:700;text-align:right}
.ddInv .ddStatus{display:inline-block;margin-top:10px;padding:3px 12px;border:1.5px solid #17171a;border-radius:999px;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.08em}
.ddInv .ddStatus.paid{border-color:#17171a;background:#17171a;color:#fff!important}
.ddInv .ddParties{display:grid;grid-template-columns:1fr 1fr;gap:24px;margin:6px 0 18px;padding:14px 0;border-top:1px solid #e4e4e8;border-bottom:1px solid #e4e4e8}
.ddInv h2{margin:0 0 5px;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.1em;color:#6b6b74!important}
.ddInv p{margin:0 0 2px;white-space:pre-line}
.ddInv .ddNote{margin:0 0 14px;color:#33333a!important}
.ddInv .ddBanner{display:flex;justify-content:space-between;align-items:center;gap:16px;margin:0 0 18px;padding:14px 18px;border:2px solid ${t.accentColor}}
.ddInv .ddBanner b{font-size:12px;letter-spacing:.1em;text-transform:uppercase}
.ddInv .ddBanner strong{font-size:24px;font-weight:800;font-variant-numeric:tabular-nums}
.ddInv .ddBanner.paid{border-color:#17171a;background:#f1f1f3}
.ddInv .ddBanner small{display:block;font-size:12px;color:#4a4a52!important}
.ddInv table{width:100%;border-collapse:collapse;margin-top:4px}
.ddInv th{text-align:left;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.08em;background:#17171a;color:#fff!important;padding:9px 10px}
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
@media print{.ddInv .ddStatus.paid,.ddInv th,.ddInv .ddDue,.ddInv tbody tr:nth-child(even) td,.ddInv .ddBanner.paid{-webkit-print-color-adjust:exact;print-color-adjust:exact}}
`;

/* The placeholders a custom layout can use. Every value is escaped, so invoice text can never become markup. */
export const TEMPLATE_PLACEHOLDERS: [string, string][] = [
  ['title', 'Document title'], ['invoice_number', 'Invoice number'], ['date', 'Issued (or created) date'], ['status', 'Status'],
  ['logo_url', 'Address of the logo image'], ['business_name', 'Your legal business name'], ['business_address', 'Your address'], ['tax_registration', 'Tax registration number'],
  ['customer_name', 'Customer name'], ['customer_email', 'Customer email'], ['customer_phone', 'Customer phone'],
  ['items', 'The item rows (<tr> with description, quantity, unit price and amount)'],
  ['subtotal', 'Subtotal'], ['tax', 'Tax'], ['total', 'Total'], ['paid', 'Paid'], ['balance_due', 'Balance due'], ['currency', 'Currency'],
  ['header_note', 'Note above the items'], ['payment_instructions', 'Payment instructions'], ['footer_note', 'Footer'],
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
  '    <p>Subtotal <span>{{subtotal}}</span></p><p>Tax <span>{{tax}}</span></p>',
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
  const values: Record<string, string> = {
    title: esc(t.title), invoice_number: esc(invoice.invoiceNumber), date: esc(day(invoice.issuedAt ?? invoice.createdAt)), status: esc(invoice.status.replaceAll('_', ' ')),
    logo_url: `${typeof window === 'undefined' ? '' : window.location.origin}/site/assets/logo.webp`,
    business_name: esc(invoice.issuerSnapshot?.legalName ?? ''), business_address: esc(invoice.issuerSnapshot?.address ?? ''), tax_registration: esc(invoice.issuerSnapshot?.taxRegistration ?? ''),
    customer_name: esc(customer?.name ?? ''), customer_email: esc(customer?.email ?? ''), customer_phone: esc(customer?.phone ?? ''),
    items: (invoice.items ?? []).map((it) => `<tr><td>${esc(it.description)}</td><td>${it.quantity}</td><td>${m(it.unitPriceCents ?? Math.round(it.totalPriceCents / Math.max(1, it.quantity)))}</td><td>${m(it.totalPriceCents)}</td></tr>`).join(''),
    subtotal: m(invoice.subtotalCents), tax: m(invoice.taxCents), total: m(invoice.totalCents), paid: m(invoice.paidCents), balance_due: m(invoice.dueCents), currency: esc(invoice.currency),
    header_note: esc(t.headerNote), payment_instructions: esc(t.paymentInstructions), footer_note: esc(t.footerNote),
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
  const paidInFull = invoice.totalCents > 0 && invoice.dueCents <= 0;
  return (
    <div className="ddInv">
      <style>{css(t)}</style>
      <div className="ddHead">
        <div className="ddBrand">
          {t.showLogo && <img src="/site/assets/logo.webp" alt="" />}
          <div className="ddIssuer">
            {invoice.issuerSnapshot && <>
              <p className="ddName">{invoice.issuerSnapshot.legalName}</p>
              <p>{invoice.issuerSnapshot.address}</p>
              {t.showTaxRegistration && invoice.issuerSnapshot.taxRegistration && <p>Tax registration: {invoice.issuerSnapshot.taxRegistration}</p>}
            </>}
          </div>
        </div>
        <div className="ddMeta">
          <h1>{t.title}</h1>
          <dl>
            <dt>Number</dt><dd>{invoice.invoiceNumber}</dd>
            {(invoice.issuedAt || invoice.createdAt) && <><dt>{invoice.issuedAt ? 'Issued' : 'Created'}</dt><dd>{day(invoice.issuedAt ?? invoice.createdAt)}</dd></>}
          </dl>
          <span className={'ddStatus' + (paidInFull ? ' paid' : '')}>{paidInFull ? 'Paid' : invoice.status.replaceAll('_', ' ')}</span>
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
        <div className="r">
          <h2>Currency</h2>
          <p>{invoice.currency}</p>
        </div>
      </div>
      <div className={'ddBanner' + (paidInFull ? ' paid' : '')}>
        <div><b>{paidInFull ? 'Paid in full' : 'Dues pending'}</b>{paidInFull && <small>Thank you. Nothing further is owed.</small>}</div>
        <strong>{paidInFull ? money(invoice.paidCents, invoice.currency) : money(invoice.dueCents, invoice.currency)}</strong>
      </div>
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
        <div><span>Tax</span><span>{money(invoice.taxCents, invoice.currency)}</span></div>
        <div className="ddTot"><span>Total</span><span>{money(invoice.totalCents, invoice.currency)}</span></div>
        <div><span>Paid</span><span>{money(invoice.paidCents, invoice.currency)}</span></div>
        <div className="ddDue"><span>Pending</span><span>{money(invoice.dueCents, invoice.currency)}</span></div>
      </div>
      {t.paymentInstructions && <div className="ddPay"><h2>Payment</h2><p>{t.paymentInstructions}</p></div>}
      {t.footerNote && <div className="ddFoot"><p>{t.footerNote}</p></div>}
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

/* Loads the organization's template once; falls back to the built-in layout if it cannot be read. */
export function usePrintTemplate(url: string): PrintTemplate {
  const [template, setTemplate] = useState<PrintTemplate>(DEFAULT_PRINT_TEMPLATE);
  useEffect(() => {
    let live = true;
    fetch(url, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null)).then((b) => { if (live && b?.template) setTemplate({ ...DEFAULT_PRINT_TEMPLATE, ...b.template }); }).catch(() => undefined);
    return () => { live = false; };
  }, [url]);
  return template;
}
