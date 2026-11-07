'use client';
import { useCallback, useEffect, useState } from 'react';
import NumberField from '../NumberField';
import Select from '../GlassSelect';

type Issuer = { id: string; legalName: string; address: string; taxRegistration: string; currency: string; numberPrefix: string; version: number; nextNumber: number };
type Configuration = { issuers: Issuer[]; locations: { id: string; name: string }[]; bindings: { locationId: string; issuerId: string }[] };
const blank = { legalName: '', address: '', taxRegistration: '', numberPrefix: '', startNumber: '' };

/* The business details printed on invoices and quotations, and the numbering. Dapper's own outlets (DDP) share one set of
   details and one number series, for example DDP1, DDP2. A franchise that is run by its own owner gets its own set, with its
   own name, address and number prefix, and each outlet is pointed at the set it uses. */
export default function InvoiceIssuerEditor({ organizationId, revision }: { organizationId: string; revision: number }) {
  const base = `/api/v1/organizations/${organizationId}/invoice-issuers`;
  const [data, setData] = useState<Configuration | null>(null);
  const [editing, setEditing] = useState<string | 'new' | null>(null);
  const [form, setForm] = useState(blank);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const response = await fetch(base, { cache: 'no-store' });
    if (!response.ok) throw new Error('The invoice details are unavailable, or you no longer have access to them.');
    setData(await response.json());
  }, [base]);
  useEffect(() => { setError(''); load().catch((e) => setError(e.message)); }, [load, revision]);

  const issuers = data?.issuers ?? [];
  const locations = data?.locations ?? [];
  const bound = (locationId: string) => data?.bindings.find((b) => b.locationId === locationId)?.issuerId ?? '';
  const usedBy = (issuerId: string) => locations.filter((l) => bound(l.id) === issuerId).map((l) => l.name);
  const current = editing && editing !== 'new' ? issuers.find((i) => i.id === editing) : undefined;

  function open(target: string | 'new') {
    const i = target === 'new' ? undefined : issuers.find((x) => x.id === target);
    setForm(i ? { legalName: i.legalName, address: i.address, taxRegistration: i.taxRegistration, numberPrefix: i.numberPrefix, startNumber: '' } : blank);
    setEditing(target); setError(''); setNotice('');
  }

  async function save(event: React.FormEvent) {
    event.preventDefault();
    const startNumber = Number(form.startNumber);
    if (!current && (!Number.isSafeInteger(startNumber) || startNumber < 1)) { setError('Type the number the first invoice should have, for example 1.'); return; }
    setBusy(true); setError(''); setNotice('');
    try {
      const { startNumber: unused, ...fields } = form; void unused;
      const response = await fetch(base + (current ? '/' + current.id : ''), {
        method: current ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...fields, currency: 'CAD', version: current?.version ?? 0, ...(!current ? { startNumber } : {}) }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.error ?? 'The details could not be saved. What you typed is still here.');
      // The very first set of details switches itself on for every outlet; later sets are pointed at outlets one by one below.
      const skipped: string[] = [];
      if (!current && issuers.length === 0) {
        for (const l of locations) {
          const link = await fetch(base + '/locations/' + l.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ issuerId: body.id }) });
          if (!link.ok) skipped.push(l.name);
        }
      }
      await load();
      setEditing(null);
      setNotice(skipped.length ? `Saved, but these outlets could not be switched on: ${skipped.join(', ')}.` : current ? 'Saved. Invoices already issued keep the details they were issued with.' : issuers.length === 0 ? 'Saved and switched on for every outlet.' : 'Saved. Now choose below which outlets use it.');
    } catch (e) { setError(e instanceof Error ? e.message : 'The details could not be saved.'); }
    finally { setBusy(false); }
  }

  async function assign(locationId: string, issuerId: string) {
    if (!issuerId) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(base + '/locations/' + locationId, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ issuerId }) });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'That outlet could not be switched.');
      await load();
      setNotice('Saved. New invoices and quotations from that outlet use these details.');
    } catch (e) { setError(e instanceof Error ? e.message : 'That outlet could not be switched.'); }
    finally { setBusy(false); }
  }

  return (
    <section className="panel detail-card">
      <h2>Business details on invoices</h2>
      <p>These appear at the top of invoices and quotations and decide how they are numbered. Dapper&apos;s own outlets share one set (for example numbers starting DDP). A franchise run by its own owner gets a separate set with its own name, address and prefix. You need these before you can take a payment at the end of a job.</p>
      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      {data && (
        <>
          {issuers.length > 0 && (
            <ul className="management-list">
              {issuers.map((i) => (
                <li key={i.id} className="panel" style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap', alignItems: 'center' }}>
                  <div>
                    <strong>{i.legalName}</strong>
                    <p style={{ margin: '0.2rem 0 0' }}>Next invoice: {Number.isSafeInteger(i.nextNumber) ? i.numberPrefix + i.nextNumber : 'out of range'} · Used by: {usedBy(i.id).join(', ') || 'no outlet yet'}</p>
                  </div>
                  <button type="button" className="secondary" disabled={busy} onClick={() => open(i.id)}>Edit</button>
                </li>
              ))}
            </ul>
          )}

          {editing === null && <div><button type="button" className="primary" disabled={busy} onClick={() => open('new')}>{issuers.length === 0 ? 'Add your business details' : '+ Add a franchise’s own details'}</button></div>}

          {editing !== null && (
            <form onSubmit={save}>
              <h3>{current ? 'Edit business details' : issuers.length === 0 ? 'Your business details' : 'A franchise’s own business details'}</h3>
              <div className="management-form">
                <label className="form-field">Business legal name<input required minLength={2} maxLength={200} value={form.legalName} disabled={busy} onChange={(e) => setForm({ ...form, legalName: e.target.value })} placeholder="Business name as registered" /></label>
                <label className="form-field">Tax number (GST/HST, if you have one)<input maxLength={100} value={form.taxRegistration} disabled={busy} onChange={(e) => setForm({ ...form, taxRegistration: e.target.value })} /></label>
              </div>
              <label className="form-field">Business address<textarea required minLength={5} maxLength={1000} value={form.address} disabled={busy} onChange={(e) => setForm({ ...form, address: e.target.value })} placeholder={'Street, city, province, postal code'} /></label>
              <div className="management-form">
                <label className="form-field">Invoice number starts with<input required maxLength={24} pattern="[A-Z0-9][A-Z0-9-]*" title="Capital letters, digits and dashes only" value={form.numberPrefix} disabled={busy} onChange={(e) => setForm({ ...form, numberPrefix: e.target.value.toUpperCase() })} placeholder={issuers.length === 0 ? 'DDP' : 'A short code for this franchise'} /></label>
                {!current && (
                  <label className="form-field" style={{ maxWidth: 280 }}>First invoice number
                    <NumberField required min="1" step="1" placeholder="e.g. 1" value={form.startNumber} disabled={busy} onChange={(e) => setForm({ ...form, startNumber: e.target.value })} />
                  </label>
                )}
              </div>
              {current && <p>The next invoice will be {Number.isSafeInteger(current.nextNumber) ? current.numberPrefix + current.nextNumber : 'out of range'}. Changing the details never restarts the numbering. Prices are in Canadian dollars (CAD).</p>}
              <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save business details'}</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
              </div>
            </form>
          )}

          {issuers.length > 0 && locations.length > 0 && (issuers.length > 1 || locations.some((l) => !bound(l.id))) && (
            <div>
              <h3>Which details each outlet uses</h3>
              <div className="management-form">
                {locations.map((l) => (
                  <label key={l.id} className="form-field">{l.name}
                    <Select value={bound(l.id)} disabled={busy} onChange={(e) => void assign(l.id, e.target.value)}>
                      {!bound(l.id) && <option value="">Choose…</option>}
                      {issuers.map((i) => <option key={i.id} value={i.id}>{i.legalName} ({i.numberPrefix}…)</option>)}
                    </Select>
                  </label>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </section>
  );
}
