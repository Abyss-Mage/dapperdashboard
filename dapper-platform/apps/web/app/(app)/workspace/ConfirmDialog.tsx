'use client';
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import gsap from 'gsap';
import { useGSAP } from '@gsap/react';
import styles from './ConfirmDialog.module.css';

gsap.registerPlugin(useGSAP);

export type ConfirmDialogProps = {
  isOpen: boolean;
  title: string;
  /** Needed for delete / archive; the name is shown in the question. */
  entityName?: string;
  entityType?: string;
  /** "confirm" is a plain yes / no question that uses `message`. */
  actionType: 'delete' | 'archive' | 'confirm';
  explanation?: string;
  message?: string;
  impactNotice?: string;
  isBusy?: boolean;
  confirmLabel?: string;
  /** danger turns the confirm button red; delete is always danger. */
  tone?: 'danger' | 'default';
  onConfirm: () => void | Promise<void>;
  onCancel: () => void;
};

const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

const Icon = ({ kind }: { kind: ConfirmDialogProps['actionType'] }) => (
  <svg width="26" height="26" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    {kind === 'delete' && <path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 002 2h6a2 2 0 002-2l1-12M9 7V4h6v3" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />}
    {kind === 'archive' && <path d="M4 7h16v3H4zM6 10v8a2 2 0 002 2h8a2 2 0 002-2v-8M10 14h4" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />}
    {kind === 'confirm' && <><circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.7" /><path d="M12 7.5v5.2M12 16.3v.2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" /></>}
  </svg>
);

export default function ConfirmDialog({ isOpen, title, entityName, entityType = 'item', actionType, explanation, message, impactNotice, isBusy = false, confirmLabel, tone, onConfirm, onCancel }: ConfirmDialogProps) {
  const [mounted, setMounted] = useState(false);
  const cancelRef = useRef<HTMLButtonElement>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const previouslyFocused = useRef<HTMLElement | null>(null);
  const danger = tone ? tone === 'danger' : actionType === 'delete';

  useEffect(() => {
    if (isOpen) { previouslyFocused.current = document.activeElement as HTMLElement | null; setMounted(true); return; }
    if (!mounted) return;
    const overlay = overlayRef.current, card = cardRef.current;
    const done = () => { setMounted(false); previouslyFocused.current?.focus?.(); };
    if (!overlay || !card || reduced()) { done(); return; }
    const tl = gsap.timeline({ onComplete: done });
    tl.to(card, { autoAlpha: 0, y: 14, scale: 0.97, duration: 0.18, ease: 'power2.in' }, 0).to(overlay, { autoAlpha: 0, duration: 0.2 }, 0);
    return () => { tl.kill(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  useGSAP(() => {
    if (!mounted || !isOpen || !overlayRef.current || !cardRef.current) return;
    const tl = gsap.timeline();
    if (!reduced()) {
      tl.fromTo(overlayRef.current, { autoAlpha: 0 }, { autoAlpha: 1, duration: 0.25, ease: 'power1.out' }, 0)
        .fromTo(cardRef.current, { autoAlpha: 0, y: 26, scale: 0.92 }, { autoAlpha: 1, y: 0, scale: 1, duration: 0.45, ease: 'back.out(1.5)' }, 0.03)
        .fromTo(cardRef.current.querySelectorAll('[data-in]'), { autoAlpha: 0, y: 10 }, { autoAlpha: 1, y: 0, duration: 0.35, stagger: 0.06, ease: 'power2.out', clearProps: 'opacity,visibility,transform' }, 0.14);
    }
    // Safe initial focus on Cancel to prevent accidental destruction.
    const t = window.setTimeout(() => cancelRef.current?.focus(), 60);
    return () => { window.clearTimeout(t); tl.kill(); };
  }, { dependencies: [mounted, isOpen] });

  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !isBusy) { e.preventDefault(); e.stopPropagation(); onCancel(); }
      if (e.key === 'Tab') { // keep focus inside the dialog
        const els = cardRef.current?.querySelectorAll<HTMLElement>('button:not(:disabled)');
        if (!els?.length) return;
        const first = els[0], last = els[els.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [isOpen, isBusy, onCancel]);

  if (!mounted) return null;
  const verb = actionType === 'delete' ? 'delete' : 'archive';
  const defaultLabel = actionType === 'delete' ? `Delete ${entityType}` : actionType === 'archive' ? `Archive ${entityType}` : 'Confirm';

  return createPortal(
    <div ref={overlayRef} className={styles.overlay} role="presentation" onMouseDown={(e) => { if (e.target === e.currentTarget && !isBusy) onCancel(); }}>
      <div ref={cardRef} className={styles.card} role="dialog" aria-modal="true" aria-labelledby="confirm-dialog-title" aria-describedby="confirm-dialog-desc">
        <span className={`${styles.badge} ${danger ? styles.badgeDanger : ''}`} data-in="" aria-hidden="true"><Icon kind={actionType} /></span>
        <h2 id="confirm-dialog-title" className={styles.title} data-in="">{title}</h2>
        <div id="confirm-dialog-desc" className={styles.body} data-in="">
          {actionType === 'confirm' ? <p>{message}</p> : <p>Are you sure you want to {verb} <strong>{entityName}</strong>?</p>}
          {explanation && <p className={styles.muted}>{explanation}</p>}
          {impactNotice && <div className={styles.notice}><strong>Dependency notice:</strong> {impactNotice}</div>}
        </div>
        <div className={styles.actions} data-in="">
          <button ref={cancelRef} type="button" className="secondary" onClick={onCancel} disabled={isBusy}>Cancel</button>
          <button type="button" className={`primary ${danger ? styles.danger : ''}`} onClick={onConfirm} disabled={isBusy}>{isBusy ? 'Processing…' : (confirmLabel || defaultLabel)}</button>
        </div>
      </div>
    </div>,
    // Inside the open popup when there is one: a modal popup sits above the page and blocks it, so a question drawn on the page
    // body would be hidden and unanswerable (error404 #52: "Issue and email" stuck on "Saving…").
    [...document.querySelectorAll('dialog')].filter((d) => d.open && d.matches(':modal')).pop() ?? document.body,
  );
}
