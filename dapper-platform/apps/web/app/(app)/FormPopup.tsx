'use client';

import { useEffect, useRef, type ReactNode } from 'react';

/* A form in a centred popup, so it opens where you are instead of far up the page. Escape asks the screen to close it, and the screen can refuse while saving. */
export default function FormPopup({ label, onClose, children, width = 760 }: { label: string; onClose: () => void; children: ReactNode; width?: number }) {
  const box = useRef<HTMLDialogElement>(null);
  useEffect(() => { const d = box.current; if (d && !d.open) d.showModal(); }, []);
  return (
    <dialog ref={box} aria-label={label} onCancel={(e) => { e.preventDefault(); onClose(); }} style={{ width: `min(${width}px, calc(100vw - 32px))`, maxHeight: 'calc(100dvh - 32px)', margin: 'auto', padding: 20, overflow: 'auto' }}>
      {children}
    </dialog>
  );
}
