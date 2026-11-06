'use client';

import { useEffect, useRef, useState } from 'react';
import { X } from 'lucide-react';
import s from './chrome.module.css';

/* The booking popup. Any link to /booking, or any element marked data-book, opens it as a dialog over the page the customer is on (a
   full-screen sheet on a phone); nothing navigates. It shows the real booking page in embed mode (just the steps), so booking behaves
   exactly as on /booking, and it keeps its progress if closed and reopened. Without script the links still go to /booking. */
export default function BookDialog() {
  const ref = useRef<HTMLDialogElement>(null);
  const [src, setSrc] = useState('');
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const show = () => {
      setSrc((cur) => cur || '/booking?embed=1');
      const d = ref.current;
      if (d && !d.open) { d.showModal(); document.documentElement.style.overflow = 'hidden'; }
    };
    const onClick = (e: MouseEvent) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const a = (e.target as Element | null)?.closest?.('a[href="/booking"], [data-book]') as HTMLAnchorElement | null;
      if (!a || a.target === '_blank' || location.pathname === '/booking') return;
      e.preventDefault(); e.stopImmediatePropagation();
      show();
    };
    document.addEventListener('click', onClick, true);
    window.addEventListener('dapper:book', show);
    return () => { document.removeEventListener('click', onClick, true); window.removeEventListener('dapper:book', show); };
  }, []);

  const close = () => ref.current?.close();
  return (
    <dialog ref={ref} className={s.dialog} aria-label="Book your detail" data-lenis-prevent
      onClose={() => { document.documentElement.style.overflow = ''; }}
      onClick={(e) => { if (e.target === e.currentTarget) close(); }}>
      <div className={s.dialogBar}>
        <strong>Book your detail</strong>
        <button type="button" className={s.close} onClick={close} aria-label="Close booking"><X size={24} aria-hidden="true" /></button>
      </div>
      {src ? <iframe className={s.frame} src={src} title="Book your detail" onLoad={() => setLoaded(true)} /> : null}
      {src && !loaded ? <p className={s.wait} role="status">Loading…</p> : null}
    </dialog>
  );
}
