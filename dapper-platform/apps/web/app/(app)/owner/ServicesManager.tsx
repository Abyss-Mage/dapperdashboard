'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import styles from './owner.module.css';
import ConfirmDialog from '../workspace/ConfirmDialog';
import Select from '../GlassSelect';
import Collapse from '../motion/Collapse';
import NumberField from '../NumberField';
import ServiceSlots from './ServiceSlots';
import FormPopup from '../FormPopup';
import { categories as websiteCategories } from '../../(site)/content';

type Service = { id: string; name: string; kind: string; parentId: string | null; parentIds?: string[]; version: number; archived: boolean; centralServiceId?: string | null; priceCents?: number | null; durationMinutes?: number | null };
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

export default function ServicesManager({ mode, listUrl, organizationId, canManage, title, intro, extrasOnly = false, locationBase, canSlots = false }: { canSlots?: boolean; mode: 'central' | 'franchise'; listUrl: string; organizationId: string; canManage: boolean; title: string; intro: string; extrasOnly?: boolean; locationBase?: string }) {
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
  // Bulk edit: tick several services, then change their price or time together, or archive them.
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [bulk, setBulk] = useState<null | { priceMode: 'keep' | 'set' | 'up' | 'down'; price: string; timeMode: 'keep' | 'set'; time: string; unit: Unit }>(null);
  // Edit as text: the chosen services (or every service shown) as plain lines, one per service, changed in one go.
  const [textEdit, setTextEdit] = useState<null | { rows: Service[]; text: string; problems: string[] }>(null);
  const togglePick = (id: string) => setPicked((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
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
  // The website category of a service: which page of the website lists it and the description shown there (the selected franchise's website entry).
  const wbase = locationBase ?? (mode === 'franchise' ? listUrl.slice(0, -'/services'.length) : undefined);
  type Pub = { serviceId: string; category: string; summary: string; published: boolean; version: number; starred?: boolean };
  const [web, setWeb] = useState<{ state: 'idle' | 'loading' | 'ready' | 'nosite' | 'missing'; local?: Service; pub?: Pub; cat: string; summary: string; star?: boolean }>({ state: 'idle', cat: '', summary: '' });
  const [webTags, setWebTags] = useState<Record<string, { cat: string; published: boolean }>>({});
  const catTitle = (key: string) => websiteCategories.find((x) => x.key === key)?.short ?? key;
  async function websiteLookup(id: string) {
    const list = (await readJson(await fetch(wbase + '/services', { cache: 'no-store' }))).items as Service[];
    const local = mode === 'franchise' ? list.find((x) => x.id === id) : list.find((x) => x.centralServiceId === id && !x.archived);
    const content = await readJson(await fetch(wbase + '/content', { cache: 'no-store' }));
    const pub = local ? (content.services as Pub[]).find((x) => x.serviceId === local.id) : undefined;
    return { local, pub, site: !!content.site?.slug };
  }
  // Put every main service that is not on the website yet into the category its name says (Sedan - ..., SUV - ..., Aircraft ..., ... PPF), after a preview.
  // Matches the packages dapperdetailing.ca lists on each category page. "Express Interior Detail" is not on the current website, so it is left out.
  const categoryByName = (n: string) => /express interior/i.test(n) ? '' : /ppf/i.test(n) ? 'ppf' : /^2 wheeler/i.test(n) ? '2-wheeler' : /^hatchback/i.test(n) ? 'hatchback' : /^sedan/i.test(n) ? 'sedan' : /^suv/i.test(n) ? 'suv' : /^pickup/i.test(n) ? 'pickup' : /^van\b/i.test(n) ? 'van' : /^aircraft/i.test(n) ? 'aircraft' : /^boat/i.test(n) ? 'boat' : /ceramic|armour/i.test(n) ? 'ceramic' : /^window tint/i.test(n) ? 'window-tint' : '';
  const nounOf = (cat: string) => ({ sedan: 'sedan', suv: 'SUV', pickup: 'pickup', van: 'van', hatchback: 'hatchback', '2-wheeler': 'motorcycle', aircraft: 'aircraft', boat: 'boat' } as Record<string, string>)[cat] ?? 'vehicle';
  const [auto, setAuto] = useState<null | { todo: { local: Service; cat: string }[]; skipped: string[]; done: number }>(null);
  async function previewAuto() {
    if (!wbase) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const locals = (await readJson(await fetch(wbase + '/services', { cache: 'no-store' }))).items as Service[];
      const content = await readJson(await fetch(wbase + '/content', { cache: 'no-store' }));
      if (!content.site?.slug) throw new Error('Set up the website entry under Website images first.');
      const have = new Set((content.services as Pub[]).filter((x) => x.version > 0).map((x) => x.serviceId));
      const todo: { local: Service; cat: string }[] = [], skipped: string[] = [];
      for (const l of locals) {
        if (l.kind !== 'main' || l.archived || have.has(l.id)) continue;
        const cat = categoryByName(l.name);
        if (cat) todo.push({ local: l, cat }); else skipped.push(l.name);
      }
      if (!todo.length && !skipped.length) { setNotice('Every main service is already in a website category.'); return; }
      setAuto({ todo, skipped, done: 0 });
    } catch (e) { setError(e instanceof Error ? e.message : 'The services could not be checked.'); }
    finally { setBusy(false); }
  }
  async function applyAuto() {
    if (!auto || !wbase) return;
    setBusy(true); setError(''); setNotice('');
    let n = 0;
    try {
      for (const { local, cat } of auto.todo) {
        const summary = `${local.name}. Book online, or call the team to talk through the right package for your ${nounOf(cat)}.`;
        await readJson(await fetch(`${wbase}/content/services/${local.id}`, send('PUT', { category: cat, summary, published: true, version: 0, serviceVersion: local.version })));
        n += 1; setAuto((a) => (a ? { ...a, done: n } : a));
      }
      setNotice(`${n} service${n === 1 ? '' : 's'} added to their website categories.`);
      setAuto(null);
    } catch (e) { setError((e instanceof Error ? e.message : 'The assignment stopped.') + (n ? ` ${n} were added before this.` : '')); }
    finally { setBusy(false); await load().catch(() => undefined); }
  }
  async function loadWebsite(s: Service) {
    if (!wbase || s.kind !== 'main') { setWeb({ state: 'idle', cat: '', summary: '' }); return; }
    setWeb({ state: 'loading', cat: '', summary: '' });
    try {
      const { local, pub, site } = await websiteLookup(s.id);
      if (!site) setWeb({ state: 'nosite', cat: '', summary: '' });
      else if (!local) setWeb({ state: 'missing', cat: '', summary: '' });
      else setWeb({ state: 'ready', local, pub, cat: pub?.published ? pub.category : '', summary: pub?.summary ?? '', star: !!pub?.starred });
    } catch { setWeb({ state: 'idle', cat: '', summary: '' }); }
  }
  // After the service itself is saved: put it on (or take it off) its website category.
  async function applyWebsite(id: string) {
    if (web.state !== 'ready') return;
    const was = web.pub?.published ? web.pub.category : '';
    if (web.cat === was && !!web.star === !!web.pub?.starred && (web.summary.trim() === (web.pub?.summary ?? '').trim() || !web.cat)) return;
    const { local, pub, site } = await websiteLookup(id);
    if (!site) throw new Error('Set up the website entry under Website images first.');
    if (!local) throw new Error('This service is not switched on at the selected franchise, so it cannot be placed on its website yet.');
    const category = web.cat || pub?.category || '';
    if (!category) return;
    const summary = web.summary.trim() || pub?.summary || websiteCategories.find((x) => x.key === category)?.summary || local.name;
    await readJson(await fetch(`${wbase}/content/services/${local.id}`, send('PUT', { category, summary, published: web.cat !== '', starred: !!web.star && web.cat !== '', version: pub?.version ?? 0, serviceVersion: local.version })));
  }
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
      if (wbase && canManage) {
        try {
          const [locals, content] = await Promise.all([fetch(wbase + '/services', { cache: 'no-store' }).then(readJson), fetch(wbase + '/content', { cache: 'no-store' }).then(readJson)]);
          const pubs = new Map<string, { category: string; published: boolean }>((content.services as Pub[]).filter((x) => x.version > 0).map((x) => [x.serviceId, x]));
          const tags: Record<string, { cat: string; published: boolean }> = {};
          for (const l of locals.items as Service[]) { const p = pubs.get(l.id); if (p && !l.archived) tags[mode === 'franchise' ? l.id : l.centralServiceId ?? ''] = { cat: p.category, published: p.published }; }
          setWebTags(tags);
        } catch { setWebTags({}); }
      }
      if (mode === 'franchise' && canManage) {
        const r = await fetch(`/api/v1/organizations/${organizationId}/central-services`, { cache: 'no-store' });
        setCentral(r.ok ? (await r.json()).items.filter((s: Service) => !s.archived) : []);
      }
    } catch (e) {
      setItems(null);
      setError(e instanceof Error ? e.message : 'The catalogue could not be loaded.');
    }
  }, [listUrl, mode, canManage, organizationId, wbase]);
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
  // An add-on is one record offered with several main services: it is listed under each of them.
  const parentsOf = (s: Service) => s.parentIds && s.parentIds.length ? s.parentIds : s.parentId ? [s.parentId] : [];
  const addonsOf = (id: string) => visible.filter((s) => s.kind === 'addon' && parentsOf(s).includes(id));
  const orphans = visible.filter((s) => s.kind === 'addon' && !parentsOf(s).some((p) => mains.some((m) => m.id === p)));
  const nameOf = (id: string | null) => all.find((s) => s.id === id)?.name ?? 'a main service';
  const mainsInCategory = (key: string) => all.filter((s) => !s.archived && s.kind === 'main' && webTags[s.id]?.published && webTags[s.id].cat === key).map((s) => s.id);

  function open(next: Service | 'new') {
    setEditing(next);
    setError('');
    setNotice('');
    setName(next === 'new' ? '' : next.name);
    setKind(next === 'new' ? 'main' : next.kind);
    setParentId(next === 'new' ? '' : next.parentId ?? '');
    setParentIds(next === 'new' || next.kind !== 'addon' ? [] : parentsOf(next));
    setPrice(next === 'new' || next.priceCents == null ? '' : (next.priceCents / 100).toFixed(2));
    const u = next === 'new' || next.durationMinutes == null ? 'minutes' : bestUnit(next.durationMinutes);
    setUnit(u);
    setDuration(next === 'new' || next.durationMinutes == null ? '' : String(next.durationMinutes / UNITS[u]));
    if (next === 'new') setWeb({ state: 'idle', cat: '', summary: '' }); else void loadWebsite(next);
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
        await readJson(await fetch(was === 'new' || !was ? listUrl : `${listUrl}/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', { ...fields, parentIds, ...(was && was !== 'new' ? { version: was.version } : {}) })));
      } else {
        await readJson(await fetch(was === 'new' || !was ? listUrl : `${listUrl}/${was.id}`, send(was === 'new' || !was ? 'POST' : 'PUT', { ...fields, parentId: null, ...(was && was !== 'new' ? { version: was.version } : {}) })));
      }
      if (was && was !== 'new') await applyWebsite(was.id);
      setEditing(null);
      return `Service “${name.trim()}” saved.`;
    });
  };
  const pickedItems = (): Service[] => (items ?? []).filter((x) => picked.has(x.id) && !x.archived);
  const applyBulk = () => {
    if (!bulk) return;
    const chosen = pickedItems();
    void run(async () => {
      let changed = 0;
      for (const sv of chosen) {
        const next: Record<string, unknown> = { name: sv.name, kind: sv.kind, parentId: sv.kind === 'addon' ? null : sv.parentId, parentIds: sv.kind === 'addon' ? parentsOf(sv) : undefined, version: sv.version, durationMinutes: sv.durationMinutes ?? null };
        if (mode === 'central') {
          let cents = sv.priceCents ?? null;
          const v = Number(bulk.price);
          if (bulk.priceMode === 'set') cents = Math.round(v * 100);
          else if (bulk.priceMode === 'up' && cents != null) cents = Math.round(cents * (1 + v / 100));
          else if (bulk.priceMode === 'down' && cents != null) cents = Math.max(0, Math.round(cents * (1 - v / 100)));
          next.priceCents = cents;
        }
        if (bulk.timeMode === 'set' && sv.kind === 'main') next.durationMinutes = Math.round(Number(bulk.time) * UNITS[bulk.unit]);
        await readJson(await fetch(`${listUrl}/${sv.id}`, send('PUT', next)));
        changed += 1;
      }
      setBulk(null); setPicked(new Set());
      return `${changed} service${changed === 1 ? '' : 's'} updated.`;
    });
  };
  const lineOf = (sv: Service) => [sv.name, ...(mode === 'central' ? [sv.priceCents == null ? '' : (sv.priceCents / 100).toFixed(2)] : []), sv.kind === 'main' && sv.durationMinutes != null ? String(sv.durationMinutes) : ''].join(' | ');
  const openText = () => {
    const rows = (picked.size ? pickedItems() : visible.filter((x) => !x.archived));
    setTextEdit({ rows, text: rows.map(lineOf).join('\n'), problems: [] });
  };
  const applyText = () => {
    if (!textEdit) return;
    const lines = textEdit.text.split('\n').map((l) => l.trim()).filter(Boolean);
    const problems: string[] = [];
    if (lines.length !== textEdit.rows.length) problems.push(`There are ${lines.length} lines but ${textEdit.rows.length} services. Keep one line per service, in the same order.`);
    const changes: { sv: Service; name: string; cents: number | null; minutes: number | null }[] = [];
    if (!problems.length) lines.forEach((line, i) => {
      const sv = textEdit.rows[i];
      const parts = line.split('|').map((x) => x.trim());
      const name = parts[0] ?? '';
      const priceTxt = mode === 'central' ? (parts[1] ?? '') : '';
      const minTxt = (mode === 'central' ? parts[2] : parts[1]) ?? '';
      if (name.length < 2 || name.length > 120) problems.push(`Line ${i + 1}: the name needs 2 to 120 characters.`);
      const cents = mode === 'central' ? (priceTxt === '' ? null : Math.round(Number(priceTxt.replace(/[$,\s]/g, '')) * 100)) : sv.priceCents ?? null;
      if (mode === 'central' && priceTxt !== '' && !(cents != null && cents >= 0)) problems.push(`Line ${i + 1}: the price is not a number.`);
      const minutes = sv.kind === 'main' ? (minTxt === '' ? null : Math.round(Number(minTxt))) : sv.durationMinutes ?? null;
      if (sv.kind === 'main' && minTxt !== '' && !(minutes != null && minutes > 0)) problems.push(`Line ${i + 1}: the minutes are not a number.`);
      if (name !== sv.name || cents !== (sv.priceCents ?? null) || minutes !== (sv.durationMinutes ?? null)) changes.push({ sv, name, cents, minutes });
    });
    if (problems.length) { setTextEdit({ ...textEdit, problems }); return; }
    if (!changes.length) { setTextEdit({ ...textEdit, problems: ['Nothing was changed.'] }); return; }
    void run(async () => {
      for (const { sv, name, cents, minutes } of changes) {
        const next: Record<string, unknown> = { name, kind: sv.kind, parentId: sv.kind === 'addon' ? null : sv.parentId, parentIds: sv.kind === 'addon' ? parentsOf(sv) : undefined, version: sv.version, durationMinutes: minutes };
        if (mode === 'central') next.priceCents = cents;
        await readJson(await fetch(`${listUrl}/${sv.id}`, send('PUT', next)));
      }
      setTextEdit(null); setPicked(new Set());
      return `${changes.length} service${changes.length === 1 ? '' : 's'} updated.`;
    });
  };
  const archiveBulk = () => {
    const chosen = pickedItems();
    void run(async () => {
      for (const sv of chosen) await readJson(await fetch(`${listUrl}/${sv.id}/archive`, send('POST', { version: sv.version })));
      setPicked(new Set());
      return `${chosen.length} service${chosen.length === 1 ? '' : 's'} archived.`;
    });
  };
  const restore = (s: Service) => run(async () => { await readJson(await fetch(`${listUrl}/${s.id}/restore`, send('POST', { version: s.version }))); return `“${s.name}” restored.`; });
  const archive = (s: Service) => run(async () => { await readJson(await fetch(`${listUrl}/${s.id}/archive`, send('POST', { version: s.version }))); return `“${s.name}” archived.`; });
  const del = (s: Service) => run(async () => { const b = await readJson(await fetch(`${listUrl}/${s.id}?version=${s.version}`, send('DELETE'))); return b.message ?? `“${s.name}” removed.`; });
  const duplicate = (s: Service) => run(async () => {
    const copy = `${s.name} (copy)`.slice(0, 120);
    await readJson(await fetch(listUrl, send('POST', { name: copy, kind: s.kind, parentIds: s.kind === 'addon' ? parentsOf(s) : undefined, ...(mode === 'central' ? { priceCents: s.priceCents ?? null } : {}), durationMinutes: s.durationMinutes ?? null })));
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
      <strong className={styles.svcName} title={s.name}>{canManage && !s.archived && <input type="checkbox" aria-label={`Select ${s.name}`} checked={picked.has(s.id)} onChange={() => togglePick(s.id)} style={{ marginRight: 10, verticalAlign: 'middle' }} />}{s.name}{webTags[s.id] && s.kind === 'main' && <small style={{ marginLeft: 10, padding: '2px 9px', borderRadius: 99, fontSize: 12, fontWeight: 500, background: 'rgba(227,23,43,.12)', color: 'inherit', opacity: webTags[s.id].published ? 1 : .6, whiteSpace: 'nowrap' }}>{webTags[s.id].published ? 'On website: ' + catTitle(webTags[s.id].cat) : 'Hidden on website'}</small>}</strong>
      <span className={styles.svcCell}>{s.archived ? 'Archived' : s.kind === 'addon' ? 'Add-on' : 'Main'}</span>
      <span className={styles.svcCell}>{s.priceCents != null ? money(s.priceCents) : 'No price'}</span>
      <span className={styles.svcCell}>{s.durationMinutes != null ? minutesText(s.durationMinutes) : 'No time'}</span>
      {canManage && s.archived && (
        <div className={styles.svcActions}>
          <button type="button" className="secondary" disabled={busy} onClick={() => void restore(s)}>Restore</button>
          <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'delete', item: s })}>Delete</button>
        </div>
      )}
      {(canManage || canSlots) && !s.archived && (
        <div className={styles.svcActions}>
          {canManage && (mode === 'central' || !s.centralServiceId) && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => open(s)}>Edit</button>}
          {(mode === 'franchise' || !!locationBase) && s.kind === 'main' && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => void openSlots(s)}>Slots</button>}
          {canManage && (mode === 'central' || !s.centralServiceId) && <button type="button" className="secondary" disabled={busy || !!editing} onClick={() => void duplicate(s)}>Duplicate</button>}
          {canManage && <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'archive', item: s })}>Archive</button>}
          {canManage && <button type="button" className="secondary" disabled={busy} onClick={() => setTarget({ action: 'delete', item: s })}>Delete</button>}
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
        {canManage && !editing && wbase && <button type="button" className="secondary" disabled={busy} onClick={() => void previewAuto()}>Assign website categories</button>}
        {canManage && !editing && <button type="button" className="primary" onClick={() => { fold(false); open('new'); }}>New service</button>}
      </div>
      <Collapse open={!folded} className={styles.stack}>

      {error && !editing && <p role="alert" className="field-error">{error}</p>}
      {notice && <p role="status" className={styles.status}>{notice}</p>}

      {auto && (
        <div className={styles.resource} role="region" aria-label="Assign website categories">
          <h3 style={{ margin: 0 }}>Assign website categories</h3>
          <p className={styles.personSub}>{auto.todo.length} main service{auto.todo.length === 1 ? '' : 's'} not on the website yet will go into the category their name says, shown on the website with a plain description you can edit afterwards. Names and prices are not touched.</p>
          <ul style={{ margin: '8px 0', paddingLeft: 18 }}>
            {websiteCategories.map((x) => { const k = auto.todo.filter((t) => t.cat === x.key).length; return k ? <li key={x.key}>{x.title}: {k}</li> : null; })}
          </ul>
          {auto.skipped.length > 0 && <p className={styles.personSub}>Left as they are (no matching website category): {auto.skipped.join('; ')}. Set a category for these by editing them.</p>}
          <div className={styles.rowBetween}>
            <button type="button" className="primary" disabled={busy || auto.todo.length === 0} onClick={() => void applyAuto()}>{busy ? `Adding… ${auto.done} of ${auto.todo.length}` : `Add ${auto.todo.length} to the website`}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setAuto(null)}>Cancel</button>
          </div>
        </div>
      )}

      {canManage && picked.size > 0 && !editing && (
        <div className={styles.resource} role="region" aria-label="Selected services">
          <div className={styles.rowBetween}>
            <strong>{picked.size} selected</strong>
            <span className={styles.svcActions}>
              <button type="button" className="primary" disabled={busy} onClick={openText}>Edit as text</button>
              <button type="button" className="secondary" disabled={busy} onClick={() => setBulk({ priceMode: 'keep', price: '', timeMode: 'keep', time: '', unit: 'minutes' })}>Set price or time for all</button>
              <button type="button" className="secondary" disabled={busy} onClick={archiveBulk}>Archive selected</button>
              <button type="button" className="secondary" disabled={busy} onClick={() => setPicked(new Set())}>Clear</button>
            </span>
          </div>
          {bulk && (
            <div style={{ display: 'grid', gap: 12, marginTop: 12 }}>
              {mode === 'central' && (
                <div className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
                  <label className="form-field">Price
                    <Select value={bulk.priceMode} onChange={(e) => setBulk({ ...bulk, priceMode: e.target.value as 'keep' | 'set' | 'up' | 'down' })} disabled={busy}>
                      <option value="keep">Leave as it is</option><option value="set">Set to an amount (CAD)</option><option value="up">Raise by a percent</option><option value="down">Lower by a percent</option>
                    </Select>
                  </label>
                  {bulk.priceMode !== 'keep' && <label className="form-field" style={{ maxWidth: 200 }}>{bulk.priceMode === 'set' ? 'Amount before tax' : 'Percent'}<NumberField inputMode="decimal" min="0" step="0.01" value={bulk.price} onChange={(e) => setBulk({ ...bulk, price: e.target.value })} disabled={busy} /></label>}
                </div>
              )}
              <div className={styles.inlineForm} style={{ alignItems: 'flex-end' }}>
                <label className="form-field">Time to complete (main services)
                  <Select value={bulk.timeMode} onChange={(e) => setBulk({ ...bulk, timeMode: e.target.value as 'keep' | 'set' })} disabled={busy}>
                    <option value="keep">Leave as it is</option><option value="set">Set to</option>
                  </Select>
                </label>
                {bulk.timeMode === 'set' && (
                  <span className={styles.durationRow}>
                    <NumberField inputMode="decimal" min="0.5" step="0.5" value={bulk.time} onChange={(e) => setBulk({ ...bulk, time: e.target.value })} disabled={busy} />
                    <Select value={bulk.unit} onChange={(e) => setBulk({ ...bulk, unit: e.target.value as Unit })} disabled={busy} aria-label="Time unit"><option value="minutes">Minutes</option><option value="hours">Hours</option><option value="days">Days</option></Select>
                  </span>
                )}
              </div>
              <div className={styles.svcActions}>
                <button type="button" className="primary" disabled={busy || (bulk.priceMode === 'keep' && bulk.timeMode === 'keep') || (bulk.priceMode !== 'keep' && !(Number(bulk.price) >= 0 && bulk.price.trim() !== '')) || (bulk.timeMode === 'set' && !(Number(bulk.time) > 0))} onClick={applyBulk}>{busy ? 'Saving…' : `Apply to ${picked.size}`}</button>
                <button type="button" className="secondary" disabled={busy} onClick={() => setBulk(null)}>Cancel</button>
              </div>
              <p className={styles.footnote}>Each service keeps a revision, so the earlier price or time stays on record. Add-ons keep their own time.</p>
            </div>
          )}
        </div>
      )}

      {canManage && textEdit && !editing && (
        <div className={styles.resource} role="region" aria-label="Edit services as text">
          <h3>Edit {textEdit.rows.length} service{textEdit.rows.length === 1 ? '' : 's'} as text</h3>
          <p className={styles.footnote}>One service per line: {mode === 'central' ? 'Name | Price in CAD before tax | Minutes' : 'Name | Minutes'}. Change any words or numbers, keep the same lines in the same order, then apply. Add-ons have no minutes.</p>
          <textarea className={styles.textBulk} value={textEdit.text} rows={Math.min(24, textEdit.rows.length + 2)} spellCheck={false} aria-label="Services as text" disabled={busy} onChange={(e) => setTextEdit({ ...textEdit, text: e.target.value, problems: [] })} />
          {textEdit.problems.map((m) => <p key={m} role="alert" className="field-error">{m}</p>)}
          <div className={styles.svcActions}>
            <button type="button" className="primary" disabled={busy} onClick={applyText}>{busy ? 'Saving…' : 'Apply changes'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => setTextEdit(null)}>Cancel</button>
          </div>
          <p className={styles.footnote}>Each service keeps a revision, so the earlier name, price or time stays on record.</p>
        </div>
      )}

      {editing && (
        <FormPopup label={editing === 'new' ? 'New service' : `Edit ${editing.name}`} onClose={() => { if (!busy) { setEditing(null); setError(''); } }}>
        <form onSubmit={save}>
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
                  <NumberField inputMode="decimal" min={unit === 'minutes' ? '1' : '0.5'} step={unit === 'minutes' ? '5' : '0.5'} placeholder="Not set" value={duration} onChange={(e) => setDuration(e.target.value)} disabled={busy} />
                  <Select value={unit} onChange={(e) => setUnit(e.target.value as Unit)} disabled={busy} aria-label="Time unit">
                    <option value="minutes">Minutes</option><option value="hours">Hours</option><option value="days">Days</option>
                  </Select>
                </span>
              </label>
            )}
            {kind === 'addon' && (
              <fieldset className={styles.parentPick}>
                <legend>Offered with these categories and main services</legend>
                {/* Ticking a category links the add-on to every package on that page of the website; unticking removes them. */}
                {websiteCategories.some((c) => mainsInCategory(c.key).length > 0) && <div className={styles.catPick}>
                  {websiteCategories.map((c) => {
                    const ids = mainsInCategory(c.key);
                    if (!ids.length) return null;
                    const on = ids.filter((id) => parentIds.includes(id)).length;
                    return (
                      <label key={c.key} className={styles.parentOpt}>
                        <input type="checkbox" checked={on === ids.length} ref={(el) => { if (el) el.indeterminate = on > 0 && on < ids.length; }} disabled={busy} onChange={(e) => setParentIds(e.target.checked ? [...new Set([...parentIds, ...ids])] : parentIds.filter((x) => !ids.includes(x)))} />
                        <span>{c.short}</span>
                      </label>
                    );
                  })}
                </div>}
                {all.filter((x) => !x.archived && x.kind === 'main').map((m) => {
                  const locked = false;
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
          {kind === 'main' && editing !== 'new' && web.state !== 'idle' && (
            <fieldset className={styles.parentPick} style={{ maxHeight: 'none', overflow: 'visible', marginTop: 12 }}>
              <legend>On the website</legend>
              {web.state === 'loading' && <p className={styles.personSub}>Loading the website settings…</p>}
              {web.state === 'nosite' && <p className={styles.personSub}>Set up the website entry under Website images first, then choose a category here.</p>}
              {web.state === 'missing' && <p className={styles.personSub}>This service is not switched on at the selected franchise, so it cannot be placed on the website yet.</p>}
              {web.state === 'ready' && (
                <>
                  <label className="form-field">Website category
                    <Select value={web.cat} disabled={busy} onChange={(e) => { const v = e.target.value; setWeb({ ...web, cat: v, summary: web.summary.trim() || !v ? web.summary : websiteCategories.find((x) => x.key === v)?.summary ?? '' }); }}>
                      <option value="">Not on the website</option>
                      {websiteCategories.map((x) => <option key={x.key} value={x.key}>{x.title}</option>)}
                    </Select>
                  </label>
                  {web.cat && (
                    <label className="form-field">Description on the website
                      <textarea rows={3} maxLength={2000} value={web.summary} disabled={busy} onChange={(e) => setWeb({ ...web, summary: e.target.value })} />
                    </label>
                  )}
                  {web.cat && (
                    <label className={styles.parentOpt}>
                      <input type="checkbox" checked={!!web.star} disabled={busy} onChange={(e) => setWeb({ ...web, star: e.target.checked })} />
                      <span>Bestseller for {websiteCategories.find((x) => x.key === web.cat)?.short ?? 'this category'}. It shows first in its tab with a Bestseller label. Several packages can be bestsellers, for example one per size or vehicle.</span>
                    </label>
                  )}
                  <p className={styles.footnote}>Saved with the service. The name and price on the website always come from this screen.</p>
                </>
              )}
            </fieldset>
          )}
          {error && <p role="alert" className="field-error">{error}</p>}
          <div className={styles.rowBetween}>
            <button className="primary" disabled={busy || name.trim().length < 2 || (kind === 'addon' && parentIds.length === 0) || (price.trim() !== '' && !(Number(price) >= 0)) || (durationMinutes !== null && !(durationMinutes >= 1 && durationMinutes <= 10080))}>{busy ? 'Saving…' : editing === 'new' ? 'Add service' : 'Save revision'}</button>
            <button type="button" className="secondary" disabled={busy} onClick={() => { setEditing(null); setError(''); }}>Cancel</button>
          </div>
          <p className={styles.footnote}>{mode === 'central' ? 'The price is entered before tax; each franchise adds its own taxes. ' : 'Prices come from the central catalogue. '}Each save keeps a revision; past work orders and invoices keep the name they were created with.</p>
        </form>
        </FormPopup>
      )}

      {/* The slot settings open as a popup over the list, like the service form. */}
      {slotsFor && <FormPopup label={`Slots for ${slotsFor.name}`} width={680} onClose={() => setSlotsFor(null)}><ServiceSlots base={slotsBase} service={slotsFor} onClose={() => setSlotsFor(null)} /></FormPopup>}

      {mode === 'franchise' && canManage && available.length > 0 && (
        <section className={styles.resource}>
          <h3>Available from the central catalogue</h3>
          <ul className={styles.svcList}>
            {available.map((c) => {
              const parentLinked = c.kind !== 'addon' || (c.parentIds ?? (c.parentId ? [c.parentId] : [])).some((p) => all.some((s) => s.centralServiceId === p));
              return (
                <li key={c.id} className={styles.svc}>
                  <div className={styles.svcMain}><strong>{c.name}</strong><span className={styles.personSub}>{c.kind === 'addon' ? `Add-on to ${(c.parentIds ?? (c.parentId ? [c.parentId] : [])).map((p) => central.find((x) => x.id === p)?.name ?? 'a main service').join(', ')}` : 'Main service'}</span></div>
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
          {canManage && !editing && <button type="button" className="secondary" disabled={busy || !visible.some((x) => !x.archived)} onClick={openText}>{picked.size ? `Edit ${picked.size} as text` : 'Edit as text'}</button>}
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
