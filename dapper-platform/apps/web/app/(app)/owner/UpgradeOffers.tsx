'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import { parseName } from '../workspace/BookingWizard';

type Offer = { fromPack: string; toPack: string; pitch: string; perks: string[]; active: boolean; discountPercent?: number; discountCents?: number };
const blank: Offer = { fromPack: '', toPack: '', pitch: '', perks: [], active: true };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* Upgrade offers for the booking flow (website and staff): when a customer picks one package, the flow suggests a bigger one for the same
   vehicle, shows how much more it costs, and lists what it adds. Packages are named without the vehicle, so one offer covers every vehicle. */
export default function UpgradeOffers({ organizationId, canManage }: { organizationId: string; canManage: boolean }) {
  const url = `/api/v1/organizations/${organizationId}/package-upsells`;
  const [offers, setOffers] = useState<Offer[] | null>(null);
  const [packs, setPacks] = useState<string[]>([]);
  const [form, setForm] = useState<Offer | null>(null);
  const [perksText, setPerksText] = useState('');
  // The discount for taking the offer (error404 #34): a percent or a dollar amount off the upgraded package, for online bookings.
  const [discKind, setDiscKind] = useState<'percent' | 'amount'>('percent'), [discValue, setDiscValue] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const [o, s] = await Promise.all([readJson(await fetch(url, { cache: 'no-store' })), readJson(await fetch(`/api/v1/organizations/${organizationId}/central-services`, { cache: 'no-store' }))]);
    setOffers(o.items ?? []);
    const names = (s.items ?? []).filter((x: { kind: string; archived?: boolean }) => x.kind === 'main' && !x.archived).map((x: { name: string }) => parseName(x.name).pack);
    setPacks([...new Set<string>(names)].sort((a, b) => a.localeCompare(b)));
  }, [url, organizationId]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  const edit = (o: Offer) => { setForm(o); setPerksText(o.perks.join('\n')); setDiscKind(o.discountCents ? 'amount' : 'percent'); setDiscValue(o.discountPercent ? String(o.discountPercent) : o.discountCents ? (o.discountCents / 100).toFixed(2) : ''); setNotice(''); setError(''); };
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true); setError('');
    try {
      await readJson(await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...form, perks: perksText.split('\n'), discountPercent: discKind === 'percent' ? Number(discValue) || 0 : 0, discountCents: discKind === 'amount' ? Math.round((Number(discValue) || 0) * 100) : 0 }) }));
      setForm(null); setNotice('Saved.'); await load();
    } catch (err) { setError(err instanceof Error ? err.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }
  async function remove(o: Offer) {
    setBusy(true); setError('');
    try { await readJson(await fetch(`${url}?fromPack=${encodeURIComponent(o.fromPack)}`, { method: 'DELETE' })); setNotice('Removed.'); await load(); }
    catch (err) { setError(err instanceof Error ? err.message : 'The offer could not be removed.'); }
    finally { setBusy(false); }
  }
  // Offers whose package names no longer exist in the catalogue never show to customers; say so instead of hiding it.
  const known = useMemo(() => new Set(packs.map((p) => p.toLowerCase())), [packs]);
  const missing = (o: Offer) => !known.has(o.fromPack.toLowerCase()) || !known.has(o.toPack.toLowerCase());
  const options = (keep: string) => [...new Set([...(keep ? [keep] : []), ...packs])];

  return (
    <section className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Upgrade offers</h2>
          <p className={styles.status} style={{ margin: '4px 0 0' }}>When a customer picks a package, suggest a bigger one. The booking flow shows the extra cost for their vehicle and what the upgrade adds.</p>
        </div>
        {canManage && !form && <button type="button" className="primary" onClick={() => edit(blank)}>Add offer</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {form && (
        <form onSubmit={save} className={styles.resource}>
          <div className={styles.inlineForm}>
            <label className="form-field">When they choose
              <Select value={form.fromPack} onChange={(e) => setForm({ ...form, fromPack: e.target.value })} required disabled={busy}><option value="">Choose a package</option>{options(form.fromPack).map((p) => <option key={p} value={p}>{p}</option>)}</Select>
            </label>
            <label className="form-field">Suggest upgrading to
              <Select value={form.toPack} onChange={(e) => setForm({ ...form, toPack: e.target.value })} required disabled={busy}><option value="">Choose a package</option>{options(form.toPack).filter((p) => p !== form.fromPack).map((p) => <option key={p} value={p}>{p}</option>)}</Select>
            </label>
          </div>
          <label className="form-field">Why upgrade (optional, one line)<input maxLength={200} value={form.pitch} onChange={(e) => setForm({ ...form, pitch: e.target.value })} disabled={busy} placeholder="e.g. Do the outside as well and save compared with booking them separately" /></label>
          <label className="form-field">What the upgrade adds (optional, one per line, up to 6)<textarea rows={4} value={perksText} onChange={(e) => setPerksText(e.target.value)} disabled={busy} placeholder={'Hand wash and dry\nClay bar decontamination\nWheel and tire shine'} /></label>
          <div className="form-field" style={{ marginTop: 12 }}>
            <span>Discount for upgrading (optional, online bookings)</span>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
              <div role="group" aria-label="Discount type" className={styles.dlToggle} style={{ marginLeft: 0 }}>
                <button type="button" aria-pressed={discKind === 'percent'} onClick={() => setDiscKind('percent')} disabled={busy}>%</button>
                <button type="button" aria-pressed={discKind === 'amount'} onClick={() => setDiscKind('amount')} disabled={busy}>$</button>
              </div>
              <input inputMode="decimal" aria-label={discKind === 'percent' ? 'Discount percent' : 'Discount in dollars'} value={discValue} onChange={(e) => setDiscValue(e.target.value.replace(/[^0-9.]/g, ''))} placeholder={discKind === 'percent' ? 'e.g. 10' : 'e.g. 25.00'} disabled={busy} style={{ width: 140 }} />
              <small style={{ color: 'var(--text-muted)' }}>Taken off the upgraded package when a customer takes the offer on the website.</small>
            </div>
          </div>
          <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '12px 0' }}><input type="checkbox" checked={form.active} onChange={(e) => setForm({ ...form, active: e.target.checked })} disabled={busy} />Offer is switched on</label>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setForm(null)}>Cancel</button>
          </div>
        </form>
      )}

      {!offers && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading upgrade offers" />}
      {offers && offers.length === 0 && !form && <p className={styles.status}>No upgrade offers yet.</p>}
      {offers && offers.length > 0 && (
        <ul className={styles.upList}>
          {offers.map((o) => (
            <li key={o.fromPack} className={styles.upItem}>
              <div>
                <strong>{o.fromPack} → {o.toPack}</strong>
                {!o.active && <span className={styles.upOff}>Off</span>}
                {(o.discountPercent || o.discountCents) ? <p><strong>{o.discountPercent ? `${o.discountPercent}% off` : `$${((o.discountCents ?? 0) / 100).toFixed(2)} off`}</strong> when they upgrade online</p> : null}
                {o.pitch && <p>{o.pitch}</p>}
                {o.perks.length > 0 && <p className={styles.upPerks}>{o.perks.join(' · ')}</p>}
                {missing(o) && <p className="field-error" style={{ margin: 0 }}>One of these packages is no longer in the catalogue, so customers do not see this offer.</p>}
              </div>
              {canManage && <div className={styles.enqAct}>
                <button type="button" className="secondary" disabled={busy} onClick={() => edit(o)}>Edit</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => void remove(o)}>Remove</button>
              </div>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
