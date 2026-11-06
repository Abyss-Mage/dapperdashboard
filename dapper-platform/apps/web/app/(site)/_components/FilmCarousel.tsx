'use client';

import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, Play } from 'lucide-react';
import './films.css';

/* The studio's films as a carousel for the right-hand side of a two-column section (text on the left). Tall reels in a swipeable row with
   arrows; each plays muted while it is on screen, and selecting one calls onOpen so the page can show it with sound. */
export function FilmCarousel({ clips, onOpen }: { clips: string[]; onOpen: (i: number) => void }) {
  const track = useRef<HTMLUListElement>(null);
  const [edge, setEdge] = useState({ start: true, end: false });

  useEffect(() => {
    const el = track.current;
    if (!el) return;
    const measure = () => setEdge({ start: el.scrollLeft < 8, end: el.scrollLeft + el.clientWidth >= el.scrollWidth - 8 });
    measure();
    el.addEventListener('scroll', measure, { passive: true });
    window.addEventListener('resize', measure);
    const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const vids = Array.from(el.querySelectorAll('video'));
    const io = new IntersectionObserver((entries) => entries.forEach((e) => {
      const v = e.target as HTMLVideoElement;
      if (reduce) return;
      if (e.isIntersecting) void v.play().catch(() => undefined); else v.pause();
    }), { root: el, threshold: .35 });
    vids.forEach((v) => { if (reduce) { v.preload = 'auto'; v.currentTime = .1; } io.observe(v); });
    return () => { el.removeEventListener('scroll', measure); window.removeEventListener('resize', measure); io.disconnect(); };
  }, []);

  const by = (dir: number) => { const el = track.current; if (el) el.scrollBy({ left: dir * el.clientWidth * .7, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }); };

  return (
    <div className="fc">
      <ul className="fc__track" ref={track} tabIndex={0} aria-label="Films from the Dapper studio">
        {clips.map((c, i) => (
          <li key={c}>
            <button type="button" className="fc__reel" onClick={() => onOpen(i)} aria-label={'Watch film ' + (i + 1) + ' of ' + clips.length + ' with sound'}>
              <video src={c + '#t=0.1'} muted loop playsInline preload="metadata" aria-hidden="true" tabIndex={-1} />
              <span className="fc__n" aria-hidden="true">Film {i + 1}</span>
              <span className="fc__play" aria-hidden="true"><Play size={18} fill="currentColor" /></span>
            </button>
          </li>
        ))}
      </ul>
      <div className="fc__ctrl">
        <button type="button" onClick={() => by(-1)} disabled={edge.start} aria-label="Previous films"><ArrowLeft size={22} aria-hidden="true" /></button>
        <button type="button" onClick={() => by(1)} disabled={edge.end} aria-label="Next films"><ArrowRight size={22} aria-hidden="true" /></button>
      </div>
    </div>
  );
}
