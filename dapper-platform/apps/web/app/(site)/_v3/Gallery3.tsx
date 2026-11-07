'use client';
import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import { portfolio } from '../dapper';

// The portfolio photographs as a masonry gallery; a photo opens full size, Escape or a click closes it.
export default function Gallery3() {
  const [open, setOpen] = useState<number | null>(null);
  useEffect(() => {
    if (open === null) return;
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(null);
      if (e.key === 'ArrowRight') setOpen((i) => (i === null ? i : (i + 1) % portfolio.length));
      if (e.key === 'ArrowLeft') setOpen((i) => (i === null ? i : (i + portfolio.length - 1) % portfolio.length));
    };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  }, [open]);
  return (
    <>
      <div className="d-gallery">
        {portfolio.map((p, i) => (
          <button key={p.src} type="button" onClick={() => setOpen(i)} aria-label={'Open photo: ' + p.caption} data-d-in>
            <img src={p.src} alt={p.caption} loading="lazy" /><span>{p.caption}</span>
          </button>
        ))}
      </div>
      {open !== null && (
        <div className="d-lightbox" role="dialog" aria-modal="true" aria-label={portfolio[open].caption} onClick={() => setOpen(null)}>
          <img src={portfolio[open].src} alt={portfolio[open].caption} onClick={(e) => e.stopPropagation()} />
          <p>{portfolio[open].caption} · {open + 1} / {portfolio.length}</p>
          <button type="button" className="d-film__close" onClick={() => setOpen(null)} aria-label="Close"><X size={22} /></button>
        </div>
      )}
    </>
  );
}
