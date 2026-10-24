'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

type Rate = { id: string; name: string; rateBasisPoints: number; active: boolean; version: number };
type Settings = { pricesIncludeTax: boolean; rounding: string | null; version: number };

const pct = (bp: number) => `${(bp / 100).toFixed(2)}%`;
async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The change could not be saved.'));
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

/* Taxes belong to one franchise. Nothing is pre-filled: a franchise has no tax until you add one. */
export default function TaxSettings({ base, franchiseName, canManage }: { base: string; franchiseName: string; canManage: boolean }) {
  const [rates, setRates] = useState<Rate[] | null>(null);
  const [settings, setSettings] = useState<Settings>({ pricesIncludeTax: false, rounding: null, version: 0 });
  const [editing, setEditing] = useState<Rate | 'new' | null>(null);
  const [name, setName] = useState('');
  const [percent, setPercent] = useState('');
  const [active, setActive] = useState(true);
  const [target, setTarget] = useState<Rate | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    try {
      const body = await readJson(await fetch(`${base}/taxes`, { cache: 'no-store' }));
      setRates(body.rates);
      setSettings(body.settings);
      setError('');
    } catch (e) {
      setRates(null);
      setError(e instanceof Error ? e.message : 'Taxes could not be loaded.');
    }
  }, [base]);
  useEffect(() => {
    setEditing(null);
    setNotice('');
    void load();
  }, [load]);

  function open(r: Rate | 'new', preset?: { name: string; percent: string }) {
    setEditing(r);
    setError('');
    setNotice('');
    setName(r === 'new' ? preset?.name ?? '' : r.name);
    setPercent(r === 'new' ? preset?.percent ?? '' : String(r.rateBasisPoints / 100));
    setActive(r === 'new' ? true : r.active);
  }

  async function run(fn: () => Promise<string>) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      setNotice(await fn());
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'The change could not be saved.');
    } finally {
      setBusy(false);
    }
  }
  function save(e: React.FormEvent) {
    e.preventDefault();
    const value = Number(percent);
    if (!Number.isFinite(value) || value < 0 || value > 50) return setError('Enter a rate between 0 and 50 percent.');
    const was = editing;
    const payload = { name: name.trim(), rateBasisPoints: Math.round(value * 100), active };
    void run(async () => {
      await readJson(await fetch(was === 'new' || !was ? `${base}/taxes` : `${base}/taxes/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', was && was !== 'new' ? { ...payload, version: was.version } : payload)));
      setEditing(null);
      return `${payload.name} saved.`;
    });
  }
  const toggle = (r: Rate) => run(async () => { await readJson(await fetch(`${base}/taxes/${r.id}`, send('PUT', { name: r.name, rateBasisPoints: r.rateBasisPoints, active: !r.active, version: r.version }))); return `${r.name} ${r.active ? 'turned off' : 'turned on'}.`; });
  const remove = (r: Rate) => run(async () => { await readJson(await fetch(`${base}/taxes/${r.id}`, send('DELETE'))); return `${r.name} deleted.`; });
  const saveSettings = (next: { pricesIncludeTax: boolean; rounding: string }) => run(async () => { setSettings(await readJson(await fetch(`${base}/tax-settings`, send('PUT', { ...next, version: settings.version })))); return 'Tax settings saved.'; });

  const activeRates = (rates ?? []).filter((r) => r.active);
  const combined = activeRates.reduce((a, r) => a + r.rateBasisPoints, 0);

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <p className={styles.personSub}>Taxes for <strong>{franchiseName}</strong>. Each franchise has its own. Prices are entered without tax and tax is added separately unless you change that below.</p>
        {canManage && !editing && <button type="button" className="primary" onClick={() => open('new')}>Add tax</button>}
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {!canManage && <p className={styles.footnote}>Changing taxes needs the invoice configuration permission (Super Admin or Owner).</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'Add a tax' : `Edit ${editing.name}`}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Tax name<input required minLength={2} maxLength={60} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} placeholder="e.g. HST" /></label>
            <label className="form-field">Rate (%)<input required inputMode="decimal" value={percent} onChange={(e) => setPercent(e.target.value)} disabled={busy} placeholder="e.g. 13" /></label>
            <label className={styles.capItem} style={{ gridTemplateColumns: '20px 1fr', alignSelf: 'end', minHeight: 44 }}>
              <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} disabled={busy} /><span>Active</span>
            </label>
          </div>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2 || !percent.trim()}>{busy ? 'Saving…' : 'Save tax'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
        </form>
      )}

      {rates && rates.length === 0 && !editing && (
        <section className={styles.resource}>
          <p className={styles.status}>No taxes set for this franchise yet.</p>
          {canManage && (
            <>
              <p className={styles.personSub}>Your old booking-system invoice template charged HST at 13% on top of the price. That is a suggestion only: nothing is added until you click, and you can change the name and rate.</p>
              <div><button type="button" className="secondary" onClick={() => open('new', { name: 'HST', percent: '13' })}>Start with HST 13%</button></div>
            </>
          )}
        </section>
      )}

      {rates && rates.length > 0 && (
        <ul className={styles.svcList}>
          {rates.map((r) => (
            <li key={r.id} className={styles.svcGroup}>
              <div className={styles.svc} style={{ gridTemplateColumns: 'minmax(0,1fr) auto auto' }}>
                <div className={styles.svcMain}><strong>{r.name}</strong><span className={styles.personSub}>Revision {r.version}</span></div>
                <div className={styles.svcTags}><span className={styles.codeTag}>{pct(r.rateBasisPoints)}</span><span className={`${styles.badge} ${r.active ? styles.badge_complete : ''}`}>{r.active ? 'Active' : 'Off'}</span></div>
                {canManage && (
                  <div className={styles.svcActions}>
                    <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(r)}>Edit</button>
                    <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => void toggle(r)}>{r.active ? 'Turn off' : 'Turn on'}</button>
                    <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => setTarget(r)}>Delete</button>
                  </div>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {activeRates.length > 0 && <p className={styles.footnote}>Active taxes add up to {pct(combined)} on top of a price. Taxes are saved per franchise; invoices and quotes do not use them yet, so nothing in billing changes until that step is approved.</p>}

      <section className={styles.resource}>
        <h3>How prices and rounding work</h3>
        <div className={styles.inlineForm}>
          <label className="form-field">Prices are entered
            <select value={settings.pricesIncludeTax ? 'in' : 'ex'} disabled={busy || !canManage || !rates} onChange={(e) => void saveSettings({ pricesIncludeTax: e.target.value === 'in', rounding: settings.rounding ?? '' })}>
              <option value="ex">Without tax (tax added separately)</option>
              <option value="in">With tax included</option>
            </select>
          </label>
          <label className="form-field">Rounding
            <select value={settings.rounding ?? ''} disabled={busy || !canManage || !rates} onChange={(e) => void saveSettings({ pricesIncludeTax: settings.pricesIncludeTax, rounding: e.target.value })}>
              <option value="">Not decided yet</option>
              <option value="half_up">Nearest cent, half rounds up</option>
              <option value="half_even">Nearest cent, half rounds to even</option>
              <option value="down">Always round down</option>
            </select>
          </label>
        </div>
        <p className={styles.footnote}>Rounding stays “not decided” until you choose it; nothing is assumed.</p>
      </section>

      <ConfirmDialog isOpen={target !== null} title="Delete tax" entityName={target ? `${target.name} (${pct(target.rateBasisPoints)})` : ''} entityType="Tax" actionType="delete" isBusy={busy}
        explanation="The tax is removed from this franchise. The change is recorded in the tax history."
        impactNotice="Invoices and quotes do not use franchise taxes yet." confirmLabel="Delete tax"
        onConfirm={async () => { if (!target) return; const t = target; setTarget(null); await remove(t); }} onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
