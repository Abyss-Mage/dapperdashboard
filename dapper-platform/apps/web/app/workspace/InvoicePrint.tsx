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
  title: 'Invoice', accentColor: '#111111', showLogo: true, headerNote: '', billToLabel: 'Bill to', itemLabel: 'Item', quantityLabel: 'Qty',
  unitPriceLabel: 'Unit price', amountLabel: 'Amount', showTaxRegistration: true, showCustomerContact: true, paymentInstructions: '', footerNote: '', paperSize: 'letter',
};

const money = (cents: number, currency = 'CAD') => `${(cents / 100).toLocaleString('en-CA', { style: 'currency', currency })}`;
const day = (iso?: string | null) => (iso ? new Date(iso).toLocaleDateString('en-CA', { year: 'numeric', month: 'long', day: 'numeric' }) : '');

const css = (t: PrintTemplate) => `
.ddInv{font:14px/1.45 system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;color:#111;background:#fff;padding:0;max-width:760px;margin:0 auto}
.ddInv *{box-sizing:border-box;background:transparent;border-radius:0;box-shadow:none;backdrop-filter:none}
.ddInv,.ddInv *{color:#111!important;opacity:1!important;text-shadow:none!important}
.ddInv .ddStatus{border-radius:999px}
.ddInv .ddHead{display:flex;justify-content:space-between;align-items:flex-start;gap:24px;padding-bottom:16px;border-bottom:3px solid ${t.accentColor}}
.ddInv .ddBrand{display:flex;gap:14px;align-items:flex-start}
.ddInv .ddBrand img{width:64px;height:64px;object-fit:contain}
.ddInv h1{margin:0 0 2px;font-size:28px;letter-spacing:-.01em;color:${t.accentColor}!important}
.ddInv .ddNum{font-size:16px;font-weight:600}
.ddInv .ddMeta{text-align:right;font-size:13px}
.ddInv .ddStatus{display:inline-block;margin-top:6px;padding:2px 10px;border:1px solid ${t.accentColor};border-radius:999px;font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:${t.accentColor}!important}
.ddInv .ddParties{display:grid;grid-template-columns:1fr 1fr;gap:24px;margin:20px 0}
.ddInv h2{margin:0 0 4px;font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:#555!important}
.ddInv p{margin:0 0 2px;white-space:pre-line}
.ddInv .ddNote{margin:0 0 16px;color:#333}
.ddInv table{width:100%;border-collapse:collapse;margin-top:8px}
.ddInv th{text-align:left;font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:#555!important;border-bottom:1px solid #999;padding:6px 0}
.ddInv td{padding:8px 0;border-bottom:1px solid #ddd;vertical-align:top}
.ddInv .r{text-align:right;font-variant-numeric:tabular-nums}
.ddInv .ddTotals{margin:14px 0 0 auto;width:min(320px,100%)}
.ddInv .ddTotals div{display:flex;justify-content:space-between;padding:3px 0;font-variant-numeric:tabular-nums}
.ddInv .ddDue{border-top:2px solid ${t.accentColor};margin-top:6px;padding-top:8px!important;font-size:17px;font-weight:700}
.ddInv .ddPay,.ddInv .ddFoot{margin-top:22px;padding-top:12px;border-top:1px solid #ddd}
.ddInv .ddFoot,.ddInv .ddFoot *{color:#444!important}
.ddInv .ddFoot{font-size:12.5px}
`;

export function InvoiceDocument({ invoice, template }: { invoice: PrintInvoice; template: PrintTemplate }) {
  const t = template;
  const customer = invoice.customerSnapshot ?? (invoice.customerName ? { name: invoice.customerName, email: '', phone: '' } : null);
  return (
    <div className="ddInv">
      <style>{css(t)}</style>
      <div className="ddHead">
        <div className="ddBrand">
          {t.showLogo && <img src="/site/assets/logo.webp" alt="" />}
          <div>
            <h1>{t.title}</h1>
            <div className="ddNum">{invoice.invoiceNumber}</div>
          </div>
        </div>
        <div className="ddMeta">
          {(invoice.issuedAt || invoice.createdAt) && <div>{invoice.issuedAt ? 'Issued' : 'Created'} {day(invoice.issuedAt ?? invoice.createdAt)}</div>}
          <span className="ddStatus">{invoice.status.replaceAll('_', ' ')}</span>
        </div>
      </div>
      <div className="ddParties">
        <div>
          {invoice.issuerSnapshot && <>
            <p><strong>{invoice.issuerSnapshot.legalName}</strong></p>
            <p>{invoice.issuerSnapshot.address}</p>
            {t.showTaxRegistration && invoice.issuerSnapshot.taxRegistration && <p>Tax registration: {invoice.issuerSnapshot.taxRegistration}</p>}
          </>}
        </div>
        <div>
          {customer && <>
            <h2>{t.billToLabel}</h2>
            <p><strong>{customer.name}</strong></p>
            {t.showCustomerContact && customer.email && <p>{customer.email}</p>}
            {t.showCustomerContact && customer.phone && <p>{customer.phone}</p>}
          </>}
        </div>
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
        <div><span>Total ({invoice.currency})</span><span>{money(invoice.totalCents, invoice.currency)}</span></div>
        {invoice.paidCents > 0 && <div><span>Paid</span><span>{money(invoice.paidCents, invoice.currency)}</span></div>}
        <div className="ddDue"><span>Balance due</span><span>{money(invoice.dueCents, invoice.currency)}</span></div>
      </div>
      {t.paymentInstructions && <div className="ddPay"><h2>Payment</h2><p>{t.paymentInstructions}</p></div>}
      {t.footerNote && <div className="ddFoot"><p>{t.footerNote}</p></div>}
    </div>
  );
}

/* Prints through a temporary layer so the app chrome never appears on paper: everything else in <body> is hidden while printing. */
export function printInvoiceDocument(invoice: PrintInvoice, template: PrintTemplate) {
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
