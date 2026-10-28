'use client';
import { useRef, useState } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsap';
import { mediaUrl, usePublished } from './usePublished';
import type { PublishedImage } from './usePublished';
import { X } from './Icons';

const FILTERS = [
  { id: 'all', label: 'All' },
  { id: 'detailing', label: 'Detailing' },
  { id: 'ceramic', label: 'Ceramic coating' },
  { id: 'ppf', label: 'PPF' },
];

/* Published portfolio photos, taped to the paint. `limit` shows the latest few (home page); otherwise the full gallery with filters. */
export default function Gallery({ limit }: { limit?: number }) {
  const { status, portfolio, retry } = usePublished();
  const [filter, setFilter] = useState('all');
  const [open, setOpen] = useState<PublishedImage | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLElement | null>(null);
  const grid = useRef<HTMLDivElement>(null);

  const shown = portfolio.filter((p) => filter === 'all' || p.category === filter).slice(0, limit ?? 200);

  useGSAP(() => {
    if (status !== 'ready' || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const items = gsap.utils.toArray<HTMLElement>('figure', grid.current);
    gsap.set(items, { y: 60, opacity: 0, rotate: () => gsap.utils.random(-2.5, 2.5) });
    ScrollTrigger.batch(items, { start: 'top 92%', once: true, onEnter: (b) => gsap.to(b, { y: 0, opacity: 1, rotate: 0, duration: 1.1, stagger: 0.08, ease: 'expo.out' }) });
    ScrollTrigger.refresh();
  }, { scope: grid, dependencies: [status, filter], revertOnUpdate: true });

  const show = (item: PublishedImage, from: HTMLElement) => {
    opener.current = from;
    setOpen(item);
    requestAnimationFrame(() => dialog.current?.showModal());
  };

  return (
    <div>
      {!limit && (
        <div className="filters" role="group" aria-label="Filter portfolio">
          {FILTERS.map((f) => <button key={f.id} type="button" className="chip" aria-pressed={filter === f.id} disabled={status !== 'ready'} onClick={() => setFilter(f.id)}>{f.label}</button>)}
        </div>
      )}
      <p className="status" role="status">
        {status === 'loading' ? 'Loading the latest work…' : status === 'error' ? '' : shown.length ? `${shown.length} published ${shown.length === 1 ? 'photo' : 'photos'}.` : 'No published photos in this category yet.'}
      </p>
      {status === 'error' && (
        <div className="state"><p>The portfolio could not be loaded. Try again, or contact Dapper to talk about recent work.</p><button type="button" className="btn btn--tape" onClick={retry}>Try again</button></div>
      )}
      <div ref={grid} className="gallery">
        {shown.map((item) => (
          <figure key={item.mediaId}>
            <button type="button" onClick={(e) => show(item, e.currentTarget)} aria-label={'View ' + item.title}>
              <span className="taped"><img src={mediaUrl(item.mediaId)} alt={item.alt} loading="lazy" width={1100} height={825} /></span>
            </button>
            <figcaption>{item.title}</figcaption>
          </figure>
        ))}
      </div>
      <dialog ref={dialog} className="lightbox" aria-labelledby="lightbox-title" onClose={() => { setOpen(null); opener.current?.focus(); }} onClick={(e) => { if (e.target === dialog.current) dialog.current?.close(); }}>
        {open && (
          <>
            <img src={mediaUrl(open.mediaId)} alt={open.alt} />
            <h2 id="lightbox-title">{open.title}</h2>
            <button type="button" className="btn btn--tape lightbox__close" onClick={() => dialog.current?.close()}><X size={18} aria-hidden />Close</button>
          </>
        )}
      </dialog>
    </div>
  );
}
