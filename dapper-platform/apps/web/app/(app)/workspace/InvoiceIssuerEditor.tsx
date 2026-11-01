'use client';
import { useCallback, useEffect, useState } from 'react';
import NumberField from '../NumberField';

type Issuer = { id: string; legalName: string; address: string; taxRegistration: string; currency: string; numberPrefix: string; version: number; nextNumber: number };
type Configuration = { issuers: Issuer[]; locations: { id: string; name: string }[]; bindings: { locationId: string; issuerId: string }[] };
const blank = { legalName: '', address: '', taxRegistration: '', numberPrefix: '', startNumber: '' };

/* The business details printed on every invoice, and the numbering: the legal name and address, an optional tax number, and the
   prefix and next number for invoice numbers. One set of details covers all franchises: saving it switches it on for every
   franchise that does not use it yet, so there is nothing else to link. */
export default function InvoiceIssuerEditor({ organizationId, revision }: { organizationId: string; revision: number }) {
  const base = `/api/v1/organizations/${organizationId}/invoice-issuers`;
  const [data, setData] = useState<Configuration | null>(null);
  const [form, setForm] = useState(blank);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const response = await fetch(base, { cache: 'no-store' });
    if (!response.ok) throw new Error('The invoice details are unavailable, or you no longer have access to them.');
    const next: Configuration = await response.json();
    setData(next);
    const first = next.issuers[0];
    setForm(first ? { legalName: first.legalName, address: first.address, taxRegistration: first.taxRegistration, numberPrefix: first.numberPrefix, startNumber: '' } : blank);
  }, [base]);
  useEffect(() => { setError(''); load().catch((e) => setError(e.message)); }, [load, revision]);

  const issuer = data?.issuers[0];
  const linked = new Set((data?.bindings ?? []).filter((b) => b.issuerId === issuer?.id).map((b) => b.locationId));
  const unlinked = (data?.locations ?? []).filter((l) => !linked.has(l.id));

  async function save(event: React.FormEvent) {
    event.preventDefault();
    const startNumber = Number(form.startNumber);
    if (!issuer && (!Number.isSafeInteger(startNumber) || startNumber < 1)) { setError('Type the number the first invoice should have, for example 1.'); return; }
    setBusy(true); setError(''); setNotice('');
    try {
      const { startNumber: unused, ...fields } = form; void unused;
      const response = await fetch(base + (issuer ? '/' + issuer.id : ''), {
        method: issuer ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...fields, currency: 'CAD', version: issuer?.version ?? 0, ...(!issuer ? { startNumber } : {}) }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.error ?? 'The details could not be saved. What you typed is still here.');
      // switch the details on for every franchise that is not using them yet
      const skipped: string[] = [];
      for (const l of data?.locations ?? []) {
        if (issuer && linked.has(l.id)) continue;
        const link = await fetch(base + '/locations/' + l.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ issuerId: body.id }) });
        if (!link.ok) skipped.push(l.name);
      }
      await load();
      setNotice(skipped.length ? `Saved, but these franchises could not be switched on: ${skipped.join(', ')}.` : 'Saved. Invoices from every franchise now show these details. Invoices already issued keep the details they were issued with.');
    } catch (e) { setError(e instanceof Error ? e.message : 'The details could not be saved.'); }
    finally { setBusy(false); }
  }

  return (
    <section className="panel detail-card">
      <h2>Business details on invoices</h2>
      <p>These appear at the top of every invoice and decide how invoices are numbered. Fill them in once, with your real legal name and address. You need them before you can take a payment at the end of a job.</p>
      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      {data && (
        <form onSubmit={save}>
          <div className="management-form">
            <label className="form-field">Business legal name<input required minLength={2} maxLength={200} value={form.legalName} disabled={busy} onChange={(e) => setForm({ ...form, legalName: e.target.value })} placeholder="Your business name as registered" /></label>
            <label className="form-field">Tax number (GST/HST, if you have one)<input maxLength={100} value={form.taxRegistration} disabled={busy} onChange={(e) => setForm({ ...form, taxRegistration: e.target.value })} /></label>
          </div>
          <label className="form-field">Business address<textarea required minLength={5} maxLength={1000} value={form.address} disabled={busy} onChange={(e) => setForm({ ...form, address: e.target.value })} placeholder={'Street, city, province, postal code'} /></label>
          <div className="management-form">
            <label className="form-field">Invoice number starts with<input required maxLength={24} pattern="[A-Z0-9][A-Z0-9-]*" title="Capital letters, digits and dashes only" value={form.numberPrefix} disabled={busy} onChange={(e) => setForm({ ...form, numberPrefix: e.target.value.toUpperCase() })} placeholder="DDP" /></label>
            {!issuer && (
              <label className="form-field" style={{ maxWidth: 280 }}>First invoice number
                <NumberField required min="1" step="1" placeholder="e.g. 1" value={form.startNumber} disabled={busy} onChange={(e) => setForm({ ...form, startNumber: e.target.value })} />
              </label>
            )}
          </div>
          {issuer && <p>The next invoice will be {Number.isSafeInteger(issuer.nextNumber) ? issuer.numberPrefix + issuer.nextNumber : 'out of range'}. Changing the details here never restarts the numbering. Prices are in Canadian dollars (CAD).</p>}
          {issuer && unlinked.length > 0 && <p>Not switched on yet at: {unlinked.map((l) => l.name).join(', ')}. Saving switches it on.</p>}
          <button className="primary" disabled={busy}>{busy ? 'Saving…' : issuer ? 'Save business details' : 'Save and switch on for all franchises'}</button>
        </form>
      )}
    </section>
  );
}
