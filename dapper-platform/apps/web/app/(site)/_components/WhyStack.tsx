'use client';
import { useRef } from 'react';
import { why } from '../dapper';
import { gsap, ScrollTrigger, useGSAP } from './gsapContact';
import './why.css';

/* "Why vehicle owners trust Dapper": the six reasons as dark cards that stack as you scroll (the one underneath shrinks back and dims while
   the next slides up over it). The same section as on the About page. */
export function WhyStack({ id }: { id: string }) {
  const root = useRef<HTMLElement>(null);
  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference) and (min-width: 1024px)', () => {
      const cards = gsap.utils.toArray<HTMLElement>('.ws-card');
      cards.forEach((card, i) => {
        if (i < cards.length - 1) gsap.to(card, { scale: .94, '--dim': .55, ease: 'none', scrollTrigger: { trigger: cards[i + 1], start: 'top 92%', end: 'top 28%', scrub: true } });
      });
      document.fonts?.ready.then(() => ScrollTrigger.refresh());
    });
    return () => mm.revert();
  }, { scope: root });
  return (
    <section ref={root} className="ws" aria-labelledby={id}>
      <h2 id={id} className="ws__h" data-head>Why vehicle owners trust <em>Dapper.</em></h2>
      <div className="ws-stack">
        {why.map((w, i) => (
          <article key={w.title} className="ws-card" style={{ ['--i' as string]: i }}>
            <h3>{w.title}</h3>
            <p>{w.text}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
