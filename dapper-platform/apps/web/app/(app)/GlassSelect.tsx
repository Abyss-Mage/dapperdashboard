'use client';

import { Children, Fragment, isValidElement, useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import gsap from 'gsap';
import { useGSAP } from '@gsap/react';
import styles from './GlassSelect.module.css';

gsap.registerPlugin(useGSAP);

/* A drop-in replacement for the native <select>: same <option> / <optgroup> children, same value / onChange(e.target.value),
   but a glass menu that opens from the button, with GSAP motion:
   - the menu scales out from the button (and rises from the bottom as a sheet on phones),
   - options stagger in, and a single highlight pill glides between them (mouse and keyboard),
   - the options near the pointer swell slightly, like the macOS dock,
   - the chosen label slides in when it changes and the chevron turns.
   Keyboard: arrows, Home/End, Enter/Space, Esc, Tab and type-ahead. Motion is skipped for prefers-reduced-motion. */

type Opt = { value: string; label: string; disabled: boolean; group?: string };
type Pos = { sheet: boolean; above: boolean; left: number; top?: number; bottom?: number; width: number; maxH: number };
export type SelectChange = { target: { value: string; name: string }; currentTarget: { value: string; name: string } };
type Props = {
  value?: string | number;
  defaultValue?: string | number;
  onChange?: (e: SelectChange) => void;
  children?: ReactNode;
  disabled?: boolean;
  required?: boolean;
  id?: string;
  name?: string;
  className?: string;
  style?: CSSProperties;
  title?: string;
  autoFocus?: boolean;
  autoComplete?: string;
  'aria-label'?: string;
  'aria-labelledby'?: string;
  'aria-describedby'?: string;
  'aria-invalid'?: boolean | 'true' | 'false';
};

const text = (node: ReactNode): string => {
  if (node == null || typeof node === 'boolean') return '';
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(text).join('');
  if (isValidElement(node)) return text((node.props as { children?: ReactNode }).children);
  return '';
};
function collect(children: ReactNode, group: string | undefined, out: Opt[]) {
  Children.forEach(children, (child) => {
    if (!isValidElement(child)) return;
    const p = child.props as { children?: ReactNode; value?: string | number; disabled?: boolean; label?: string };
    if (child.type === Fragment) collect(p.children, group, out);
    else if (child.type === 'optgroup') collect(p.children, p.label, out);
    else if (child.type === 'option') {
      const label = text(p.children);
      out.push({ value: p.value !== undefined ? String(p.value) : label, label, disabled: !!p.disabled, group });
    }
  });
}
const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

export default function Select({ value, defaultValue, onChange, children, disabled, required, id, name, className, style, title, autoFocus, autoComplete: _autoComplete, ...aria }: Props) {
  const uid = useId();
  const options = useMemo(() => { const out: Opt[] = []; collect(children, undefined, out); return out; }, [children]);
  const [inner, setInner] = useState(defaultValue !== undefined ? String(defaultValue) : '');
  const current = value !== undefined ? String(value) : inner;
  const selIndex = options.findIndex((o) => o.value === current);
  const shownIndex = selIndex >= 0 ? selIndex : 0; // like a native select, an unknown value shows the first option
  const label = options[shownIndex]?.label ?? '';
  const placeholder = !options[shownIndex] || options[shownIndex].value === '';

  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [pos, setPos] = useState<Pos | null>(null);
  const [active, setActive] = useState(-1);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const hlRef = useRef<HTMLDivElement>(null);
  const scrimRef = useRef<HTMLDivElement>(null);
  const labelRef = useRef<HTMLSpanElement>(null);
  const chevRef = useRef<HTMLSpanElement>(null);
  const optEls = useRef<(HTMLDivElement | null)[]>([]);
  const closing = useRef(false);
  const typed = useRef({ text: '', timer: 0 });
  const dock = useRef<{ touched: Set<number>; setters: WeakMap<HTMLElement, { s: (v: number) => void; x: (v: number) => void }> }>({ touched: new Set(), setters: new WeakMap() });
  const activeRef = useRef(active);
  activeRef.current = active;
  const listId = `${uid}-list`;

  const enabledFrom = (start: number, dir: 1 | -1) => { for (let i = start; i >= 0 && i < options.length; i += dir) if (!options[i].disabled) return i; return -1; };

  function show() {
    if (disabled || !options.length || mounted) return;
    setActive(selIndex >= 0 && !options[selIndex].disabled ? selIndex : enabledFrom(0, 1));
    setOpen(true);
    setMounted(true);
  }
  function hide(restoreFocus = true) {
    if (!mounted || closing.current) return;
    closing.current = true;
    setOpen(false);
    const done = () => { setMounted(false); setPos(null); closing.current = false; if (restoreFocus) triggerRef.current?.focus({ preventScroll: true }); };
    const pop = popRef.current;
    if (!pop || reduced()) return done();
    if (scrimRef.current) gsap.to(scrimRef.current, { autoAlpha: 0, duration: 0.16 });
    gsap.to(pop, pos?.sheet ? { yPercent: 100, duration: 0.22, ease: 'power2.in', onComplete: done } : { autoAlpha: 0, scale: 0.96, y: pos?.above ? 6 : -6, duration: 0.14, ease: 'power2.in', onComplete: done });
  }
  const hideRef = useRef(hide);
  hideRef.current = hide;

  function choose(i: number) {
    const o = options[i];
    if (!o || o.disabled) return;
    if (value === undefined) setInner(o.value);
    const target = { value: o.value, name: name ?? '' };
    onChange?.({ target, currentTarget: target });
    hide(true);
  }

  // Place the menu next to the button (or as a bottom sheet on phones), flipping above when there is no room below.
  useLayoutEffect(() => {
    if (!mounted) return;
    const place = () => {
      const t = triggerRef.current;
      if (!t) return;
      const r = t.getBoundingClientRect(), vw = window.innerWidth, vh = window.innerHeight;
      let next: Pos;
      if (vw < 640) next = { sheet: true, above: false, left: 0, width: vw, maxH: Math.min(vh * 0.72, 520) };
      else {
        const width = Math.min(Math.max(r.width, 224), vw - 24), left = Math.min(Math.max(12, r.left), vw - width - 12);
        const natural = Math.min(listRef.current?.scrollHeight ?? 0, 360) + 14, below = vh - r.bottom - 14, above = r.top - 14;
        const up = natural > below && above > below;
        const maxH = Math.max(140, Math.min(374, up ? above : below));
        next = up ? { sheet: false, above: true, left, bottom: vh - r.top + 8, width, maxH } : { sheet: false, above: false, left, top: r.bottom + 8, width, maxH };
      }
      setPos((prev) => (prev && prev.sheet === next.sheet && prev.above === next.above && prev.left === next.left && prev.top === next.top && prev.bottom === next.bottom && prev.width === next.width && prev.maxH === next.maxH ? prev : next));
    };
    place();
    const onScroll = (e: Event) => { if (!popRef.current?.contains(e.target as Node)) place(); };
    window.addEventListener('resize', place);
    window.addEventListener('scroll', onScroll, true);
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', onScroll, true); };
  }, [mounted]);

  // Close on an outside press.
  useEffect(() => {
    if (!mounted) return;
    const onDown = (e: PointerEvent) => {
      const n = e.target as Node;
      if (popRef.current?.contains(n) || triggerRef.current?.contains(n)) return;
      hideRef.current(false);
    };
    document.addEventListener('pointerdown', onDown, true);
    return () => document.removeEventListener('pointerdown', onDown, true);
  }, [mounted]);

  const moveHighlight = (instant: boolean) => {
    const hl = hlRef.current, el = optEls.current[activeRef.current];
    if (!hl) return;
    if (!el) { gsap.set(hl, { autoAlpha: 0 }); return; }
    const vars = { y: el.offsetTop, height: el.offsetHeight, autoAlpha: 1 };
    if (instant || reduced()) gsap.set(hl, vars);
    else gsap.to(hl, { ...vars, duration: 0.26, ease: 'power3.out', overwrite: 'auto' });
  };

  // Opening motion.
  useGSAP(() => {
    const pop = popRef.current;
    if (!pop || !pos) return;
    const tl = gsap.timeline();
    if (reduced()) gsap.set(pop, { autoAlpha: 1 });
    else {
      if (pos.sheet) {
        tl.fromTo(scrimRef.current, { autoAlpha: 0 }, { autoAlpha: 1, duration: 0.2 }, 0).fromTo(pop, { autoAlpha: 1, yPercent: 100 }, { yPercent: 0, duration: 0.36, ease: 'power3.out' }, 0);
      } else {
        tl.fromTo(pop, { autoAlpha: 0, scale: 0.94, y: pos.above ? 10 : -10, transformOrigin: pos.above ? '50% 100%' : '50% 0%' }, { autoAlpha: 1, scale: 1, y: 0, duration: 0.26, ease: 'power3.out' }, 0);
      }
      const first = optEls.current.slice(0, 14).filter(Boolean) as HTMLElement[];
      tl.fromTo(first, { autoAlpha: 0, y: 8 }, { autoAlpha: 1, y: 0, duration: 0.3, stagger: 0.018, ease: 'power2.out', clearProps: 'opacity,visibility,transform' }, 0.05);
    }
    moveHighlight(true);
    return () => { tl.kill(); };
  }, { dependencies: [!!pos] });

  // The highlight glides to the active option; keyboard moves keep it in view.
  useEffect(() => {
    if (!mounted || !pos) return;
    optEls.current[active]?.scrollIntoView({ block: 'nearest' });
    moveHighlight(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active]);

  // Chevron turns; the chosen label slides in when it changes.
  useEffect(() => { if (chevRef.current) gsap.to(chevRef.current, { rotation: open ? 180 : 0, duration: reduced() ? 0 : 0.28, ease: 'power3.out' }); }, [open]);
  const firstLabel = useRef(true);
  useEffect(() => {
    if (firstLabel.current) { firstLabel.current = false; return; }
    if (reduced() || !labelRef.current) return;
    gsap.fromTo(labelRef.current, { yPercent: 80, autoAlpha: 0 }, { yPercent: 0, autoAlpha: 1, duration: 0.32, ease: 'power3.out', overwrite: true });
  }, [label]);
  useEffect(() => { if (autoFocus) triggerRef.current?.focus(); }, [autoFocus]);
  useEffect(() => () => { gsap.killTweensOf([popRef.current, hlRef.current, labelRef.current, chevRef.current]); window.clearTimeout(typed.current.timer); }, []);

  // The macOS-dock effect: options near the pointer grow a little and lean in, fading out with distance.
  const reach = 96, grow = gsap.utils.mapRange(0, reach, 1.055, 1), lean = gsap.utils.mapRange(0, reach, 7, 0), within = gsap.utils.clamp(0, reach);
  const setters = (el: HTMLElement) => {
    let s = dock.current.setters.get(el);
    if (!s) { s = { s: gsap.quickTo(el, 'scale', { duration: 0.25, ease: 'power3.out' }), x: gsap.quickTo(el, 'x', { duration: 0.25, ease: 'power3.out' }) }; dock.current.setters.set(el, s); }
    return s;
  };
  const resetDock = () => { dock.current.touched.forEach((i) => { const el = optEls.current[i]; if (el) { setters(el).s(1); setters(el).x(0); } }); dock.current.touched = new Set(); };
  function onListMove(e: React.PointerEvent) {
    if (e.pointerType !== 'mouse' || reduced()) return;
    const idx = Number((e.target as HTMLElement).closest('[data-i]')?.getAttribute('data-i'));
    if (Number.isNaN(idx)) return;
    const near = new Set<number>();
    for (let i = Math.max(0, idx - 3); i <= Math.min(options.length - 1, idx + 3); i++) {
      const el = optEls.current[i];
      if (!el || options[i].disabled) continue;
      const r = el.getBoundingClientRect(), d = within(Math.abs(e.clientY - (r.top + r.height / 2)));
      setters(el).s(grow(d));
      setters(el).x(lean(d));
      near.add(i);
    }
    dock.current.touched.forEach((i) => { if (!near.has(i)) { const el = optEls.current[i]; if (el) { setters(el).s(1); setters(el).x(0); } } });
    dock.current.touched = near;
  }

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return;
    const step = (dir: 1 | -1) => { const n = enabledFrom(active + dir, dir); return n >= 0 ? n : active; };
    switch (e.key) {
      case 'ArrowDown': case 'ArrowUp':
        e.preventDefault();
        if (!mounted) show(); else setActive(step(e.key === 'ArrowDown' ? 1 : -1));
        return;
      case 'Home': case 'End':
        if (!mounted) return;
        e.preventDefault(); setActive(e.key === 'Home' ? enabledFrom(0, 1) : enabledFrom(options.length - 1, -1));
        return;
      case 'Enter': case ' ':
        e.preventDefault();
        if (!mounted) show(); else choose(active);
        return;
      case 'Escape':
        if (mounted) { e.preventDefault(); e.stopPropagation(); hide(true); }
        return;
      case 'Tab':
        if (mounted) hide(false);
        return;
    }
    if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      window.clearTimeout(typed.current.timer);
      typed.current.text += e.key.toLowerCase();
      typed.current.timer = window.setTimeout(() => { typed.current.text = ''; }, 600);
      const from = mounted ? active : shownIndex;
      const order = [...options.keys()].map((_, k) => (from + (typed.current.text.length > 1 ? 0 : 1) + k) % options.length);
      const hit = order.find((i) => !options[i].disabled && options[i].label.toLowerCase().startsWith(typed.current.text));
      if (hit === undefined) return;
      if (mounted) setActive(hit); else choose(hit);
    }
  }

  const popStyle: CSSProperties | undefined = pos ? { left: pos.left, top: pos.top, bottom: pos.bottom, width: pos.width, maxHeight: pos.maxH } : undefined;
  let lastGroup: string | undefined;

  return (
    <div className={styles.wrap} data-glass-select="">
      <button
        ref={triggerRef} id={id} type="button" role="combobox" title={title} disabled={disabled}
        className={`${styles.trigger} ${open ? styles.triggerOpen : ''} ${className ?? ''}`}
        style={{ ...style, paddingRight: 38 }}
        aria-haspopup="listbox" aria-expanded={open} aria-controls={mounted ? listId : undefined}
        aria-activedescendant={mounted && active >= 0 ? `${uid}-o${active}` : undefined}
        {...aria}
        onClick={() => (mounted ? hide(true) : show())} onKeyDown={onKeyDown}
      >
        <span className={styles.labelBox}><span ref={labelRef} className={`${styles.label} ${placeholder ? styles.placeholder : ''}`}>{label || ' '}</span></span>
        <span ref={chevRef} className={styles.chev} aria-hidden="true">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3.5 6l4.5 4.5L12.5 6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>
        </span>
      </button>
      {(required || name) && <input className={styles.proxy} tabIndex={-1} aria-hidden="true" name={name} required={required} value={current} onChange={() => undefined} />}
      {mounted && createPortal(
        <>
          {pos?.sheet && <div ref={scrimRef} className={styles.scrim} onPointerDown={() => hide(false)} />}
          <div ref={popRef} className={`${styles.pop} ${pos?.sheet ? styles.sheet : ''} ${pos ? '' : styles.measuring}`} style={popStyle}>
            {pos?.sheet && <span className={styles.grab} aria-hidden="true" />}
            <div ref={listRef} id={listId} role="listbox" aria-label={aria['aria-label']} className={styles.list} onPointerMove={onListMove} onPointerLeave={resetDock}>
              <div ref={hlRef} className={styles.hl} aria-hidden="true" />
              {options.map((o, i) => {
                const heading = o.group && o.group !== lastGroup ? o.group : null;
                lastGroup = o.group;
                return (
                  <Fragment key={i}>
                    {heading && <div className={styles.group} role="presentation">{heading}</div>}
                    <div
                      ref={(el) => { optEls.current[i] = el; }} id={`${uid}-o${i}`} role="option" data-i={i}
                      aria-selected={i === selIndex} aria-disabled={o.disabled || undefined}
                      className={`${styles.opt} ${i === selIndex ? styles.optSelected : ''} ${o.value === '' ? styles.optEmpty : ''}`}
                      onPointerMove={() => { if (!o.disabled && active !== i) setActive(i); }}
                      onClick={() => choose(i)}
                    >
                      <span className={styles.optLabel}>{o.label || ' '}</span>
                      {i === selIndex && <svg className={styles.check} width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M3.5 8.5l3 3 6-7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" /></svg>}
                    </div>
                  </Fragment>
                );
              })}
            </div>
          </div>
        </>,
        // A modal <dialog> sits above everything in the page, so inside one the menu has to live in the dialog too.
        triggerRef.current?.closest('dialog') ?? document.body,
      )}
    </div>
  );
}
