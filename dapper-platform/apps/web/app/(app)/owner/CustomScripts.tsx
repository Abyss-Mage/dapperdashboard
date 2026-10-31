'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import ConfirmDialog from '../workspace/ConfirmDialog';

type Script = { id: string; name: string; placement: 'head' | 'body'; code: string; paths: string[]; active: boolean };
type Form = { id: string; name: string; placement: 'head' | 'body'; code: string; paths: string; active: boolean };
const BLANK: Form = { id: '', name: '', placement: 'head', code: '', paths: '', active: true };

/* Extra code for the public website (chat widgets, review badges, custom tracking), the way WPCode works in WordPress.
   Each snippet is one script, style or tag, goes in the page head or the end of the body, and can be limited to pages. */
export default function CustomScripts({ organizationId, revision }: { organizationId: string; revision: number }) {
  const url = `/api/v1/organizations/${organizationId}/custom-scripts`;
  const [items, setItems] = useState<Script[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [target, setTarget] = useState<Script | null>(null);
  const [loaded, setLoaded] = useState(false), [busy, setBusy] = useState(false);
  const [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const r = await fetch(url, { cache: 'no-store' });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'Custom scripts are unavailable or access has changed.');
    setItems(b.items ?? []); setLoaded(true);
  }, [url]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load, revision]);

  async function call(method: string, path: string, body?: unknown) {
    const r = await fetch(url + path, { method, headers: body ? { 'Content-Type': 'application/json' } : undefined, body: body ? JSON.stringify(body) : undefined });
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error ?? 'That could not be saved.');
    return b;
  }
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const body = { name: form.name, placement: form.placement, code: form.code, active: form.active, paths: form.paths.split(/\r?\n|,/).map((p) => p.trim()).filter(Boolean) };
      await call(form.id ? 'PUT' : 'POST', form.id ? '/' + form.id : '', body);
      setForm(null); await load();
      setNotice('Saved. The public website uses it on its next page load.');
    } catch (err) { setError(err instanceof Error ? err.message : 'That could not be saved.'); }
    finally { setBusy(false); }
  }
  async function toggle(s: Script) {
    setBusy(true); setError(''); setNotice('');
    try { await call('PUT', '/' + s.id, { ...s, active: !s.active }); await load(); }
    catch (err) { setError(err instanceof Error ? err.message : 'That could not be changed.'); }
    finally { setBusy(false); }
  }
  async function remove() {
    if (!target) return;
    setBusy(true); setError(''); setNotice('');
    try { await call('DELETE', '/' + target.id); setNotice(`Deleted "${target.name}".`); setTarget(null); await load(); }
    catch (err) { setError(err instanceof Error ? err.message : 'That could not be deleted.'); setTarget(null); }
    finally { setBusy(false); }
  }

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Custom scripts</h2>
          <p className={styles.personSub}>Add as many snippets as you need to the public website. Paste the tag exactly as the provider gives it.</p>
        </div>
        {loaded && !form && <button type="button" className="primary" onClick={() => { setForm({ ...BLANK }); setNotice(''); }}>Add script</button>}
      </div>
      <p className={styles.footnote} role="note">Only add code from a provider you trust. A script runs in every visitor&apos;s browser on this address, which is also where the dashboard lives, so a harmful script could act as anyone who is signed in while viewing the website. Only Super Admin and Owner roles can change this page.</p>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      {form && (
        <form onSubmit={save} className={styles.resource}>
          <div className={styles.inlineForm}>
            <label className="form-field">Name
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} maxLength={80} required placeholder="e.g. Chat widget" disabled={busy} />
            </label>
            <label className="form-field">Goes in
              <Select value={form.placement} onChange={(e) => setForm({ ...form, placement: e.target.value as 'head' | 'body' })} disabled={busy}>
                <option value="head">Page head (loads first)</option>
                <option value="body">End of the page body</option>
              </Select>
            </label>
          </div>
          <label className="form-field">Code
            <textarea value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} rows={9} maxLength={20000} required spellCheck={false} placeholder={'<script src="https://example.com/widget.js" async></script>'} disabled={busy} style={{ fontFamily: 'ui-monospace, Consolas, monospace', fontSize: 13, width: '100%' }} />
          </label>
          <label className="form-field">Only on these pages (optional)
            <textarea value={form.paths} onChange={(e) => setForm({ ...form, paths: e.target.value })} rows={3} spellCheck={false} placeholder={'One per line, for example /contact or /services\nLeave empty for every page'} disabled={busy} style={{ width: '100%' }} />
          </label>
          <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <input type="checkbox" checked={form.active} onChange={(e) => setForm({ ...form, active: e.target.checked })} disabled={busy} /> Switched on
          </label>
          <div className={styles.svcActions}>
            <button type="button" className="secondary" onClick={() => setForm(null)} disabled={busy}>Cancel</button>
            <button className="primary" disabled={busy || !form.name.trim() || !form.code.trim()}>{busy ? 'Saving…' : 'Save script'}</button>
          </div>
        </form>
      )}
      {loaded && !items.length && !form && <p className={styles.status}>No custom scripts yet.</p>}
      {items.map((s) => (
        <div key={s.id} className={styles.resource} style={{ opacity: s.active ? 1 : 0.65 }}>
          <div className={styles.rowBetween}>
            <div>
              <strong>{s.name}</strong>
              <div className={styles.personSub}>{s.placement === 'head' ? 'Page head' : 'End of body'} · {s.paths.length ? s.paths.join(', ') : 'Every page'} · {s.active ? 'On' : 'Off'}</div>
            </div>
            <div className={styles.svcActions}>
              <button type="button" className="secondary" disabled={busy} onClick={() => void toggle(s)}>{s.active ? 'Turn off' : 'Turn on'}</button>
              <button type="button" className="secondary" disabled={busy} onClick={() => { setForm({ id: s.id, name: s.name, placement: s.placement, code: s.code, paths: s.paths.join('\n'), active: s.active }); setNotice(''); }}>Edit</button>
              <button type="button" className="secondary" disabled={busy} onClick={() => setTarget(s)}>Delete</button>
            </div>
          </div>
        </div>
      ))}
      <ConfirmDialog isOpen={target !== null} title="Delete script" entityName={target?.name ?? ''} entityType="Script" actionType="delete" isBusy={busy}
        explanation="The website stops loading it on the next page view." onConfirm={() => void remove()} onCancel={() => setTarget(null)} />
    </div>
  );
}
