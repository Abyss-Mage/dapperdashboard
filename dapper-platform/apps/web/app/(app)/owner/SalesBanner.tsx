'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import { DatePicker } from '../Pickers';

type Banner = { active: boolean; text: string; linkLabel: string; linkUrl: string; tone: 'red' | 'black'; endsAt: string | null };
const empty: Banner = { active: false, text: '', linkLabel: '', linkUrl: '', tone: 'red', endsAt: null };
// A date in the franchise's calendar ("2026-11-30") from the stored end time, and back: the strip shows until the end of that day.
const dayOf = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString('en-CA') : '');
const endOf = (day: string) => (day ? new Date(`${day}T23:59:59`).toISOString() : null);

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* The sales banner: a thin strip above the website header (a sale, a seasonal offer), switched on and off here. */
export default function SalesBanner({ base }: { base: string }) {
  const root = `${base}/content`;
  const [form, setForm] = useState<Banner>(empty);
  const [hasSite, setHasSite] = useState<boolean | undefined>(undefined);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const set = <K extends keyof Banner>(k: K, v: Banner[K]) => { setForm((f) => ({ ...f, [k]: v })); setNotice(''); };

  const load = useCallback(async () => {
    const data = await readJson(await fetch(root, { cache: 'no-store' }));
    setHasSite(!!data.site?.slug);
    setForm({ ...empty, ...(data.banner ?? {}), tone: data.banner?.tone === 'black' ? 'black' : 'red' });
  }, [root]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function save(next: Banner) {
    setBusy(true); setError(''); setNotice('');
    try {
      const saved = await readJson(await fetch(`${root}/banner`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(next) }));
      setForm({ ...empty, ...saved });
      setNotice(saved.active ? 'Saved. The banner is live on the website.' : 'Saved. The banner is switched off.');
    } catch (e) { setError(e instanceof Error ? e.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }

  const ended = !!form.endsAt && new Date(form.endsAt) <= new Date();
  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Sales banner</h2>
          <p className={styles.status} style={{ margin: '4px 0 0' }}>A thin strip across the top of every website page, above the header. Use it for a sale or a seasonal offer.</p>
        </div>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {hasSite === undefined && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading the banner" />}
      {hasSite === false && <p className={styles.status}>This franchise has no website yet. Set one up under Website images first.</p>}
      {hasSite && (
        <form className={styles.resource} onSubmit={(e) => { e.preventDefault(); void save(form); }}>
          <p className={styles.bannerLabel}>Preview</p>
          <div className={styles.bannerPreview} data-tone={form.tone} aria-hidden="true">
            <span>{form.text || 'Your banner text appears here'}</span>
            {form.linkUrl && form.linkLabel && <b>{form.linkLabel} →</b>}
          </div>
          <label className="form-field">Banner text<input required={form.active} minLength={3} maxLength={160} value={form.text} onChange={(e) => set('text', e.target.value)} disabled={busy} placeholder="e.g. Winter sale: 15% off ceramic coating this month" /></label>
          <div className={styles.inlineForm}>
            <label className="form-field">Button label (optional)<input maxLength={30} value={form.linkLabel} required={!!form.linkUrl} onChange={(e) => set('linkLabel', e.target.value)} disabled={busy} placeholder="e.g. Book now" /></label>
            <label className="form-field">Button link (optional)<input maxLength={300} value={form.linkUrl} required={!!form.linkLabel} onChange={(e) => set('linkUrl', e.target.value.trim())} disabled={busy} placeholder="/book-appointment or https://…" /></label>
          </div>
          <div className={styles.inlineForm}>
            <fieldset className="form-field" style={{ border: 0, padding: 0, margin: 0 }}>
              <legend>Colour</legend>
              <div role="radiogroup" style={{ display: 'flex', gap: 22, marginTop: 8 }}>
                {([['red', 'Dapper red'], ['black', 'Black']] as const).map(([v, l]) => (
                  <label key={v} style={{ display: 'flex', gap: 8, alignItems: 'center', cursor: 'pointer' }}><input type="radio" name="tone" checked={form.tone === v} onChange={() => set('tone', v)} disabled={busy} />{l}</label>
                ))}
              </div>
            </fieldset>
            <label className="form-field">Show until (optional)<DatePicker value={dayOf(form.endsAt)} onChange={(e) => set('endsAt', endOf(e.target.value))} /></label>
          </div>
          {ended && <p className={styles.status}>The end date has passed, so the banner is not showing on the website.</p>}
          <label style={{ display: 'flex', gap: 8, alignItems: 'center', margin: '12px 0' }}><input type="checkbox" checked={form.active} onChange={(e) => set('active', e.target.checked)} disabled={busy} />Show the banner on the website</label>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
            {form.endsAt && <button type="button" className="secondary" disabled={busy} onClick={() => set('endsAt', null)}>Clear end date</button>}
          </div>
        </form>
      )}
    </div>
  );
}
