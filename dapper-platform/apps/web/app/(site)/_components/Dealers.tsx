'use client';

import { useEffect, useRef, useState } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsapContact';
import { partners } from '../dapper';
import { mediaUrl } from './usePublished';
import './dealers.css';

/* The dealerships as an interlocking lattice of small white diamonds (an inverted triangle on a wide screen). Used wherever the site shows the
   dealers: the diamonds spring in from the middle when they arrive, bob in a slow ripple, and pointing at one sends a ripple through the rest.
   The dealers come from the dashboard (Website images > Dealerships on the website, published ones only); until the dashboard has any, the
   built-in list is shown. Subaru and Hyundai in the built-in list are white marks, so their diamonds are in their own brand blue.
   Phones get a narrower arrangement. */

type Dealer = { name: string; src?: string; live?: boolean };
const BRAND: Record<string, string> = { Subaru: '#10247c', Hyundai: '#002c5f' };

// Wide: rows of k, k-1, k-2 ... shifted one half-step each row (an inverted triangle). Narrow: alternating rows of three and two.
function layout(n: number) {
  let k = 1;
  while ((k * (k + 1)) / 2 < n) k++;
  const wide: number[][] = [];
  for (let r = 0, left = n; left > 0; r++) { const c = Math.min(k - r, left); for (let j = 0; j < c; j++) wide.push([r + 2 * j, r]); left -= c; }
  const narrow: number[][] = [];
  for (let r = 0, left = n; left > 0; r++) { const c = Math.min(r % 2 ? 2 : 3, left); for (let j = 0; j < c; j++) narrow.push([(r % 2) + 2 * j, r]); left -= c; }
  const rows = (pts: number[][]) => pts.reduce((m, p) => Math.max(m, p[1]), 0) + 1;
  return { wide, narrow, k, wideRows: rows(wide), narrowRows: rows(narrow) };
}

export function Dealers() {
  const root = useRef<HTMLDivElement>(null);
  const [dealers, setDealers] = useState<Dealer[]>(partners);

  // The published dealers from the dashboard replace the built-in list when there are any.
  useEffect(() => {
    let off = false;
    fetch('/api/v1/public/dealers', { cache: 'no-store', headers: { Accept: 'application/json' } })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((b) => { if (!off && Array.isArray(b.items) && b.items.length) setDealers(b.items.map((i: { name: string; mediaId: string }) => ({ name: i.name, src: mediaUrl(i.mediaId), live: true }))); })
      .catch(() => undefined);
    return () => { off = true; };
  }, []);

  const L = layout(dealers.length);

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
  }, { scope: root, dependencies: [dealers] });

  return (
    <div className="dl" ref={root}>
      <ul className="dl__sr">{dealers.map((p) => <li key={p.name}>{p.name}</li>)}</ul>
      <div className="dl__lat" aria-hidden="true" style={{ ['--wcols' as string]: L.k, ['--wrows' as string]: L.wideRows, ['--nrows' as string]: L.narrowRows }}>
        {dealers.map((p, i) => {
          const n = L.narrow[i] ?? [0, 0], w = L.wide[i] ?? [0, 0];
          const bg = p.live ? undefined : BRAND[p.name];
          return (
            <div className="dl__dia" key={p.name + i} style={{ ['--mc' as string]: n[0], ['--mr' as string]: n[1], ['--wc' as string]: w[0], ['--wr' as string]: w[1] }}>
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
