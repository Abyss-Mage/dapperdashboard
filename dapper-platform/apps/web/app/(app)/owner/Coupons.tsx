'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import Select from '../GlassSelect';
import NumberField from '../NumberField';
import ServicePicker from './ServicePicker';
import { DatePicker } from '../Pickers';

type Coupon = {
  id: string; title: string; code: string; kind: 'percent' | 'fixed'; value: number; period: 'range' | 'never'; startsOn: string; endsOn: string;
  customerId: string; customerName: string; maxUses: number; used: number; serviceIds: string[]; active: boolean; version: number;
};
type Named = { id: string; name: string };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
const money = (cents: number) => `$${(cents / 100).toFixed(2)}`;
const discountText = (c: Coupon) => (c.kind === 'percent' ? `${c.value}% off` : `${money(c.value)} off`);
const periodText = (c: Coupon) => (c.period === 'never' ? 'Never expires' : `${c.startsOn} to ${c.endsOn}`);
const makeCode = () => {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  const bytes = crypto.getRandomValues(new Uint8Array(8));
  return Array.from(bytes, (b) => alphabet[b % alphabet.length]).join('');
};

/* Discount coupons for the selected franchise: a code that takes a percentage or an amount off a booking, optionally limited to a
   period, one customer, some services and a number of uses. Staff can add one to an invoice from the appointment; customers type
   it when they book online. */
export default function Coupons({ base, canManage }: { base: string; canManage: boolean }) {
  const [items, setItems] = useState<Coupon[] | null>(null);
  const [customers, setCustomers] = useState<Named[]>([]);
  const [services, setServices] = useState<Named[]>([]);
  const [editing, setEditing] = useState<Coupon | 'new' | null>(null);
  const [form, setForm] = useState({ title: '', code: '', kind: 'percent' as 'percent' | 'fixed', value: '', period: 'never' as 'range' | 'never', startsOn: '', endsOn: '', customerId: '', maxUses: '0', serviceIds: [] as string[], active: true });
  const [target, setTarget] = useState<Coupon | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    try {
      const [c, cu, sv] = await Promise.all([`${base}/coupons`, `${base}/customer-directory`, `${base}/services`].map((u) => fetch(u, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : { items: [] }))));
      setItems(c.items ?? []);
      setCustomers((cu.items ?? []).map((x: { id: string; name: string }) => ({ id: x.id, name: x.name })));
      setServices((sv.items ?? []).filter((x: { kind: string; archived: boolean }) => x.kind === 'main' && !x.archived).map((x: { id: string; name: string }) => ({ id: x.id, name: x.name })));
      setError('');
    } catch (e) { setItems(null); setError(e instanceof Error ? e.message : 'Coupons could not be loaded.'); }
  }, [base]);
  useEffect(() => { setEditing(null); void load(); }, [load]);

  function edit(c: Coupon | 'new') {
    setEditing(c); setError(''); setNotice('');
    setForm(c === 'new'
      ? { title: '', code: '', kind: 'percent', value: '', period: 'never', startsOn: '', endsOn: '', customerId: '', maxUses: '0', serviceIds: [], active: true }
      : { title: c.title, code: c.code, kind: c.kind, value: c.kind === 'percent' ? String(c.value) : (c.value / 100).toFixed(2), period: c.period, startsOn: c.startsOn, endsOn: c.endsOn, customerId: c.customerId, maxUses: String(c.maxUses), serviceIds: c.serviceIds, active: c.active });
  }
  const set = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => setForm((f) => ({ ...f, [key]: value }));

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const value = form.kind === 'percent' ? Math.round(Number(form.value)) : Math.round(Number(form.value) * 100);
      const body = { title: form.title, code: form.code, kind: form.kind, value, period: form.period, startsOn: form.startsOn, endsOn: form.endsOn, customerId: form.customerId, maxUses: Math.round(Number(form.maxUses) || 0), serviceIds: form.serviceIds, active: form.active };
      await readJson(await fetch(editing === 'new' || !editing ? `${base}/coupons` : `${base}/coupons/${editing.id}`, send(editing === 'new' || !editing ? 'POST' : 'PUT', { ...body, ...(editing && editing !== 'new' ? { version: editing.version } : {}) })));
      setNotice(editing === 'new' ? 'Coupon added.' : 'Coupon saved.');
      setEditing(null);
      await load();
    } catch (err) { setError(err instanceof Error ? err.message : 'The coupon could not be saved.'); }
    finally { setBusy(false); }
  }
  async function remove() {
    if (!target) return;
    setBusy(true); setError('');
    try { await readJson(await fetch(`${base}/coupons/${target.id}`, send('DELETE'))); setNotice(`Coupon ${target.code} deleted.`); setTarget(null); await load(); }
    catch (err) { setError(err instanceof Error ? err.message : 'The coupon could not be deleted.'); setTarget(null); }
    finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Coupons</h2>
        {canManage && !editing && <button type="button" className="primary" onClick={() => edit('new')}>Add coupon</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'Add coupon' : `Edit ${editing.code}`}</h3>
          <div role="radiogroup" aria-label="Coupon period" style={{ display: 'flex', gap: 22, margin: '4px 0 12px' }}>
            {([['range', 'Date range'], ['never', 'Never expires']] as const).map(([v, l]) => (
              <label key={v} style={{ display: 'flex', gap: 8, alignItems: 'center', cursor: 'pointer' }}><input type="radio" name="period" checked={form.period === v} onChange={() => set('period', v)} />{l}</label>
            ))}
          </div>
          {form.period === 'range' && (
            <div className={styles.inlineForm}>
              <label className="form-field">Starts<DatePicker value={form.startsOn} onChange={(e) => set('startsOn', e.target.value)} max={form.endsOn || undefined} required /></label>
              <label className="form-field">Ends<DatePicker value={form.endsOn} onChange={(e) => set('endsOn', e.target.value)} min={form.startsOn || undefined} required /></label>
            </div>
          )}
          <div className={styles.inlineForm}>
            <label className="form-field">Title<input required maxLength={100} value={form.title} onChange={(e) => set('title', e.target.value)} disabled={busy} placeholder="Enter coupon title" /></label>
            <label className="form-field">Coupon code
              <span style={{ display: 'flex', gap: 8 }}>
                <input required minLength={3} maxLength={32} pattern="[A-Za-z0-9\-]{3,32}" value={form.code} onChange={(e) => set('code', e.target.value.toUpperCase())} disabled={busy} placeholder="Enter coupon code" style={{ flex: 1, minWidth: 0 }} />
                <button type="button" className="secondary" disabled={busy} onClick={() => set('code', makeCode())}>Generate</button>
              </span>
            </label>
            <label className="form-field">Discount
              <span style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 110px', gap: 8 }}>
                <NumberField required min="0.01" step="0.01" inputMode="decimal" value={form.value} onChange={(e) => set('value', e.target.value)} disabled={busy} placeholder="Enter discount" />
                <Select value={form.kind} onChange={(e) => set('kind', e.target.value as 'percent' | 'fixed')} disabled={busy} aria-label="Discount type"><option value="percent">%</option><option value="fixed">$</option></Select>
              </span>
            </label>
          </div>
          <div className={styles.inlineForm}>
            <label className="form-field">Customer (optional)
              <Select value={form.customerId} onChange={(e) => set('customerId', e.target.value)} disabled={busy}><option value="">Any customer</option>{customers.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</Select>
            </label>
            <label className="form-field">Times it can be used (0 for unlimited)
              <NumberField min="0" step="1" inputMode="numeric" value={form.maxUses} onChange={(e) => set('maxUses', e.target.value)} disabled={busy} />
            </label>
          </div>
          <ServicePicker label="Only for these services" services={services} value={form.serviceIds} onChange={(ids) => set('serviceIds', ids)} disabled={busy} />
          <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '12px 0' }}><input type="checkbox" checked={form.active} onChange={(e) => set('active', e.target.checked)} disabled={busy} />Coupon is switched on</label>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </form>
      )}

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading coupons" />}
      {items && items.length === 0 && <p className={styles.status}>No coupons yet.</p>}
      <ul className={styles.people}>
        {(items ?? []).map((c) => (
          <li key={c.id} className={styles.person} style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) auto', gap: 12, alignItems: 'center', opacity: c.active ? 1 : 0.6 }}>
            <span className={styles.custRow}>
              <strong className={styles.custName}>{c.title}</strong>
              <span className={styles.custContact}><code>{c.code}</code></span>
              <span className={styles.custContact}>{discountText(c)}</span>
              <span className={styles.custCount}>{c.used}{c.maxUses ? ` of ${c.maxUses}` : ''} used</span>
              <span className={styles.custAddress}>{[periodText(c), c.customerName ? `Only ${c.customerName}` : 'Any customer', c.serviceIds.length ? `${c.serviceIds.length} service${c.serviceIds.length === 1 ? '' : 's'}` : 'All services', c.active ? 'On' : 'Off'].join(' · ')}</span>
            </span>
            {canManage && !editing && (
              <span className={styles.svcActions}>
                <button type="button" className="secondary" disabled={busy} onClick={() => edit(c)}>Edit</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => setTarget(c)}>Delete</button>
              </span>
            )}
          </li>
        ))}
      </ul>
      <ConfirmDialog isOpen={target !== null} title="Delete coupon" entityName={target?.code ?? ''} entityType="Coupon" actionType="delete" isBusy={busy}
        explanation="A coupon that has never been used is deleted. One that has been used cannot be deleted; switch it off instead."
        confirmLabel="Delete coupon" onConfirm={remove} onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
