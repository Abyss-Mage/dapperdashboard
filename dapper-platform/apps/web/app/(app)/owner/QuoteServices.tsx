'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';

// The list the website uses when none is saved here (the same as the contact page's built-in list).
const BUILT_IN = ['Detailing', 'Paint correction', 'Ceramic coating', 'Paint protection film', 'Window tint', 'Mobile detailing'];

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? 'The change could not be saved.');
  return body;
}

/* The contact page's "Service to quote" list (owner, error404 #163): one service per line. Saving an empty box brings back the built-in list. */
export default function QuoteServices({ base }: { base: string }) {
  const root = `${base}/content`;
  const [text, setText] = useState('');
  const [hasSite, setHasSite] = useState<boolean | undefined>(undefined);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    const data = await readJson(await fetch(root, { cache: 'no-store' }));
    setHasSite(!!data.site?.slug);
    setText(((data.quoteServices as string[] | undefined)?.length ? data.quoteServices : BUILT_IN).join('\n'));
  }, [root]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, [load]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setError(''); setNotice('');
    try {
      const saved = await readJson(await fetch(`${root}/quote-services`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ services: text.split('\n') }) }));
      const list = saved.services as string[];
      setText((list.length ? list : BUILT_IN).join('\n'));
      setNotice(list.length ? `Saved. The contact form offers ${list.length} service${list.length === 1 ? '' : 's'}.` : 'Saved. The contact form uses its built-in list again.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The change could not be saved.'); }
    finally { setBusy(false); }
  }

  if (hasSite === false) return null;
  return (
    <form className={styles.resource} onSubmit={save}>
      <h3 style={{ margin: 0 }}>Services in the quote form</h3>
      <p className={styles.personSub}>The &ldquo;Service to quote&rdquo; list on the website&rsquo;s contact page. One service per line, in the order you want them. Packages without a price send customers here with that package already chosen.</p>
      <textarea className={styles.textBulk} rows={8} maxLength={2500} value={text} onChange={(e) => { setText(e.target.value); setNotice(''); }} disabled={busy || hasSite === undefined} aria-label="Services in the quote form" />
      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}
      <div className={styles.svcActions}>
        <button className="primary" disabled={busy || hasSite === undefined}>{busy ? 'Saving…' : 'Save the list'}</button>
        <button type="button" className="secondary" disabled={busy} onClick={() => setText(BUILT_IN.join('\n'))}>Use the built-in list</button>
      </div>
    </form>
  );
}
