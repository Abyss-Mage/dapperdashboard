'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';

type Service = { id: string; name: string; kind: string; parentId: string | null; version: number; archived: boolean; centralServiceId?: string | null; priceCents?: number | null };
const money = (c: number) => `$${(c / 100).toLocaleString('en-CA', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
type Filter = 'active' | 'main' | 'addon' | 'archived';
type Target = { action: 'delete' | 'archive'; item: Service };

async function readJson(r: Response) {
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error ?? (Object.values(body.errors ?? {}).join('. ') || 'The change could not be saved.'));
  return body;
}
const send = (method: string, body?: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });

/* One catalogue screen for both levels. mode "central" = the organization catalogue; mode "franchise" = the selected franchise's own
   services, with a one-click way to switch on central services. Behaviour matches the older editors: revisions, archive, delete. */
export default function ServicesManager({ mode, listUrl, organizationId, canManage, title, intro }: { mode: 'central' | 'franchise'; listUrl: string; organizationId: string; canManage: boolean; title: string; intro: string }) {
  const [items, setItems] = useState<Service[] | null>(null);
  const [central, setCentral] = useState<Service[]>([]);
  const [filter, setFilter] = useState<Filter>('active');
  const [query, setQuery] = useState('');
  const [editing, setEditing] = useState<Service | 'new' | null>(null);
  const [name, setName] = useState('');
  const [kind, setKind] = useState('main');
  const [parentId, setParentId] = useState('');
  const [price, setPrice] = useState('');
  const [target, setTarget] = useState<Target | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    try {
      setItems((await readJson(await fetch(listUrl, { cache: 'no-store' }))).items);
      if (mode === 'franchise' && canManage) {
        const r = await fetch(`/api/v1/organizations/${organizationId}/central-services`, { cache: 'no-store' });
        setCentral(r.ok ? (await r.json()).items.filter((s: Service) => !s.archived) : []);
      }
    } catch (e) {
      setItems(null);
      setError(e instanceof Error ? e.message : 'The catalogue could not be loaded.');
    }
  }, [listUrl, mode, canManage, organizationId]);
  useEffect(() => {
    setItems(null);
    setError('');
    setNotice('');
    setEditing(null);
    void load();
  }, [load]);

  const all = items ?? [];
  const counts = { main: all.filter((s) => s.kind === 'main' && !s.archived).length, addon: all.filter((s) => s.kind === 'addon' && !s.archived).length, archived: all.filter((s) => s.archived).length };
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    const match = (s: Service) => !q || s.name.toLowerCase().includes(q);
    const inFilter = (s: Service) => (filter === 'archived' ? s.archived : !s.archived && (filter === 'active' || s.kind === filter));
    return all.filter((s) => inFilter(s) && match(s));
  }, [all, filter, query]);
  const mains = visible.filter((s) => s.kind === 'main');
  const addonsOf = (id: string) => visible.filter((s) => s.kind === 'addon' && s.parentId === id);
  const orphans = visible.filter((s) => s.kind === 'addon' && !mains.some((m) => m.id === s.parentId));
  const nameOf = (id: string | null) => all.find((s) => s.id === id)?.name ?? 'a main service';

  function open(next: Service | 'new') {
    setEditing(next);
    setError('');
    setNotice('');
    setName(next === 'new' ? '' : next.name);
    setKind(next === 'new' ? 'main' : next.kind);
    setParentId(next === 'new' ? '' : next.parentId ?? '');
    setPrice(next === 'new' || next.priceCents == null ? '' : (next.priceCents / 100).toFixed(2));
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
  const save = (e: React.FormEvent) => {
    e.preventDefault();
    const was = editing;
    void run(async () => {
      await readJson(await fetch(was === 'new' || !was ? listUrl : `${listUrl}/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', { name: name.trim(), kind, parentId: kind === 'addon' ? parentId : null, ...(mode === 'central' ? { priceCents: price.trim() === '' ? null : Math.round(Number(price) * 100) } : {}), ...(was && was !== 'new' ? { version: was.version } : {}) })));
      setEditing(null);
      return `Service “${name.trim()}” saved.`;
    });
  };
  const archive = (s: Service) => run(async () => { await readJson(await fetch(`${listUrl}/${s.id}/archive`, send('POST', { version: s.version }))); return `“${s.name}” archived.`; });
  const del = (s: Service) => run(async () => { const b = await readJson(await fetch(`${listUrl}/${s.id}?version=${s.version}`, send('DELETE'))); return b.message ?? `“${s.name}” removed.`; });
  const enable = (c: Service) => run(async () => { await readJson(await fetch(`${listUrl}/from-central`, send('POST', { centralServiceId: c.id, version: c.version }))); return `Central service “${c.name}” is now available here.`; });

  const linked = new Set(all.map((s) => s.centralServiceId).filter(Boolean));
  const available = central.filter((c) => !linked.has(c.id));

  const row = (s: Service, nested = false) => (
    <li key={s.id} className={`${styles.svc} ${nested ? styles.svcNested : ''}`}>
      <div className={styles.svcMain}>
        <strong>{s.name}</strong>
        <span className={styles.personSub}>
          {s.kind === 'addon' ? `Add-on to ${nameOf(s.parentId)}` : 'Main service'} · {s.priceCents != null ? `${money(s.priceCents)} before tax` : 'no price set'} · revision {s.version}
        </span>
      </div>
      <div className={styles.svcTags}>
        {s.centralServiceId && <span className={styles.chip}>From central catalogue</span>}
        {s.archived && <span className={styles.badge}>Archived</span>}
      </div>
      {canManage && !s.archived && (
        <div className={styles.svcActions}>
          {(mode === 'central' || !s.centralServiceId) && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(s)}>Edit</button>}
          <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'archive', item: s })}>Archive</button>
          <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'delete', item: s })}>Delete</button>
        </div>
      )}
    </li>
  );

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div>
          <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>{title}</h2>
          <p className={styles.personSub}>{intro} <Link href="/content" className={styles.inlineLink}>Service pages &amp; portfolio images</Link></p>
        </div>
        {canManage && !editing && <button type="button" className="primary" onClick={() => open('new')}>New service</button>}
      </div>

      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'New service' : `Edit “${editing.name}”`}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Name<input required minLength={2} maxLength={120} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} /></label>
            <label className="form-field">Type
              <select value={kind} onChange={(e) => { setKind(e.target.value); setParentId(''); }} disabled={busy}>
                <option value="main">Main service</option>
                <option value="addon">Add-on</option>
              </select>
            </label>
            {mode === 'central' && (
              <label className="form-field">Price before tax (CAD)
                <input type="number" inputMode="decimal" min="0" step="0.01" placeholder="Not set" value={price} onChange={(e) => setPrice(e.target.value)} disabled={busy} />
              </label>
            )}
            {kind === 'addon' && (
              <label className="form-field">Belongs to
                <select required value={parentId} onChange={(e) => setParentId(e.target.value)} disabled={busy}>
                  <option value="">Choose a main service</option>
                  {all.filter((s) => !s.archived && s.kind === 'main' && (editing === 'new' || s.id !== editing.id)).map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
              </label>
            )}
          </div>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2 || (kind === 'addon' && !parentId) || (price.trim() !== '' && !(Number(price) >= 0))}>{busy ? 'Saving…' : editing === 'new' ? 'Add service' : 'Save revision'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
          <p className={styles.footnote}>{mode === 'central' ? 'The price is entered before tax; each franchise adds its own taxes. Durations and deposits are set elsewhere and need approved values. ' : 'Prices come from the central catalogue. '}Each save keeps a revision; past work orders and invoices keep the name they were created with.</p>
        </form>
      )}

      {mode === 'franchise' && canManage && available.length > 0 && (
        <section className={styles.resource}>
          <h3>Available from the central catalogue</h3>
          <ul className={styles.svcList}>
            {available.map((c) => {
              const parentLinked = c.kind !== 'addon' || all.some((s) => s.centralServiceId === c.parentId);
              return (
                <li key={c.id} className={styles.svc}>
                  <div className={styles.svcMain}><strong>{c.name}</strong><span className={styles.personSub}>{c.kind === 'addon' ? `Add-on to ${central.find((x) => x.id === c.parentId)?.name ?? 'a main service'}` : 'Main service'}</span></div>
                  <div className={styles.svcActions}><button type="button" className="secondary" disabled={busy || !parentLinked} title={parentLinked ? undefined : 'Enable its main service first'} onClick={() => void enable(c)}>Enable here</button></div>
                </li>
              );
            })}
          </ul>
        </section>
      )}

      <div className={styles.rowBetween}>
        <div className={styles.segments} role="group" aria-label="Filter services">
          {([['active', `All · ${counts.main + counts.addon}`], ['main', `Main · ${counts.main}`], ['addon', `Add-ons · ${counts.addon}`], ['archived', `Archived · ${counts.archived}`]] as [Filter, string][]).map(([id, text]) => (
            <button key={id} type="button" className={styles.segment} aria-pressed={filter === id} onClick={() => setFilter(id)}>{text}</button>
          ))}
        </div>
        <input type="search" className={styles.searchBox} placeholder="Search services" aria-label="Search services" value={query} onChange={(e) => setQuery(e.target.value)} />
      </div>

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading services" />}
      {items && visible.length === 0 && <p className={styles.status}>{all.length === 0 ? 'No services yet.' : 'Nothing matches this filter.'}</p>}
      {visible.length > 0 && (
        <ul className={styles.svcList}>
          {mains.map((m) => (
            <li key={m.id} className={styles.svcGroup}>
              <ul className={styles.svcList}>{row(m)}{addonsOf(m.id).map((a) => row(a, true))}</ul>
            </li>
          ))}
          {orphans.map((o) => <li key={o.id} className={styles.svcGroup}><ul className={styles.svcList}>{row(o)}</ul></li>)}
        </ul>
      )}

      <ConfirmDialog isOpen={target !== null} title={target?.action === 'delete' ? 'Delete service' : 'Archive service'} entityName={target?.item.name ?? ''} entityType="Service" actionType={target?.action ?? 'delete'} isBusy={busy}
        explanation={target?.action === 'delete' ? 'An unused service is removed permanently. One with bookings, invoices or work orders leaves the active catalogue but its history is kept.' : 'Archiving takes the service out of booking and scheduling while keeping completed jobs, past bookings and the audit trail.'}
        impactNotice={target?.item.kind === 'main' ? 'Add-ons attached to this service are affected too.' : undefined}
        confirmLabel={target?.action === 'delete' ? 'Delete service' : 'Archive service'}
        onConfirm={async () => { if (!target) return; const t = target; setTarget(null); await (t.action === 'delete' ? del(t.item) : archive(t.item)); }}
        onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
