'use client';

import { useEffect, useLayoutEffect, useRef } from 'react';
import gsap from 'gsap';

/* Counts a figure up to its value ("$1,284.50", "42", "12 h") and, when it changes, from the old value to the new one.
   The text is plain until the number starts moving, so server rendering, copy and screen readers see the real value. */

const useIsoLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;
const PATTERN = /^([^0-9-]*)(-?[\d,]*\.?\d+)([\s\S]*)$/;

export default function AnimatedNumber({ text, duration = 1.1 }: { text: string; duration?: number }) {
  const ref = useRef<HTMLSpanElement>(null);
  const last = useRef(0);
  useIsoLayoutEffect(() => {
    const el = ref.current, m = PATTERN.exec(text);
    if (!el || !m) return;
    const [, pre, num, post] = m;
    const target = parseFloat(num.replace(/,/g, ''));
    if (!Number.isFinite(target) || matchMedia('(prefers-reduced-motion: reduce)').matches) { last.current = Number.isFinite(target) ? target : 0; return; }
    const decimals = (num.split('.')[1] ?? '').length, group = num.includes(',');
    const fmt = (v: number) => pre + v.toLocaleString('en-CA', { minimumFractionDigits: decimals, maximumFractionDigits: decimals, useGrouping: group }) + post;
    const state = { v: last.current };
    el.textContent = fmt(state.v);
    const tween = gsap.to(state, { v: target, duration, ease: 'power3.out', onUpdate: () => { el.textContent = fmt(state.v); }, onComplete: () => { el.textContent = text; last.current = target; } });
    return () => { tween.kill(); last.current = state.v; };
  }, [text, duration]);
  return <span ref={ref} suppressHydrationWarning>{text}</span>;
}
