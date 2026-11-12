'use client';

import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode, type RefObject } from 'react';
import { createPortal } from 'react-dom';
import gsap from 'gsap';
import { useGSAP } from '@gsap/react';
import base from './GlassSelect.module.css';
import styles from './Pickers.module.css';
import type { SelectChange } from './GlassSelect';

gsap.registerPlugin(useGSAP);

/* Modern replacements for the browser's own date, time, date-and-time and colour inputs. Same props as the native inputs
   (value, onChange(e.target.value), min, max, required, disabled, name, aria-label) so they drop in where an <input> was.
   The menu is the same glass surface as the dropdowns: it opens out of the field (or rises as a sheet on phones), the days
   settle in, the selected day is a pill that glides, months slide, and everything works from the keyboard. */

const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
const pad = (n: number) => String(n).padStart(2, '0');
const toIso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const fromIso = (s: string) => { const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s); return m ? new Date(+m[1], +m[2] - 1, +m[3]) : null; };
const addDays = (d: Date, n: number) => { const x = new Date(d); x.setDate(x.getDate() + n); return x; };
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

type FieldProps = {
  value?: string; defaultValue?: string; onChange?: (e: SelectChange) => void; disabled?: boolean; required?: boolean;
  id?: string; name?: string; className?: string; style?: CSSProperties; placeholder?: string; autoFocus?: boolean;
  'aria-label'?: string; 'aria-labelledby'?: string; 'aria-describedby'?: string; title?: string;
};
type Pos = { sheet: boolean; above: boolean; left: number; top?: number; bottom?: number; width: number; maxH: number };

/* ---------- the shared glass popover ---------- */
function Popover({ open, anchorRef, onClose, width, label, children }: { open: boolean; anchorRef: RefObject<HTMLElement | null>; onClose: (restoreFocus: boolean) => void; width: number; label: string; children: (ready: boolean) => ReactNode }) {
  const [mounted, setMounted] = useState(false);
  const [pos, setPos] = useState<Pos | null>(null);
  const popRef = useRef<HTMLDivElement>(null);
  const scrimRef = useRef<HTMLDivElement>(null);

  useEffect(() => { if (open) setMounted(true); }, [open]);
  useEffect(() => {
    if (open || !mounted) return;
    const pop = popRef.current;
    const done = () => { setMounted(false); setPos(null); };
    if (!pop || reduced()) { done(); return; }
    if (scrimRef.current) gsap.to(scrimRef.current, { autoAlpha: 0, duration: 0.16 });
    const t = gsap.to(pop, pos?.sheet ? { yPercent: 100, duration: 0.22, ease: 'power2.in', onComplete: done } : { autoAlpha: 0, scale: 0.96, y: pos?.above ? 6 : -6, duration: 0.14, ease: 'power2.in', onComplete: done });
    return () => { t.kill(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, mounted]);

  useLayoutEffect(() => {
    if (!mounted) return;
    const place = () => {
      const a = anchorRef.current, pop = popRef.current;
      if (!a || !pop) return;
      const r = a.getBoundingClientRect(), vw = window.innerWidth, vh = window.innerHeight;
      let next: Pos;
      if (vw < 640) next = { sheet: true, above: false, left: 0, width: vw, maxH: Math.min(vh * 0.88, 680) };
      else {
        const w = Math.min(width, vw - 24), left = Math.min(Math.max(12, r.left), vw - w - 12);
        const natural = pop.scrollHeight + 4, below = vh - r.bottom - 14, above = r.top - 14, up = natural > below && above > below;
        const maxH = Math.max(200, Math.min(natural, up ? above : below));
        next = up ? { sheet: false, above: true, left, bottom: vh - r.top + 8, width: w, maxH } : { sheet: false, above: false, left, top: r.bottom + 8, width: w, maxH };
      }
      setPos((p) => (p && p.sheet === next.sheet && p.above === next.above && p.left === next.left && p.top === next.top && p.bottom === next.bottom && p.width === next.width && p.maxH === next.maxH ? p : next));
    };
    place();
    const onScroll = (e: Event) => { if (!popRef.current?.contains(e.target as Node)) place(); };
    window.addEventListener('resize', place);
    window.addEventListener('scroll', onScroll, true);
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', onScroll, true); };
  }, [mounted, anchorRef, width]);

  useEffect(() => {
    if (!mounted || !open) return;
    const down = (e: PointerEvent) => { const n = e.target as Node; if (popRef.current?.contains(n) || anchorRef.current?.contains(n)) return; onClose(false); };
    const key = (e: globalThis.KeyboardEvent) => { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); onClose(true); } };
    document.addEventListener('pointerdown', down, true);
    document.addEventListener('keydown', key, true);
    return () => { document.removeEventListener('pointerdown', down, true); document.removeEventListener('keydown', key, true); };
  }, [mounted, open, onClose, anchorRef]);

  useGSAP(() => {
    const pop = popRef.current;
    if (!pop || !pos) return;
    if (reduced()) { gsap.set(pop, { autoAlpha: 1 }); return; }
    const tl = gsap.timeline();
    if (pos.sheet) tl.fromTo(scrimRef.current, { autoAlpha: 0 }, { autoAlpha: 1, duration: 0.2 }, 0).fromTo(pop, { autoAlpha: 1, yPercent: 100 }, { yPercent: 0, duration: 0.36, ease: 'power3.out' }, 0);
    else tl.fromTo(pop, { autoAlpha: 0, scale: 0.94, y: pos.above ? 10 : -10, transformOrigin: pos.above ? '50% 100%' : '50% 0%' }, { autoAlpha: 1, scale: 1, y: 0, duration: 0.26, ease: 'power3.out' }, 0);
    const items = Array.from(pop.querySelectorAll<HTMLElement>('[data-pop-item]'));
    if (items.length) tl.fromTo(items, { autoAlpha: 0, y: 6 }, { autoAlpha: 1, y: 0, duration: 0.3, ease: 'power2.out', stagger: { amount: 0.22 }, clearProps: 'opacity,visibility,transform' }, 0.08);
    return () => { tl.kill(); };
  }, { dependencies: [!!pos] });

  if (!mounted) return null;
  return createPortal(
    <>
      {pos?.sheet && <div ref={scrimRef} className={base.scrim} onPointerDown={() => onClose(false)} />}
      <div ref={popRef} role="dialog" aria-label={label} className={`${base.pop} ${styles.popBody} ${pos?.sheet ? base.sheet : ''} ${pos ? '' : base.measuring}`} style={pos ? { left: pos.left, top: pos.top, bottom: pos.bottom, width: pos.width, maxHeight: pos.maxH } : { width }}>
        {pos?.sheet && <span className={base.grab} aria-hidden="true" />}
        {children(!!pos)}
      </div>
    </>,
    anchorRef.current?.closest('dialog') ?? document.body,
  );
}

const CalendarIcon = () => (<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true"><rect x="2.5" y="3.5" width="13" height="12" rx="3" stroke="currentColor" strokeWidth="1.6" /><path d="M2.5 7.5h13M6 2v3M12 2v3" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>);
const ClockIcon = () => (<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true"><circle cx="9" cy="9" r="6.5" stroke="currentColor" strokeWidth="1.6" /><path d="M9 5.2V9l2.6 1.7" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" /></svg>);
const Chevron = ({ dir }: { dir: 'left' | 'right' | 'down' }) => (<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d={dir === 'left' ? 'M10 3.5L5.5 8l4.5 4.5' : dir === 'right' ? 'M6 3.5L10.5 8 6 12.5' : 'M3.5 6l4.5 4.5L12.5 6'} stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>);

/* A field that holds a value, controlled (value) or not (defaultValue), and tells the parent like a native input does. */
function useField(value: string | undefined, defaultValue: string | undefined, name: string | undefined, onChange: FieldProps['onChange']) {
  const [inner, setInner] = useState(defaultValue ?? '');
  const current = value !== undefined ? value : inner;
  const set = (next: string) => { if (value === undefined) setInner(next); const target = { value: next, name: name ?? '' }; onChange?.({ target, currentTarget: target }); };
  return [current, set] as const;
}

function Trigger({ triggerRef, open, onToggle, onOpenKey, label, placeholder, icon, props }: { triggerRef: RefObject<HTMLButtonElement | null>; open: boolean; onToggle: () => void; onOpenKey: () => void; label: string; placeholder: string; icon: ReactNode; props: FieldProps }) {
  return (
    <button
      ref={triggerRef} id={props.id} type="button" title={props.title} disabled={props.disabled} aria-haspopup="dialog" aria-expanded={open}
      aria-label={props['aria-label']} aria-labelledby={props['aria-labelledby']} aria-describedby={props['aria-describedby']}
      className={`${base.trigger} ${open ? base.triggerOpen : ''} ${props.className ?? ''}`} style={{ ...props.style, paddingRight: 40 }}
      onClick={onToggle}
      onKeyDown={(e: KeyboardEvent<HTMLButtonElement>) => { if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); onOpenKey(); } }}
    >
      <span className={base.labelBox}><span className={`${base.label} ${label ? '' : base.placeholder}`}>{label || placeholder}</span></span>
      <span className={base.chev} aria-hidden="true" style={{ transform: 'none' }}>{icon}</span>
    </button>
  );
}

/* ================= date ================= */
type Mode = 'days' | 'months' | 'years';

function CalendarPanel({ value, min, max, ready, required, onPick, onClear }: { value: string; min?: string; max?: string; ready: boolean; required?: boolean; onPick: (iso: string) => void; onClear: () => void }) {
  const today = toIso(new Date());
  const selected = fromIso(value);
  const start = selected ?? fromIso(today)!;
  const [view, setView] = useState({ y: start.getFullYear(), m: start.getMonth() });
  const [mode, setMode] = useState<Mode>('days');
  const [focusIso, setFocusIso] = useState(value || today);
  const gridRef = useRef<HTMLDivElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const selRef = useRef<HTMLDivElement>(null);
  const dir = useRef(1);
  const firstView = useRef(true);
  const wantFocus = useRef(true);

  const out = (iso: string) => (!!min && iso < min) || (!!max && iso > max);
  const first = new Date(view.y, view.m, 1);
  const gridStart = addDays(first, -((first.getDay() + 6) % 7));
  const cells = Array.from({ length: 42 }, (_, i) => addDays(gridStart, i));

  const shift = (months: number) => {
    dir.current = months >= 0 ? 1 : -1;
    const d = new Date(view.y, view.m + months, 1);
    setView({ y: d.getFullYear(), m: d.getMonth() });
  };

  // The selected day is a pill that glides to its cell.
  useLayoutEffect(() => {
    const sel = selRef.current, grid = gridRef.current;
    if (!sel || !grid || mode !== 'days') return;
    const btn = value ? grid.querySelector<HTMLElement>(`[data-iso="${value}"]`) : null;
    if (!btn) { gsap.set(sel, { autoAlpha: 0 }); return; }
    const vars = { x: btn.offsetLeft, y: btn.offsetTop, width: btn.offsetWidth, height: btn.offsetHeight, autoAlpha: 1 };
    if (!ready || reduced() || gsap.getProperty(sel, 'autoAlpha') === 0) gsap.set(sel, vars);
    else gsap.to(sel, { ...vars, duration: 0.4, ease: 'power3.out', overwrite: 'auto' });
  }, [value, view, mode, ready]);

  // Months slide in from the side you moved towards.
  useEffect(() => {
    if (firstView.current) { firstView.current = false; return; }
    if (!gridRef.current || reduced()) return;
    gsap.fromTo(gridRef.current, { x: dir.current * 28, autoAlpha: 0 }, { x: 0, autoAlpha: 1, duration: 0.34, ease: 'power3.out', clearProps: 'transform,opacity,visibility' });
  }, [view.y, view.m]);

  // Keyboard focus follows the focused day.
  useEffect(() => {
    if (!ready || mode !== 'days' || !wantFocus.current) return;
    gridRef.current?.querySelector<HTMLElement>(`[data-iso="${focusIso}"]`)?.focus({ preventScroll: true });
  }, [focusIso, ready, mode, view]);

  const moveFocus = (d: Date) => {
    const iso = toIso(d);
    wantFocus.current = true;
    setFocusIso(iso);
    if (d.getFullYear() !== view.y || d.getMonth() !== view.m) { dir.current = d > new Date(view.y, view.m, 1) ? 1 : -1; setView({ y: d.getFullYear(), m: d.getMonth() }); }
  };
  const onGridKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const cur = fromIso(focusIso) ?? new Date();
    const step: Record<string, () => Date | null> = {
      ArrowLeft: () => addDays(cur, -1), ArrowRight: () => addDays(cur, 1), ArrowUp: () => addDays(cur, -7), ArrowDown: () => addDays(cur, 7),
      Home: () => addDays(cur, -((cur.getDay() + 6) % 7)), End: () => addDays(cur, 6 - ((cur.getDay() + 6) % 7)),
      PageUp: () => new Date(cur.getFullYear() + (e.shiftKey ? -1 : 0), cur.getMonth() + (e.shiftKey ? 0 : -1), cur.getDate()),
      PageDown: () => new Date(cur.getFullYear() + (e.shiftKey ? 1 : 0), cur.getMonth() + (e.shiftKey ? 0 : 1), cur.getDate()),
    };
    const fn = step[e.key];
    if (fn) { e.preventDefault(); const d = fn(); if (d) moveFocus(d); }
  };

  const title = mode === 'days' ? `${MONTHS[view.m]} ${view.y}` : mode === 'months' ? String(view.y) : `${Math.floor(view.y / 12) * 12} – ${Math.floor(view.y / 12) * 12 + 11}`;
  const onPrev = () => (mode === 'days' ? shift(-1) : mode === 'months' ? setView({ ...view, y: view.y - 1 }) : setView({ ...view, y: view.y - 12 }));
  const onNext = () => (mode === 'days' ? shift(1) : mode === 'months' ? setView({ ...view, y: view.y + 1 }) : setView({ ...view, y: view.y + 12 }));

  return (
    <div className={styles.cal}>
      <div className={styles.calHead} data-pop-item="">
        <button type="button" className={styles.calTitle} aria-live="polite" onClick={() => setMode(mode === 'days' ? 'months' : mode === 'months' ? 'years' : 'days')} aria-label={`${title}. Change month or year`}>
          {title}<span className={`${styles.titleChev} ${mode !== 'days' ? styles.titleChevUp : ''}`}><Chevron dir="down" /></span>
        </button>
        <button type="button" className={styles.navBtn} aria-label="Previous" onClick={onPrev}><Chevron dir="left" /></button>
        <button type="button" className={styles.navBtn} aria-label="Next" onClick={onNext}><Chevron dir="right" /></button>
      </div>

      {mode === 'days' && <>
        <div className={styles.dow} aria-hidden="true" data-pop-item="">{['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'].map((d) => <span key={d}>{d}</span>)}</div>
        <div ref={wrapRef} className={styles.gridWrap}>
          <div ref={selRef} className={styles.sel} aria-hidden="true" />
          <div ref={gridRef} className={styles.grid} role="grid" aria-label={title} onKeyDown={onGridKey}>
            {cells.map((d) => {
              const iso = toIso(d), inMonth = d.getMonth() === view.m, isSel = iso === value;
              return (
                <button
                  key={iso} type="button" data-iso={iso} data-pop-item="" disabled={out(iso)} tabIndex={iso === focusIso ? 0 : -1} aria-selected={isSel} aria-current={iso === today ? 'date' : undefined}
                  aria-label={d.toLocaleDateString('en-CA', { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' })}
                  className={`${styles.day} ${inMonth ? '' : styles.dayOut} ${iso === today ? styles.dayToday : ''}`}
                  onFocus={() => { wantFocus.current = false; setFocusIso(iso); }}
                  onClick={() => onPick(iso)}
                >{d.getDate()}</button>
              );
            })}
          </div>
        </div>
      </>}

      {mode === 'months' && (
        <div className={styles.cells12}>
          {MONTHS.map((n, i) => <button key={n} type="button" data-pop-item="" aria-pressed={selected?.getFullYear() === view.y && selected.getMonth() === i} className={styles.cell12} onClick={() => { dir.current = 1; setView({ y: view.y, m: i }); setMode('days'); }}>{n.slice(0, 3)}</button>)}
        </div>
      )}
      {mode === 'years' && (
        <div className={styles.cells12}>
          {Array.from({ length: 12 }, (_, i) => Math.floor(view.y / 12) * 12 + i).map((y) => <button key={y} type="button" data-pop-item="" aria-pressed={selected?.getFullYear() === y} className={styles.cell12} onClick={() => { setView({ y, m: view.m }); setMode('months'); }}>{y}</button>)}
        </div>
      )}

      <div className={styles.footer} data-pop-item="">
        {required ? <span /> : <button type="button" className={styles.link} onClick={onClear}>Clear</button>}
        <button type="button" className={styles.link} disabled={out(today)} onClick={() => onPick(today)}>Today</button>
      </div>
    </div>
  );
}

export function DatePicker(props: FieldProps & { min?: string; max?: string }) {
  const { value, defaultValue, onChange, name, required, min, max } = props;
  const [current, set] = useField(value, defaultValue, name, onChange);
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const d = fromIso(current);
  const label = d ? d.toLocaleDateString('en-CA', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' }) : '';
  const close = (restore: boolean) => { setOpen(false); if (restore) trigger.current?.focus({ preventScroll: true }); };
  return (
    <div className={base.wrap} data-glass-select="" style={{ minWidth: 196 }}>
      <Trigger triggerRef={trigger} open={open} onToggle={() => !props.disabled && setOpen((o) => !o)} onOpenKey={() => !props.disabled && setOpen(true)} label={label} placeholder={props.placeholder ?? 'Choose a date'} icon={<CalendarIcon />} props={props} />
      {(required || name) && <input className={base.proxy} tabIndex={-1} aria-hidden="true" name={name} required={required} value={current} onChange={() => undefined} />}
      <Popover open={open} anchorRef={trigger} onClose={close} width={320} label="Choose a date">
        {(ready) => <CalendarPanel value={current} min={min} max={max} ready={ready} required={required} onPick={(iso) => { set(iso); close(true); }} onClear={() => { set(''); close(true); }} />}
      </Popover>
    </div>
  );
}

/* ================= time ================= */
type Parts = { h12: number; min: number; pm: boolean };
const parseTime = (v: string): Parts | null => { const m = /^(\d{1,2}):(\d{2})/.exec(v); if (!m) return null; const h = +m[1] % 24; return { h12: h % 12 === 0 ? 12 : h % 12, min: +m[2], pm: h >= 12 }; };
const buildTime = ({ h12, min, pm }: Parts) => `${pad((h12 % 12) + (pm ? 12 : 0))}:${pad(min)}`;
const fmtTime = (v: string) => { if (v === '24:00') return 'Midnight (end of day)'; const p = parseTime(v); return p ? `${p.h12}:${pad(p.min)} ${p.pm ? 'PM' : 'AM'}` : ''; };

function Column({ label, items, selected, onSelect, ready, onDone, onSide }: { label: string; items: { key: string; text: string }[]; selected: string; onSelect: (key: string) => void; ready: boolean; onDone: () => void; onSide: (dir: -1 | 1) => void }) {
  const colRef = useRef<HTMLDivElement>(null);
  const pillRef = useRef<HTMLDivElement>(null);
  const placed = useRef(false);
  useLayoutEffect(() => {
    const col = colRef.current, pill = pillRef.current;
    const btn = col?.querySelector<HTMLElement>(`[data-key="${CSS.escape(selected)}"]`);
    if (!col || !pill || !btn) { if (pill) gsap.set(pill, { autoAlpha: 0 }); return; }
    const vars = { y: btn.offsetTop, height: btn.offsetHeight, autoAlpha: 1 };
    if (!ready || !placed.current || reduced()) { gsap.set(pill, vars); placed.current = ready; col.scrollTop = btn.offsetTop - col.clientHeight / 2 + btn.offsetHeight / 2; }
    else { gsap.to(pill, { ...vars, duration: 0.32, ease: 'power3.out', overwrite: 'auto' }); col.scrollTo({ top: btn.offsetTop - col.clientHeight / 2 + btn.offsetHeight / 2, behavior: 'smooth' }); }
  }, [selected, ready, items.length]);
  const index = items.findIndex((i) => i.key === selected);
  const onKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const go = (i: number) => { const it = items[Math.max(0, Math.min(items.length - 1, i))]; if (it) onSelect(it.key); };
    if (e.key === 'ArrowDown') { e.preventDefault(); go(index + 1); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); go(index - 1); }
    else if (e.key === 'Home') { e.preventDefault(); go(0); }
    else if (e.key === 'End') { e.preventDefault(); go(items.length - 1); }
    else if (e.key === 'ArrowLeft') { e.preventDefault(); onSide(-1); }
    else if (e.key === 'ArrowRight') { e.preventDefault(); onSide(1); }
    else if (e.key === 'Enter') { e.preventDefault(); onDone(); }
  };
  return (
    <div ref={colRef} className={styles.col} role="listbox" aria-label={label} tabIndex={0} data-col="" onKeyDown={onKey}>
      <div ref={pillRef} className={styles.pill} aria-hidden="true" />
      {items.map((i) => <button key={i.key} type="button" tabIndex={-1} role="option" data-key={i.key} aria-selected={i.key === selected} className={styles.tcell} onClick={() => onSelect(i.key)}>{i.text}</button>)}
    </div>
  );
}

function TimePanel({ value, step, ready, required, allowEndOfDay, onChange, onClear, onDone }: { value: string; step: number; ready: boolean; required?: boolean; allowEndOfDay?: boolean; onChange: (v: string) => void; onClear: () => void; onDone: () => void }) {
  const p = parseTime(value === '24:00' ? '00:00' : value) ?? { h12: 9, min: 0, pm: false };
  const hours = Array.from({ length: 12 }, (_, i) => ({ key: String(i + 1), text: String(i + 1) }));
  const mins = Array.from({ length: Math.ceil(60 / step) }, (_, i) => i * step);
  if (!mins.includes(p.min)) mins.push(p.min), mins.sort((a, b) => a - b);
  const minutes = mins.map((m) => ({ key: String(m), text: pad(m) }));
  const periods = [{ key: 'AM', text: 'AM' }, { key: 'PM', text: 'PM' }];
  const root = useRef<HTMLDivElement>(null);
  const side = (from: number) => (dir: -1 | 1) => { const cols = root.current?.querySelectorAll<HTMLElement>('[data-col]'); cols?.[Math.max(0, Math.min(2, from + dir))]?.focus(); };
  return (
    <div className={styles.time} ref={root}>
      <div className={styles.colLabels} aria-hidden="true" data-pop-item=""><span>Hour</span><span>Minute</span><span /></div>
      <div className={styles.cols} data-pop-item="">
        <Column label="Hour" items={hours} selected={String(p.h12)} ready={ready} onSelect={(k) => onChange(buildTime({ ...p, h12: +k }))} onDone={onDone} onSide={side(0)} />
        <Column label="Minute" items={minutes} selected={String(p.min)} ready={ready} onSelect={(k) => onChange(buildTime({ ...p, min: +k }))} onDone={onDone} onSide={side(1)} />
        <Column label="AM or PM" items={periods} selected={p.pm ? 'PM' : 'AM'} ready={ready} onSelect={(k) => onChange(buildTime({ ...p, pm: k === 'PM' }))} onDone={onDone} onSide={side(2)} />
      </div>
      <div className={styles.footer} data-pop-item="">
        {required ? <span /> : <button type="button" className={styles.link} onClick={onClear}>Clear</button>}
        <span style={{ display: 'flex', gap: 4 }}>
          {allowEndOfDay && <button type="button" className={styles.link} onClick={() => { onChange('24:00'); onDone(); }}>End of day</button>}
          <button type="button" className={styles.linkStrong} onClick={onDone}>Done</button>
        </span>
      </div>
    </div>
  );
}

export function TimePicker(props: FieldProps & { step?: number; allowEndOfDay?: boolean }) {
  const { value, defaultValue, onChange, name, required, step = 5, allowEndOfDay } = props;
  const [current, set] = useField(value, defaultValue, name, onChange);
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const close = (restore: boolean) => { setOpen(false); if (restore) trigger.current?.focus({ preventScroll: true }); };
  return (
    <div className={base.wrap} data-glass-select="" style={{ minWidth: 138 }}>
      <Trigger triggerRef={trigger} open={open} onToggle={() => !props.disabled && setOpen((o) => !o)} onOpenKey={() => !props.disabled && setOpen(true)} label={fmtTime(current)} placeholder={props.placeholder ?? 'Choose a time'} icon={<ClockIcon />} props={props} />
      {(required || name) && <input className={base.proxy} tabIndex={-1} aria-hidden="true" name={name} required={required} value={current} onChange={() => undefined} />}
      <Popover open={open} anchorRef={trigger} onClose={close} width={292} label="Choose a time">
        {(ready) => <TimePanel value={current} step={step} ready={ready} required={required} allowEndOfDay={allowEndOfDay} onChange={set} onClear={() => { set(''); close(true); }} onDone={() => close(true)} />}
      </Popover>
    </div>
  );
}

/* ================= date and time together (replaces datetime-local) ================= */
export function DateTimePicker(props: FieldProps & { min?: string; max?: string; step?: number }) {
  const { value, defaultValue, onChange, name, required, min, max, step } = props;
  const [current, set] = useField(value, defaultValue, name, onChange);
  const [date, time] = current.split('T');
  const emit = (d: string, t: string) => set(d ? `${d}T${t || '09:00'}` : '');
  return (
    <div className={styles.dtRow}>
      <DatePicker className={props.className} style={props.style} disabled={props.disabled} required={required} value={date ?? ''} min={min?.slice(0, 10)} max={max?.slice(0, 10)} aria-label={props['aria-label'] ? `${props['aria-label']} date` : 'Date'} autoFocus={props.autoFocus} onChange={(e) => emit(e.target.value, time ?? '')} />
      <TimePicker className={props.className} style={props.style} disabled={props.disabled || !date} required={required} step={step} value={time ?? ''} aria-label={props['aria-label'] ? `${props['aria-label']} time` : 'Time'} onChange={(e) => emit(date ?? '', e.target.value)} />
      {name && <input type="hidden" name={name} value={current} />}
    </div>
  );
}

/* ================= colour ================= */
const SWATCHES = ['#fb1a2b', '#d10f1f', '#7a0710', '#111111', '#2a2e33', '#6b7280', '#1f5e8a', '#39e1ff', '#34d399', '#e2a021', '#8b5cf6', '#ec4899'];
export function ColorPicker(props: FieldProps) {
  const { value, defaultValue, onChange, name } = props;
  const [current, set] = useField(value, defaultValue ?? '#111111', name, onChange);
  const [open, setOpen] = useState(false);
  const [hex, setHex] = useState(current);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => setHex(current), [current]);
  const close = (restore: boolean) => { setOpen(false); if (restore) trigger.current?.focus({ preventScroll: true }); };
  const valid = /^#[0-9a-fA-F]{6}$/.test(hex);
  return (
    <div className={base.wrap} data-glass-select="" style={{ minWidth: 132 }}>
      <Trigger triggerRef={trigger} open={open} onToggle={() => !props.disabled && setOpen((o) => !o)} onOpenKey={() => !props.disabled && setOpen(true)} label={current.toUpperCase()} placeholder="Choose a colour" icon={<span className={styles.dot} style={{ background: current }} />} props={props} />
      <Popover open={open} anchorRef={trigger} onClose={close} width={264} label="Choose a colour">
        {() => (
          <div className={styles.color}>
            <div className={styles.swatches} data-pop-item="">
              {SWATCHES.map((c) => <button key={c} type="button" className={styles.sw} style={{ background: c }} aria-label={c} aria-pressed={c.toLowerCase() === current.toLowerCase()} onClick={() => { set(c); close(true); }} />)}
            </div>
            <label className={styles.hexRow} data-pop-item="">
              <span className={styles.dot} style={{ background: valid ? hex : current }} />
              <input className={styles.hexInput} value={hex} maxLength={7} spellCheck={false} aria-label="Hex colour" onChange={(e) => { const v = e.target.value.startsWith('#') ? e.target.value : '#' + e.target.value; setHex(v); if (/^#[0-9a-fA-F]{6}$/.test(v)) set(v.toLowerCase()); }} />
            </label>
            <div className={styles.footer} data-pop-item=""><span /><button type="button" className={styles.linkStrong} onClick={() => close(true)}>Done</button></div>
          </div>
        )}
      </Popover>
    </div>
  );
}
