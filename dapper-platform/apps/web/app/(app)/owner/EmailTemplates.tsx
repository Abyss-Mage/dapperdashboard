'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

type Template = { key: string; label: string; subject: string; body: string; placeholders: string[]; custom: boolean };
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
async function readJson(r: Response) {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'That could not be done.');
  return b;
}

/* The wording of the emails the system sends. Plain text with {{placeholders}} that are filled in when the email goes out. */
export default function EmailTemplates({ url }: { url: string }) {
  const [items, setItems] = useState<Template[] | null>(null);
  const [key, setKey] = useState('');
  const [subject, setSubject] = useState(''), [body, setBody] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const pick = useCallback((list: Template[], k: string) => {
    const t = list.find((x) => x.key === k) ?? list[0];
    if (t) { setKey(t.key); setSubject(t.subject); setBody(t.body); }
  }, []);
  const load = useCallback(async (keep?: string) => {
    const b = await readJson(await fetch(url + '/templates', { cache: 'no-store' }));
    setItems(b.items); pick(b.items, keep ?? '');
  }, [url, pick]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  const current = items?.find((x) => x.key === key);
  async function act(fn: () => Promise<string>) {
    setBusy(true); setError(''); setNotice('');
    try { setNotice(await fn()); await load(key); } catch (e) { setError(e instanceof Error ? e.message : 'That could not be done.'); } finally { setBusy(false); }
  }
  const insert = (p: string) => setBody((b) => b + (b.endsWith(' ') || b.endsWith('\n') || !b ? '' : ' ') + '{{' + p + '}}');

  if (!items) return error ? <p role="alert" className="field-error">{error}</p> : null;
  return (
    <section className={styles.resource} aria-label="Email wording">
      <div className={styles.rowBetween}>
        <h3 style={{ margin: 0 }}>Email wording</h3>
        <span className={styles.personSub}>{current?.custom ? 'Using your wording' : 'Using the built-in wording'}</span>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.segments} role="group" aria-label="Which email">
        {items.map((t) => <button key={t.key} type="button" className={styles.segment} aria-pressed={key === t.key} onClick={() => { pick(items, t.key); setError(''); setNotice(''); }}>{t.label}</button>)}
      </div>
      <label className="form-field">Subject<input value={subject} maxLength={200} onChange={(e) => setSubject(e.target.value)} disabled={busy} /></label>
      <label className="form-field">Text<textarea rows={10} maxLength={5000} value={body} onChange={(e) => setBody(e.target.value)} disabled={busy} /></label>
      <p className={styles.footnote}>Blank lines start a new paragraph. Click a name to add it where you are typing; it is replaced with the real value when the email is sent.</p>
      <div className={styles.svcActions}>
        {current?.placeholders.map((p) => <button key={p} type="button" className="secondary" disabled={busy} onClick={() => insert(p)}>{'{{' + p + '}}'}</button>)}
      </div>
      <div className={styles.svcActions}>
        <button type="button" className="primary" disabled={busy || !subject.trim() || !body.trim()} onClick={() => void act(async () => { await readJson(await fetch(`${url}/templates/${key}`, send('PUT', { subject, body }))); return 'Saved. The next email uses this wording.'; })}>{busy ? 'Saving…' : 'Save wording'}</button>
        {current?.custom && <button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => { await readJson(await fetch(`${url}/templates/${key}`, send('DELETE'))); return 'Back to the built-in wording.'; })}>Use built-in wording</button>}
      </div>
    </section>
  );
}
