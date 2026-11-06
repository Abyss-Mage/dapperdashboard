'use client';

import { useEffect, useRef } from 'react';
import { Play } from 'lucide-react';
import { ScrollRow } from './ScrollRow';
import './films.css';

/* The studio's films as a carousel for the right-hand side of a two-column section (text on the left). Tall reels in the shared carousel
   (swipe, drag, arrows, moves along by itself); each plays muted while it is on screen, and selecting one calls onOpen so the page can
   show it with sound. */
export function FilmCarousel({ clips, onOpen }: { clips: string[]; onOpen: (i: number) => void }) {
  const wrap = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    // The viewport as root still counts what the sliding row hides, so only the films you can see play.
    const io = new IntersectionObserver((entries) => entries.forEach((e) => {
      const v = e.target as HTMLVideoElement;
      if (reduce) return;
      if (e.isIntersecting) void v.play().catch(() => undefined); else v.pause();
    }), { threshold: .35 });
    el.querySelectorAll('video').forEach((v) => { if (reduce) { v.preload = 'auto'; v.currentTime = .1; } io.observe(v); });
    return () => io.disconnect();
  }, [clips]);

  return (
    <div className="fc" ref={wrap}>
      <ScrollRow label="Films from the Dapper studio" auto={3200} className="fc__track">
        {clips.map((c, i) => (
          <li key={c}>
            <button type="button" className="fc__reel" onClick={() => onOpen(i)} aria-label={'Watch film ' + (i + 1) + ' of ' + clips.length + ' with sound'}>
              <video src={c + '#t=0.1'} muted loop playsInline preload="metadata" aria-hidden="true" tabIndex={-1} />
              <span className="fc__n" aria-hidden="true">Film {i + 1}</span>
              <span className="fc__play" aria-hidden="true"><Play size={18} fill="currentColor" /></span>
            </button>
          </li>
        ))}
      </ScrollRow>
    </div>
  );
}
