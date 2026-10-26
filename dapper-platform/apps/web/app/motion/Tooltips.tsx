'use client';

import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import gsap from 'gsap';
import styles from './Tooltips.module.css';

/* Replaces the browser's slow, plain title tooltips with a glass tip that appears at the element after a short pause
   (instantly on keyboard focus). The title is moved to data-tip so the browser's own tooltip never shows, and an element with
   no other name keeps it as its accessible name. Touch devices are left alone. */

const reduced = () => matchMedia('(prefers-reduced-motion: reduce)').matches;

export default function Tooltips() {
  const [tip, setTip] = useState<{ text: string; el: HTMLElement } | null>(null);
  const [pos, setPos] = useState<{ left: number; top: number; above: boolean } | null>(null);
  const tipRef = useRef<HTMLDivElement>(null);
  const timer = useRef(0);

  useEffect(() => {
    const take = (el: HTMLElement) => {
      const title = el.getAttribute('title');
      if (title) {
        el.setAttribute('data-tip', title);
        el.removeAttribute('title');
        if (!el.getAttribute('aria-label') && !el.getAttribute('aria-labelledby') && !(el.textContent ?? '').trim()) el.setAttribute('aria-label', title);
      }
      return el.getAttribute('data-tip') ?? '';
    };
    const find = (t: EventTarget | null) => (t instanceof Element ? (t.closest('[title], [data-tip]') as HTMLElement | null) : null);
    const hide = () => { window.clearTimeout(timer.current); setTip(null); setPos(null); };
    const show = (el: HTMLElement, delay: number) => {
      const text = take(el);
      if (!text) return;
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setTip({ text, el }), delay);
    };
    const over = (e: PointerEvent) => { if (e.pointerType !== 'mouse') return; const el = find(e.target); if (el) show(el, 380); else hide(); };
    const out = (e: PointerEvent) => { if (find(e.target) && !find(e.relatedTarget)) hide(); };
    const focus = (e: FocusEvent) => { const el = find(e.target); if (el && el.matches(':focus-visible')) show(el, 120); };
    document.addEventListener('pointerover', over);
    document.addEventListener('pointerout', out);
    document.addEventListener('focusin', focus);
    document.addEventListener('focusout', hide);
    document.addEventListener('pointerdown', hide, true);
    document.addEventListener('keydown', hide, true);
    window.addEventListener('scroll', hide, true);
    return () => {
      window.clearTimeout(timer.current);
      document.removeEventListener('pointerover', over); document.removeEventListener('pointerout', out);
      document.removeEventListener('focusin', focus); document.removeEventListener('focusout', hide);
      document.removeEventListener('pointerdown', hide, true); document.removeEventListener('keydown', hide, true);
      window.removeEventListener('scroll', hide, true);
    };
  }, []);

  // Place the tip above the element (below when there is no room), kept inside the screen.
  useEffect(() => {
    if (!tip || !tipRef.current) return;
    const r = tip.el.getBoundingClientRect(), t = tipRef.current.getBoundingClientRect();
    const above = r.top > t.height + 16;
    setPos({ left: Math.min(Math.max(8, r.left + r.width / 2 - t.width / 2), window.innerWidth - t.width - 8), top: above ? r.top - t.height - 10 : r.bottom + 10, above });
  }, [tip]);
  useEffect(() => {
    if (!pos || !tipRef.current || reduced()) return;
    gsap.fromTo(tipRef.current, { autoAlpha: 0, y: pos.above ? 6 : -6, scale: 0.94 }, { autoAlpha: 1, y: 0, scale: 1, duration: 0.22, ease: 'power3.out', overwrite: true });
  }, [pos]);

  if (!tip) return null;
  return createPortal(<div ref={tipRef} role="tooltip" className={styles.tip} style={pos ? { left: pos.left, top: pos.top } : { left: 0, top: 0, visibility: 'hidden' }}>{tip.text}</div>, document.body);
}
