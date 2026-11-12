'use client';

import { useEffect, useState } from 'react';
import styles from './owner.module.css';
import { SITE_SLUG } from '../../(site)/_components/site';
import { APP_SITES, MAIN_SITE, sitePages, type PageSEO, type SiteSEO } from '../../seo-pages';

/* Search & sitemaps (owner 2026-11-13): the XML sitemaps of the website, the dashboard and the workspace, and the website's search
   settings: site-wide title, description, keywords, share image and search-console codes, then the same per page with a switch to keep
   a page out of search. For the owner and administrators (access.manage). */

const EMPTY: SiteSEO = { title: '', description: '', keywords: '', image: '', googleVerification: '', bingVerification: '', pages: {} };
const NO_PAGE: PageSEO = { title: '', description: '', keywords: '', image: '', hidden: false };

async function read<T>(r: Response): Promise<T> {
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error ?? 'That did not work. Please try again.');
  return b as T;
}

function Count({ value, max }: { value: string; max: number }) {
  return <span className={styles.personSub} style={{ fontWeight: 400 }}> ({value.length}/{max})</span>;
}

export default function SearchSettings({ organizationId }: { organizationId: string }) {
  const url = `/api/v1/organizations/${encodeURIComponent(organizationId)}/sites/${encodeURIComponent(SITE_SLUG)}/seo`;
  const [seo, setSeo] = useState<SiteSEO | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');

  useEffect(() => {
    fetch(url, { cache: 'no-store' }).then((r) => read<SiteSEO>(r)).then((s) => setSeo({ ...EMPTY, ...s, pages: s.pages ?? {} })).catch((e) => { setError(e.message); setSeo(EMPTY); });
  }, [url]);

  const set = (patch: Partial<SiteSEO>) => setSeo((s) => ({ ...(s ?? EMPTY), ...patch }));
  const page = (path: string) => seo?.pages?.[path] ?? NO_PAGE;
  const setPage = (path: string, patch: Partial<PageSEO>) => setSeo((s) => ({ ...(s ?? EMPTY), pages: { ...(s?.pages ?? {}), [path]: { ...NO_PAGE, ...(s?.pages?.[path] ?? {}), ...patch } } }));

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!seo) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const saved = await read<SiteSEO>(await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(seo) }));
      setSeo({ ...EMPTY, ...saved, pages: saved.pages ?? {} });
      setNotice('Saved. The website shows the new settings within a minute.');
    } catch (err) { setError(err instanceof Error ? err.message : 'The settings could not be saved.'); } finally { setBusy(false); }
  }

  const sitemaps = [
    { label: 'Website', host: MAIN_SITE.replace('https://', '') },
    ...APP_SITES.map((a) => ({ label: a.label, host: a.host })),
  ];

  return (
    <div className={styles.stack}>
      <section className={styles.card} style={{ padding: 20 }}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>XML sitemaps</h2>
        <p className={styles.personSub}>Give these to Google Search Console and Bing Webmaster Tools. The dashboard and workspace sitemaps list only their sign-in page; everything behind sign-in stays out of search. Each address works once its domain points to this server.</p>
        <ul style={{ display: 'grid', gap: 10, listStyle: 'none', padding: 0, margin: '12px 0 0' }}>
          {sitemaps.map((s) => (
            <li key={s.host} style={{ display: 'flex', flexWrap: 'wrap', gap: '4px 14px', alignItems: 'baseline' }}>
              <strong style={{ minWidth: 150 }}>{s.label}</strong>
              <a href={`https://${s.host}/sitemap.xml`} target="_blank" rel="noreferrer">https://{s.host}/sitemap.xml</a>
              <a className={styles.personSub} href={`https://${s.host}/robots.txt`} target="_blank" rel="noreferrer">robots.txt</a>
            </li>
          ))}
        </ul>
        <p className={styles.personSub} style={{ marginTop: 12 }}>The website sitemap lists: {sitePages.filter((p) => !page(p.path).hidden).map((p) => p.label).join(', ')}.</p>
      </section>

      {!seo ? <p className={styles.status} role="status">Loading the search settings…</p> : (
        <form onSubmit={save} className={styles.stack}>
          <section className={styles.card} style={{ padding: 20 }}>
            <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>Whole website</h2>
            <p className={styles.personSub}>Used on every page unless a page below has its own. Leave a field empty to keep the website&rsquo;s built-in text.</p>
            <label className="form-field">Title in search results<Count value={seo.title} max={70} /><input value={seo.title} onChange={(e) => set({ title: e.target.value })} maxLength={70} disabled={busy} placeholder="Dapper Detailing Pros | Vehicle care in Peterborough" /></label>
            <label className="form-field">Description in search results<Count value={seo.description} max={320} /><textarea rows={3} value={seo.description} onChange={(e) => set({ description: e.target.value })} maxLength={320} disabled={busy} placeholder="Detailing, ceramic coating, paint protection film and window tint in Peterborough, Ontario." /></label>
            <label className="form-field">Keywords, separated by commas<Count value={seo.keywords} max={500} /><input value={seo.keywords} onChange={(e) => set({ keywords: e.target.value })} maxLength={500} disabled={busy} placeholder="car detailing Peterborough, ceramic coating, paint protection film" /></label>
            <label className="form-field">Share image (shown when a link is posted)<input value={seo.image} onChange={(e) => set({ image: e.target.value })} maxLength={400} disabled={busy} placeholder="/site/assets/services/suv.webp or https://…" /></label>
            <label className="form-field">Google Search Console verification code<input value={seo.googleVerification} onChange={(e) => set({ googleVerification: e.target.value })} maxLength={100} disabled={busy} placeholder="The content=&quot;…&quot; value of Google's HTML tag" /></label>
            <label className="form-field">Bing Webmaster verification code<input value={seo.bingVerification} onChange={(e) => set({ bingVerification: e.target.value })} maxLength={100} disabled={busy} placeholder="The content=&quot;…&quot; value of Bing's meta tag" /></label>
          </section>

          <section className={styles.card} style={{ padding: 20 }}>
            <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: '0 0 4px' }}>Each page</h2>
            <p className={styles.personSub}>Open a page to give it its own text. Empty fields use the whole-website settings or the page&rsquo;s built-in text.</p>
            <div style={{ display: 'grid', gap: 8, marginTop: 10 }}>
              {sitePages.map((p) => {
                const v = page(p.path);
                const custom = !!(v.title || v.description || v.keywords || v.image);
                return (
                  <details key={p.path} style={{ border: '1px solid var(--border)', borderRadius: 12, padding: '10px 14px' }}>
                    <summary style={{ cursor: 'pointer', fontWeight: 600 }}>{p.label} <span className={styles.personSub} style={{ fontWeight: 400 }}>{p.path}{custom ? ' · own text' : ''}{v.hidden ? ' · hidden from search' : ''}</span></summary>
                    <div style={{ display: 'grid', gap: 4, marginTop: 10 }}>
                      <label className="form-field">Title<Count value={v.title} max={70} /><input value={v.title} onChange={(e) => setPage(p.path, { title: e.target.value })} maxLength={70} disabled={busy} /></label>
                      <label className="form-field">Description<Count value={v.description} max={320} /><textarea rows={2} value={v.description} onChange={(e) => setPage(p.path, { description: e.target.value })} maxLength={320} disabled={busy} /></label>
                      <label className="form-field">Keywords, separated by commas<input value={v.keywords} onChange={(e) => setPage(p.path, { keywords: e.target.value })} maxLength={500} disabled={busy} /></label>
                      <label className="form-field">Share image<input value={v.image} onChange={(e) => setPage(p.path, { image: e.target.value })} maxLength={400} disabled={busy} placeholder="/site/assets/… or https://…" /></label>
                      <label style={{ display: 'flex', gap: 10, alignItems: 'center' }}><input type="checkbox" checked={v.hidden} onChange={(e) => setPage(p.path, { hidden: e.target.checked })} disabled={busy} />Hide this page from search engines and the sitemap</label>
                    </div>
                  </details>
                );
              })}
            </div>
          </section>

          {error && <p role="alert" className="field-error">{error}</p>}
          {notice && <p role="status" className={styles.status}>{notice}</p>}
          <div><button className="primary" disabled={busy}>{busy ? 'Saving…' : 'Save search settings'}</button></div>
        </form>
      )}
    </div>
  );
}
