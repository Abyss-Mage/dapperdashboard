'use client';

import { useEffect, useRef, useState, type InputHTMLAttributes } from 'react';
import gsap from 'gsap';
import styles from './NumberField.module.css';
import type { SelectChange } from './GlassSelect';

/* A number input with − and + buttons. It is still a real <input type="number">, so min, max, step, required and typing all work;
   the buttons step by `step`, hold to repeat (faster the longer you hold), and the arrow keys work as usual.
   Same props as the plain input: value, onChange(e.target.value), min, max, step, placeholder, disabled, required, name, aria-label. */

const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'type' | 'value' | 'defaultValue'> & {
  value?: string | number;
  defaultValue?: string | number;
  onChange?: (e: SelectChange) => void;
};

export default function NumberField({ value, defaultValue, onChange, min, max, step, disabled, readOnly, className, style, name, ...rest }: Props) {
  const [inner, setInner] = useState(defaultValue !== undefined ? String(defaultValue) : '');
  const current = value !== undefined ? String(value) : inner;
  const inputRef = useRef<HTMLInputElement>(null);
  const holdRef = useRef({ timer: 0, started: 0 });
  const latest = useRef({ current, min, max, step });
  latest.current = { current, min, max, step };

  const stepSize = step === undefined || step === 'any' ? 1 : Number(step) || 1;
  const decimals = (String(step ?? '').split('.')[1] ?? '').length;
  const lo = min === undefined || min === '' ? -Infinity : Number(min), hi = max === undefined || max === '' ? Infinity : Number(max);

  const emit = (next: string) => {
    if (value === undefined) setInner(next);
    const target = { value: next, name: name ?? '' };
    onChange?.({ target, currentTarget: target });
  };
  const bump = (dir: 1 | -1, multiplier = 1) => {
    const { current: c } = latest.current;
    const base = c === '' || Number.isNaN(Number(c)) ? (dir > 0 ? (Number.isFinite(lo) ? lo - stepSize : 0) : (Number.isFinite(hi) ? hi + stepSize : 0)) : Number(c);
    const raw = base + dir * stepSize * multiplier;
    const next = Math.min(hi, Math.max(lo, raw));
    emit(decimals ? next.toFixed(decimals) : String(Math.round(next * 1e6) / 1e6));
    if (!reduced() && inputRef.current) gsap.fromTo(inputRef.current, { scale: 1.035 }, { scale: 1, duration: 0.28, ease: 'back.out(3)', overwrite: true });
  };
  const stop = () => { window.clearTimeout(holdRef.current.timer); holdRef.current.timer = 0; };
  const start = (dir: 1 | -1) => (e: React.PointerEvent) => {
    if (disabled || readOnly || e.button !== 0) return;
    bump(dir);
    holdRef.current.started = performance.now();
    const tick = () => {
      const held = performance.now() - holdRef.current.started;
      bump(dir, held > 2600 ? 100 : held > 1300 ? 10 : 1);
      holdRef.current.timer = window.setTimeout(tick, held > 1300 ? 55 : 95);
    };
    holdRef.current.timer = window.setTimeout(tick, 420);
  };
  useEffect(() => stop, []);

  const n = Number(current);
  const atMin = current !== '' && !Number.isNaN(n) && n <= lo, atMax = current !== '' && !Number.isNaN(n) && n >= hi;
  return (
    <div className={`${styles.wrap} ${disabled ? styles.disabled : ''}`} data-number-field="">
      <button type="button" tabIndex={-1} className={styles.btn} aria-label="Decrease" disabled={disabled || readOnly || atMin} onPointerDown={start(-1)} onPointerUp={stop} onPointerLeave={stop} onPointerCancel={stop} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); bump(-1); } }}>
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true"><path d="M2.5 7h9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" /></svg>
      </button>
      <input
        {...rest} ref={inputRef} type="number" name={name} min={min} max={max} step={step} disabled={disabled} readOnly={readOnly}
        className={`${styles.input} ${className ?? ''}`} style={style} value={current}
        onChange={(e) => emit(e.target.value)}
      />
      <button type="button" tabIndex={-1} className={styles.btn} aria-label="Increase" disabled={disabled || readOnly || atMax} onPointerDown={start(1)} onPointerUp={stop} onPointerLeave={stop} onPointerCancel={stop} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); bump(1); } }}>
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true"><path d="M2.5 7h9M7 2.5v9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" /></svg>
      </button>
    </div>
  );
}
