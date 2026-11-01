'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import Select from '../GlassSelect';
import Collapse from '../motion/Collapse';
import NumberField from '../NumberField';
import ServiceSlots from './ServiceSlots';

type Service = { id: string; name: string; kind: string; parentId: string | null; version: number; archived: boolean; centralServiceId?: string | null; priceCents?: number | null; durationMinutes?: number | null };
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
const minutesText = (m: number) => (m % 1440 === 0 ? `${m / 1440} day${m === 1440 ? '' : 's'}` : m < 60 ? `${m} min` : `${Math.floor(m / 60)} h${m % 60 ? ` ${m % 60} min` : ''}`);
const UNITS = { minutes: 1, hours: 60, days: 1440 } as const;
type Unit = keyof typeof UNITS;
const bestUnit = (m: number): Unit => (m % 1440 === 0 ? 'days' : m % 60 === 0 ? 'hours' : 'minutes');

export default function ServicesManager({ mode, listUrl, organizationId, canManage, title, intro, extrasOnly = false, locationBase }: { mode: 'central' | 'franchise'; listUrl: string; organizationId: string; canManage: boolean; title: string; intro: string; extrasOnly?: boolean; locationBase?: string }) {
  const [items, setItems] = useState<Service[] | null>(null);
  const [central, setCentral] = useState<Service[]>([]);
  const [filter, setFilter] = useState<Filter>('active');
  const [query, setQuery] = useState('');
  const [editing, setEditing] = useState<Service | 'new' | null>(null);
  const [name, setName] = useState('');
  const [kind, setKind] = useState('main');
  const [parentId, setParentId] = useState('');
  const [parentIds, setParentIds] = useState<string[]>([]);
  const [slotsFor, setSlotsFor] = useState<Service | null>(null);
  const [slotsBase, setSlotsBase] = useState('');
  // Slots belong to the franchise copy of a service. In the central catalogue the selected franchise's linked copy is found first.
  async function openSlots(s: Service) {
    setError(''); setNotice('');
    if (mode === 'franchise') { setSlotsBase(listUrl.slice(0, -'/services'.length)); setSlotsFor(s); return; }
    if (!locationBase) return;
    try {
      const list = (await readJson(await fetch(locationBase + '/services', { cache: 'no-store' }))).items as Service[];
      const local = list.find((x) => x.centralServiceId === s.id && !x.archived);
      if (!local) { setError('This service is not switched on at the selected franchise, so it has no slots there yet.'); return; }
      setSlotsBase(locationBase); setSlotsFor(local);
    } catch (e) { setError(e instanceof Error ? e.message : 'The slot settings could not be opened.'); }
  }
  const [price, setPrice] = useState('');
  const [duration, setDuration] = useState('');
  const [unit, setUnit] = useState<Unit>('minutes');
  const durationMinutes = duration.trim() === '' ? null : Math.round(Number(duration) * UNITS[unit]);
  const [target, setTarget] = useState<Target | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // The whole catalogue can be folded away, and each main service can hide its add-ons; both are remembered on this device.
  const memory = `dapper-collapse:${mode}:${organizationId}`;
  const [folded, setFolded] = useState(false);
  const [closed, setClosed] = useState<Set<string>>(new Set());
  useEffect(() => {
    try { setFolded(localStorage.getItem(memory + ':all') === '1'); setClosed(new Set(JSON.parse(localStorage.getItem(memory + ':groups') ?? '[]'))); } catch { /* storage may be blocked */ }
  }, [memory]);
  const fold = (next: boolean) => { setFolded(next); try { localStorage.setItem(memory + ':all', next ? '1' : '0'); } catch { /* ignore */ } };
  const setGroups = (next: Set<string>) => { setClosed(next); try { localStorage.setItem(memory + ':groups', JSON.stringify([...next])); } catch { /* ignore */ } };
  const toggleGroup = (id: string) => { const next = new Set(closed); if (next.has(id)) next.delete(id); else next.add(id); setGroups(next); };

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
  // On a franchise's extras list only its own services count; central ones belong to the central catalogue above.
  const shown = extrasOnly ? all.filter((s) => !s.centralServiceId) : all;
  const counts = { main: shown.filter((s) => s.kind === 'main' && !s.archived).length, addon: shown.filter((s) => s.kind === 'addon' && !s.archived).length, archived: shown.filter((s) => s.archived).length };
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    const match = (s: Service) => !q || s.name.toLowerCase().includes(q);
    const inFilter = (s: Service) => (filter === 'archived' ? s.archived : !s.archived && (filter === 'active' || s.kind === filter));
    // extrasOnly: a franchise's own services; central services reach every franchise on their own.
    return all.filter((s) => inFilter(s) && match(s) && (!extrasOnly || !s.centralServiceId));
  }, [all, filter, query, extrasOnly]);
  const mains = visible.filter((s) => s.kind === 'main');
  const addonsOf = (id: string) => visible.filter((s) => s.kind === 'addon' && s.parentId === id);
  const orphans = visible.filter((s) => s.kind === 'addon' && !mains.some((m) => m.id === s.parentId));
  const nameOf = (id: string | null) => all.find((s) => s.id === id)?.name ?? 'a main service';

  // An add-on can belong to several main services: each one is its own row (same name, price) and edits are applied to all of them.
  const siblingsOf = (s: Service) => all.filter((x) => x.kind === 'addon' && !x.archived && x.name.trim().toLowerCase() === s.name.trim().toLowerCase());

  function open(next: Service | 'new') {
    setEditing(next);
    setError('');
    setNotice('');
    setName(next === 'new' ? '' : next.name);
    setKind(next === 'new' ? 'main' : next.kind);
    setParentId(next === 'new' ? '' : next.parentId ?? '');
    setParentIds(next === 'new' || next.kind !== 'addon' ? [] : siblingsOf(next).map((x) => x.parentId).filter((x): x is string => !!x));
    setPrice(next === 'new' || next.priceCents == null ? '' : (next.priceCents / 100).toFixed(2));
    const u = next === 'new' || next.durationMinutes == null ? 'minutes' : bestUnit(next.durationMinutes);
    setUnit(u);
    setDuration(next === 'new' || next.durationMinutes == null ? '' : String(next.durationMinutes / UNITS[u]));
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
    const fields = { name: name.trim(), kind, ...(mode === 'central' ? { priceCents: price.trim() === '' ? null : Math.round(Number(price) * 100) } : {}), durationMinutes };
    void run(async () => {
      if (kind === 'addon') {
        const sibs = was && was !== 'new' ? siblingsOf(was) : [];
        for (const sb of sibs) await readJson(await fetch(`${listUrl}/${sb.id}`, send('PUT', { ...fields, parentId: sb.parentId, version: sb.version })));
        const have = new Set(sibs.map((x) => x.parentId));
        for (const pid of parentIds) if (!have.has(pid)) await readJson(await fetch(listUrl, send('POST', { ...fields, parentId: pid })));
      } else {
        await readJson(await fetch(was === 'new' || !was ? listUrl : `${listUrl}/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', { ...fields, parentId: null, ...(was && was !== 'new' ? { version: was.version } : {}) })));
      }
      setEditing(null);
      return `Service “${name.trim()}” saved.`;
    });
  };
  const archive = (s: Service) => run(async () => { await readJson(await fetch(`${listUrl}/${s.id}/archive`, send('POST', { version: s.version }))); return `“${s.name}” archived.`; });
  const del = (s: Service) => run(async () => { const b = await readJson(await fetch(`${listUrl}/${s.id}?version=${s.version}`, send('DELETE'))); return b.message ?? `“${s.name}” removed.`; });
  const duplicate = (s: Service) => run(async () => {
    const copy = `${s.name} (copy)`.slice(0, 120);
    await readJson(await fetch(listUrl, send('POST', { name: copy, kind: s.kind, parentId: s.kind === 'addon' ? s.parentId : null, ...(mode === 'central' ? { priceCents: s.priceCents ?? null } : {}), durationMinutes: s.durationMinutes ?? null })));
    return `“${copy}” created. Rename it and adjust the price.`;
  });
  const enable = (c: Service) => run(async () => { await readJson(await fetch(`${listUrl}/from-central`, send('POST', { centralServiceId: c.id, version: c.version }))); return `Central service “${c.name}” is now available here.`; });

  const linked = new Set(all.map((s) => s.centralServiceId).filter(Boolean));
  const available = central.filter((c) => !linked.has(c.id));

  const row = (s: Service, nested = false, addonCount = 0) => (
    <li key={s.id} className={`${styles.svc} ${nested ? styles.svcNested : ''}`}>
      {addonCount > 0 && (
        <button type="button" className={`${styles.foldBtn} ${closed.has(s.id) ? '' : styles.foldOpen}`} aria-expanded={!closed.has(s.id)} aria-label={`${closed.has(s.id) ? 'Show' : 'Hide'} the ${addonCount} add-on${addonCount === 1 ? '' : 's'} of ${s.name}`} onClick={() => toggleGroup(s.id)}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M6 3.5L10.5 8 6 12.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>
        </button>
      )}
      <strong className={styles.svcName} title={s.name}>{s.name}</strong>
      <span className={styles.svcCell}>{s.archived ? 'Archived' : s.kind === 'addon' ? 'Add-on' : 'Main'}</span>
      <span className={styles.svcCell}>{s.priceCents != null ? money(s.priceCents) : 'No price'}</span>
      <span className={styles.svcCell}>{s.durationMinutes != null ? minutesText(s.durationMinutes) : 'No time'}</span>
      {canManage && !s.archived && (
        <div className={styles.svcActions}>
          {(mode === 'central' || !s.centralServiceId) && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(s)}>Edit</button>}
          {(mode === 'franchise' || !!locationBase) && s.kind === 'main' && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => void openSlots(s)}>Slots</button>}
          {(mode === 'central' || !s.centralServiceId) && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => void duplicate(s)}>Duplicate</button>}
          <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'archive', item: s })}>Archive</button>
          <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'delete', item: s })}>Delete</button>
        </div>
      )}
    </li>
  );

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div className={styles.foldHead}>
          <button type="button" className={`${styles.foldBtn} ${folded ? '' : styles.foldOpen}`} aria-expanded={!folded} aria-label={`${folded ? 'Expand' : 'Collapse'} ${title}`} onClick={() => fold(!folded)}>
            <svg width="18" height="18" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M6 3.5L10.5 8 6 12.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>
          </button>
          <div>
            <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>{title}</h2>
            <p className={styles.personSub}>{folded ? `${counts.main} main services · ${counts.addon} add-ons${counts.archived ? ` · ${counts.archived} archived` : ''}` : <>{intro} <Link href="/content" className={styles.inlineLink}>Service pages &amp; portfolio images</Link></>}</p>
          </div>
        </div>
        {canManage && !editing && <button type="button" className="primary" onClick={() => { fold(false); open('new'); }}>New service</button>}
      </div>
      <Collapse open={!folded} className={styles.stack}>

      {error && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {editing && (
        <form onSubmit={save} className={styles.resource}>
          <h3>{editing === 'new' ? 'New service' : `Edit “${editing.name}”`}</h3>
          <div className={styles.inlineForm}>
            <label className="form-field">Name<input required minLength={2} maxLength={120} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} /></label>
            <label className="form-field">Type
              <Select value={kind} onChange={(e) => { setKind(e.target.value); setParentId(''); }} disabled={busy}>
                <option value="main">Main service</option>
                <option value="addon">Add-on</option>
              </Select>
            </label>
            {mode === 'central' && (
              <label className="form-field">Price before tax (CAD)
                <NumberField inputMode="decimal" min="0" step="0.01" placeholder="Not set" value={price} onChange={(e) => setPrice(e.target.value)} disabled={busy} />
              </label>
            )}
            {kind === 'main' && (
              <label className="form-field">Time to complete
                <span className={styles.durationRow}>
                  <NumberField inputMode="decimal" min="0.5" step={unit === 'minutes' ? '5' : '0.5'} placeholder="Not set" value={duration} onChange={(e) => setDuration(e.target.value)} disabled={busy} />
                  <Select value={unit} onChange={(e) => setUnit(e.target.value as Unit)} disabled={busy} aria-label="Time unit">
                    <option value="minutes">Minutes</option><option value="hours">Hours</option><option value="days">Days</option>
                  </Select>
                </span>
              </label>
            )}
            {kind === 'addon' && (
              <fieldset className={styles.parentPick}>
                <legend>Offered with these main services</legend>
                {all.filter((x) => !x.archived && x.kind === 'main').map((m) => {
                  const locked = editing !== 'new' && editing !== null && siblingsOf(editing).some((x) => x.parentId === m.id);
                  return (
                    <label key={m.id} className={styles.parentOpt}>
                      <input type="checkbox" checked={parentIds.includes(m.id)} disabled={busy || locked} title={locked ? 'Already offered. Archive its row to remove it.' : undefined} onChange={(e) => setParentIds(e.target.checked ? [...parentIds, m.id] : parentIds.filter((x) => x !== m.id))} />
                      <span>{m.name}</span>
                    </label>
                  );
                })}
              </fieldset>
            )}
          </div>
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2 || (kind === 'addon' && parentIds.length === 0) || (price.trim() !== '' && !(Number(price) >= 0)) || (durationMinutes !== null && !(durationMinutes >= 1 && durationMinutes <= 10080))}>{busy ? 'Saving…' : editing === 'new' ? 'Add service' : 'Save revision'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setEditing(null)}>Cancel</button>
          </div>
          <p className={styles.footnote}>{mode === 'central' ? 'The price is entered before tax; each franchise adds its own taxes. ' : 'Prices come from the central catalogue. '}Each save keeps a revision; past work orders and invoices keep the name they were created with.</p>
        </form>
      )}

      {slotsFor && <ServiceSlots base={slotsBase} service={slotsFor} onClose={() => setSlotsFor(null)} />}

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
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          {counts.addon > 0 && <button type="button" className="secondary" onClick={() => setGroups(closed.size ? new Set() : new Set(all.filter((x) => x.kind === 'main').map((x) => x.id)))}>{closed.size ? 'Show all add-ons' : 'Hide all add-ons'}</button>}
          <input type="search" className={styles.searchBox} placeholder="Search services" aria-label="Search services" value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>
      </div>

      {!items && !error && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading services" />}
      {items && visible.length === 0 && <p className={styles.status}>{all.length === 0 ? 'No services yet.' : 'Nothing matches this filter.'}</p>}
      {visible.length > 0 && (
        <ul className={styles.svcList}>
          {mains.map((m) => {
            const kids = addonsOf(m.id);
            return (
              <li key={m.id} className={styles.svcGroup}>
                <ul className={styles.svcList}>
                  {row(m, false, kids.length)}
                  {kids.length > 0 && <li className={styles.svcKids}><Collapse open={!closed.has(m.id)}><ul className={styles.svcList}>{kids.map((a) => row(a, true))}</ul></Collapse></li>}
                </ul>
              </li>
            );
          })}
          {orphans.map((o) => <li key={o.id} className={styles.svcGroup}><ul className={styles.svcList}>{row(o)}</ul></li>)}
        </ul>
      )}

      </Collapse>

      <ConfirmDialog isOpen={target !== null} title={target?.action === 'delete' ? 'Delete service' : 'Archive service'} entityName={target?.item.name ?? ''} entityType="Service" actionType={target?.action ?? 'delete'} isBusy={busy}
        explanation={target?.action === 'delete' ? 'An unused service is removed permanently. One with bookings, invoices or jobs leaves the active catalogue but its history is kept.' : 'Archiving takes the service out of booking and scheduling while keeping completed jobs, past bookings and the audit trail.'}
        impactNotice={target?.item.kind === 'main' ? 'Add-ons attached to this service are affected too.' : undefined}
        confirmLabel={target?.action === 'delete' ? 'Delete service' : 'Archive service'}
        onConfirm={async () => { if (!target) return; const t = target; setTarget(null); await (t.action === 'delete' ? del(t.item) : archive(t.item)); }}
        onCancel={() => { if (!busy) setTarget(null); }} />
    </div>
  );
}
