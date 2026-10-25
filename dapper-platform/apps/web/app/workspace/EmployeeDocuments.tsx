'use client';
import { Check, Download, FileText } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import styles from './employee.module.css';

type Doc = { id: string; title: string; category: string; version: number; pageCount: number; effectiveDate: string; sha256: string; fileAvailable: boolean; acknowledgedAt?: string };
const categoryLabel: Record<string, string> = { onboarding: 'Onboarding', safety: 'Safety', chemical: 'Chemical', procedure: 'Procedure' };

/* The employee reads the original PDF (served only while it matches its registered checksum), then confirms with their
   typed name. That records an electronic acknowledgment and enables a receipt PDF. It is not a reviewed legal e-signature. */
export default function DocumentsCard({ timezone, onChanged }: { timezone: string; onChanged?: () => void }) {
  const [docs, setDocs] = useState<Doc[] | null>(null), [error, setError] = useState(''), [open, setOpen] = useState(''), [name, setName] = useState(''), [read, setRead] = useState(false), [busy, setBusy] = useState(false), [notice, setNotice] = useState('');
  const nameField = useRef<HTMLInputElement>(null);
  const load = useCallback(() => fetch('/api/v1/employee/documents', { cache: 'no-store' }).then(async r => { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'SOP & Compliance is unavailable.'); setDocs(b.items); setError(''); }).catch(e => setError(e.message)), []);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => { if (open) nameField.current?.focus(); }, [open]);
  const day = (v: string) => new Intl.DateTimeFormat('en-CA', { dateStyle: 'medium', timeZone: timezone }).format(new Date(v));

  async function acknowledge(e: React.FormEvent, id: string) {
    e.preventDefault(); setBusy(true); setError(''); setNotice('');
    try {
      const r = await fetch(`/api/v1/employee/documents/${id}/acknowledge`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ signedName: name.trim() }) });
      const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'The acknowledgment could not be saved.');
      setOpen(''); setName(''); setRead(false); setNotice('Acknowledged. Your receipt is ready to download.'); await load(); onChanged?.();
    } catch (err) { setError(err instanceof Error ? err.message : 'The acknowledgment could not be saved.'); }
    finally { setBusy(false); }
  }
  return <section className={styles.documentsCard} aria-labelledby="documents-title">
    <div className={styles.sectionTop}><h2 id="documents-title">SOP &amp; Compliance</h2><span className={styles.count}>Policy</span></div>
    {error && <p role="alert" className={styles.error}>{error}</p>}{notice && <p role="status" className={styles.notice}><Check size={18} aria-hidden="true" />{notice}</p>}
    {docs === null && !error ? <p className={styles.help}>Loading documents…</p> : docs && !docs.length ? <p className={styles.emptyNote}>Your administrator has not published any documents yet.</p> : docs && <ul className={styles.docList}>
      {docs.map(d => <li key={d.id} className={styles.docItem}>
        <span className={styles.docIcon}><FileText size={20} aria-hidden="true" /></span>
        <div className={styles.docMain}><strong>{d.title}</strong><span>{categoryLabel[d.category] ?? d.category} · version {d.version} · {d.pageCount} page{d.pageCount === 1 ? '' : 's'}{d.acknowledgedAt ? ` · acknowledged ${day(d.acknowledgedAt)}` : ''}</span>
          {!d.fileAvailable && <span className={styles.docWarn}>The file is not available on the server right now.</span>}
          {open === d.id && <form className={styles.ackForm} onSubmit={e => acknowledge(e, d.id)}>
            <label className={styles.checkLabel}><input type="checkbox" checked={read} onChange={e => setRead(e.target.checked)} disabled={busy} />I have read and understood “{d.title}”.</label>
            <label>Type your full name<input ref={nameField} value={name} onChange={e => setName(e.target.value)} maxLength={120} autoComplete="name" disabled={busy} required /></label>
            <div className={styles.docActions}><button className={styles.primary} disabled={busy || !read || name.trim().length < 2}>{busy ? 'Saving…' : 'Acknowledge'}</button><button type="button" className={styles.secondary} disabled={busy} onClick={() => { setOpen(''); setName(''); setRead(false); }}>Cancel</button></div>
          </form>}
        </div>
        <div className={styles.docActions}>
          {d.fileAvailable && <a className={styles.secondary} href={`/api/v1/employee/documents/${d.id}/file`} target="_blank" rel="noreferrer">View PDF</a>}
          {d.acknowledgedAt ? <a className={styles.secondary} href={`/api/v1/employee/documents/${d.id}/receipt`}><Download size={16} aria-hidden="true" />Receipt</a>
            : d.fileAvailable && open !== d.id && <button className={styles.primary} onClick={() => { setOpen(d.id); setNotice(''); }}>Acknowledge</button>}
        </div>
      </li>)}
    </ul>}
  </section>;
}
