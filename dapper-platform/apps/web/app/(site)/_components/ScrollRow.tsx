'use client';
import { useRef, type ReactNode } from 'react';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import { useScrollRow } from './useScrollRow';
import './scrollrow.css';

/* A carousel: the children (each one an <li>) in a row that can be swiped, dragged with the mouse, stepped with the arrows,
   and (with `auto`) moves along by itself. Pure markup and behaviour; the cards bring their own look. */
export function ScrollRow({ children, label, auto = 0, className = '', tone = 'dark' }: { children: ReactNode; label: string; auto?: number; className?: string; tone?: 'dark' | 'light' }) {
  const track = useRef<HTMLUListElement>(null);
  const edge = useScrollRow(track, auto);
  const by = (dir: number) => { const el = track.current; if (el) el.scrollBy({ left: dir * el.clientWidth * .7, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }); };
  return (
    <div className={'srow srow--' + tone}>
      <ul className={'srow__track ' + className} ref={track} tabIndex={0} aria-label={label}>{children}</ul>
      <div className="srow__ctrl">
        <button type="button" onClick={() => by(-1)} disabled={edge.start} aria-label="Previous"><ArrowLeft size={22} aria-hidden="true" /></button>
        <button type="button" onClick={() => by(1)} disabled={edge.end} aria-label="Next"><ArrowRight size={22} aria-hidden="true" /></button>
      </div>
    </div>
  );
}
