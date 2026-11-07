'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

type Template = { key: string; label: string; subject: string; body: string; isHtml: boolean; placeholders: string[]; custom: boolean };
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
async function readJson(r: Response) {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'That could not be done.');
  return b;
}
const SAMPLE: Record<string, string> = { name: 'Alex Morgan', phone: '+1 705 555 0123', email: 'alex@example.com', service: 'Hatchback - VIP Interior Detail', when: 'Monday, November 9 at 8:30 AM', where: 'At the Dapper shop', total: '$190.97', payment: 'Pay on the day', link: 'https://example.com/reset', organization: 'Dapper Detailing Pros' };
const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const fill = (s: string, safe: boolean) => s.replace(/\{\{(\w+)\}\}/g, (_, k: string) => (safe ? esc(SAMPLE[k] ?? '') : SAMPLE[k] ?? ''));
// Switching a plain-text email to HTML keeps its words: each blank-line paragraph becomes a <p>.
const toHtml = (text: string) => text.split(/\n\s*\n/).map((p) => '<p>' + esc(p).replace(/\n/g, '<br>') + '</p>').join('\n');

/* The wording of the emails the system sends, one editor per email: a list on the left, then the subject, plain text or HTML, and a preview. */
export default function EmailTemplates({ url }: { url: string }) {
  const [items, setItems] = useState<Template[] | null>(null);
  const [key, setKey] = useState('');
  const [subject, setSubject] = useState(''), [body, setBody] = useState(''), [isHtml, setIsHtml] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const pick = useCallback((list: Template[], k: string) => {
    const t = list.find((x) => x.key === k) ?? list[0];
    if (t) { setKey(t.key); setSubject(t.subject); setBody(t.body); setIsHtml(t.isHtml); }
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
  const setFormat = (html: boolean) => { if (html === isHtml) return; setBody((b) => (html ? toHtml(b) : b)); setIsHtml(html); };
  const previewDoc = isHtml ? fill(body, true) : fill(toHtml(body), false).replace(/\{\{(\w+)\}\}/g, '');

  if (!items) return error ? <p role="alert" className="field-error">{error}</p> : null;
  return (
    <section className={styles.resource} aria-label="Email wording">
      <div className={styles.rowBetween}>
        <h3 style={{ margin: 0 }}>Email wording</h3>
        <span className={styles.personSub}>{current?.custom ? 'Using your wording' : 'Using the built-in wording'}</span>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.tplWrap}>
        <ul className={styles.tplList} aria-label="Which email">
          {items.map((t) => (
            <li key={t.key}><button type="button" aria-pressed={key === t.key} onClick={() => { pick(items, t.key); setError(''); setNotice(''); }}>{t.label}{t.custom && <i aria-label="edited" />}</button></li>
          ))}
        </ul>
        <div className={styles.tplEdit}>
          <label className="form-field">Subject<input value={subject} maxLength={200} onChange={(e) => setSubject(e.target.value)} disabled={busy} /></label>
          <div className={styles.segments} role="group" aria-label="Format">
            <button type="button" className={styles.segment} aria-pressed={!isHtml} onClick={() => setFormat(false)}>Plain text</button>
            <button type="button" className={styles.segment} aria-pressed={isHtml} onClick={() => setFormat(true)}>HTML</button>
          </div>
          <label className="form-field">{isHtml ? 'HTML of the email' : 'Text'}
            <textarea className={styles.tplText} style={isHtml ? { fontFamily: 'ui-monospace, Consolas, monospace', fontSize: 13 } : undefined} rows={14} maxLength={5000} value={body} onChange={(e) => setBody(e.target.value)} disabled={busy} spellCheck={!isHtml} />
          </label>
          <p className={styles.footnote}>{isHtml ? 'Write the email as HTML.' : 'Blank lines start a new paragraph.'} Click a name to add it where you are typing; it is replaced with the real value when the email is sent.</p>
          <div className={styles.svcActions}>
            {current?.placeholders.map((p) => <button key={p} type="button" className="secondary" disabled={busy} onClick={() => insert(p)}>{'{{' + p + '}}'}</button>)}
          </div>
          <div className={styles.tplPreview}>
            <span className={styles.personSub}>Preview, with example values</span>
            <strong>{fill(subject, false)}</strong>
            <iframe title="Email preview" sandbox="" srcDoc={`<body style="font:15px/1.5 system-ui,sans-serif;color:#111;margin:12px">${previewDoc}</body>`} />
          </div>
          <div className={styles.svcActions}>
            <button type="button" className="primary" disabled={busy || !subject.trim() || !body.trim()} onClick={() => void act(async () => { await readJson(await fetch(`${url}/templates/${key}`, send('PUT', { subject, body, isHtml }))); return 'Saved. The next email uses this wording.'; })}>{busy ? 'Saving…' : 'Save wording'}</button>
            {current?.custom && <button type="button" className="secondary" disabled={busy} onClick={() => void act(async () => { await readJson(await fetch(`${url}/templates/${key}`, send('DELETE'))); return 'Back to the built-in wording.'; })}>Use built-in wording</button>}
          </div>
        </div>
      </div>
    </section>
  );
}
