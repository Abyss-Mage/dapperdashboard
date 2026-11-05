'use client';

import { useRef } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsapContact';
import { partners } from '../dapper';
import './dealers.css';

/* The dealerships as an interlocking lattice of small white diamonds (an inverted triangle on a wide screen). Used wherever the site shows the
   dealers: the diamonds spring in from the middle when they arrive, bob in a slow ripple, and pointing at one sends a ripple through the rest.
   Subaru and Hyundai are white marks, so their diamonds are in their own brand blue. Phones get a narrower arrangement. */

const WIDE = [[0, 0], [2, 0], [4, 0], [6, 0], [1, 1], [3, 1], [5, 1], [2, 2], [4, 2]];
const NARROW = [[0, 0], [2, 0], [4, 0], [1, 1], [3, 1], [0, 2], [2, 2], [4, 2], [1, 3]];
const BRAND: Record<string, string> = { Subaru: '#10247c', Hyundai: '#002c5f' };

export function Dealers() {
  const root = useRef<HTMLDivElement>(null);

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const tiles = gsap.utils.toArray<HTMLElement>('.dl__dia');
      if (!tiles.length) return;
      gsap.set(tiles, { scale: 0, rotation: -24, transformOrigin: '50% 50%' });
      const enter = ScrollTrigger.create({ trigger: '.dl__lat', start: 'top 85%', once: true, onEnter: () => {
        gsap.to(tiles, { scale: 1, rotation: 0, duration: .9, ease: 'back.out(1.7)', stagger: { each: .07, from: 'center' },
          onComplete: () => tiles.forEach((t, i) => gsap.to(t, { y: -7, duration: 2.6, ease: 'sine.inOut', yoyo: true, repeat: -1, delay: i * .22 })) });
      } });
      const offs = tiles.map((t) => {
        const on = () => {
          const rt = t.getBoundingClientRect();
          const cx = rt.left + rt.width / 2, cy = rt.top + rt.height / 2;
          tiles.forEach((o) => {
            const ro = o.getBoundingClientRect();
            const dist = Math.hypot(ro.left + ro.width / 2 - cx, ro.top + ro.height / 2 - cy) / rt.width;
            gsap.killTweensOf(o, 'scale,rotation');
            gsap.fromTo(o, { scale: 1, rotation: 0 }, { keyframes: [{ scale: o === t ? 1.12 : 1.06, rotation: o === t ? 0 : -5, duration: .18 }, { scale: 1, rotation: 0, duration: .9, ease: 'elastic.out(1,.35)' }], delay: dist * .09 });
          });
        };
        t.addEventListener('pointerenter', on);
        return () => t.removeEventListener('pointerenter', on);
      });
      return () => { offs.forEach((f) => f()); enter.kill(); tiles.forEach((t) => gsap.killTweensOf(t)); gsap.set(tiles, { clearProps: 'transform,scale,rotation,y,transformOrigin' }); };
    });
    return () => mm.revert();
  }, { scope: root });

  return (
    <div className="dl" ref={root}>
      <ul className="dl__sr">{partners.map((p) => <li key={p.name}>{p.name}</li>)}</ul>
      <div className="dl__lat" aria-hidden="true">
        {partners.map((p, i) => {
          const n = NARROW[i] ?? [0, 0], w = WIDE[i] ?? [0, 0];
          const bg = BRAND[p.name];
          return (
            <div className="dl__dia" key={p.name} style={{ ['--mc' as string]: n[0], ['--mr' as string]: n[1], ['--wc' as string]: w[0], ['--wr' as string]: w[1] }}>
              <span className="dl__s" style={bg ? { ['--bg' as string]: bg } : undefined}>
                {p.src ? <img className={p.name.includes('McGee') ? 'is-crop' : undefined} src={p.src} alt="" loading="lazy" /> : <span className="dl__name">{p.name}</span>}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
