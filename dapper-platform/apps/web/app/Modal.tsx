'use client';
import { useEffect, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import gsap from 'gsap';
import styles from './Modal.module.css';

/* One popup for the whole dashboard: glass card over a dimmed page, closes with Escape, the backdrop or the close button,
   returns focus to what opened it, and keeps the page behind it from scrolling. */
export default function Modal({ open, title, subtitle, onClose, children, wide }: { open: boolean; title: string; subtitle?: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const card = useRef<HTMLDivElement>(null);
  const opener = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (!open) return;
    opener.current = document.activeElement as HTMLElement | null;
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', key);
    const prev = document.body.style.overflow; document.body.style.overflow = 'hidden';
    if (card.current && !matchMedia('(prefers-reduced-motion: reduce)').matches) gsap.fromTo(card.current, { autoAlpha: 0, y: 18, scale: 0.97 }, { autoAlpha: 1, y: 0, scale: 1, duration: 0.28, ease: 'power3.out' });
    card.current?.focus();
    return () => { document.removeEventListener('keydown', key); document.body.style.overflow = prev; opener.current?.focus?.(); };
  }, [open, onClose]);
  if (!open || typeof document === 'undefined') return null;
  return createPortal(
    <div className={styles.overlay} onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div ref={card} className={`${styles.card} ${wide ? styles.wide : ''}`} role="dialog" aria-modal="true" aria-label={title} tabIndex={-1}>
        <header className={styles.head}>
          <div><h2>{title}</h2>{subtitle && <p>{subtitle}</p>}</div>
          <button type="button" className={styles.close} aria-label="Close" onClick={onClose}>×</button>
        </header>
        <div className={styles.body}>{children}</div>
      </div>
    </div>,
    document.body,
  );
}
