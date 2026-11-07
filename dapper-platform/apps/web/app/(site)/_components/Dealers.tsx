'use client';

import { useEffect, useRef, useState } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsapContact';
import { partners } from '../dapper';
import { mediaUrl } from './usePublished';
import { useHidden } from './useHidden';
import './dealers.css';

/* The dealerships as an interlocking lattice of small white diamonds (an inverted triangle on a wide screen). Used wherever the site shows the
   dealers: the diamonds fade and grow in from the middle when they arrive, and one lifts a little under the pointer.
   The dealers come from the dashboard (Website images > Dealerships on the website, published ones only); until the dashboard has any, the
   built-in list is shown. Subaru and Hyundai in the built-in list are white marks, so they are drawn in their own brand blue on the white diamond.
   Phones get a narrower arrangement. */

type Dealer = { name: string; src?: string; live?: boolean; wide?: boolean };

// Wide: rows of k, k-1, k-2 ... shifted one half-step each row (an inverted triangle). Narrow: alternating rows of three and two.
function layout(n: number) {
  let k = 1;
  while ((k * (k + 1)) / 2 < n) k++;
  const wide: number[][] = [];
  for (let r = 0, left = n; left > 0; r++) { const c = Math.min(k - r, left); for (let j = 0; j < c; j++) wide.push([r + 2 * j, r]); left -= c; }
  const narrow: number[][] = [];
  for (let r = 0, left = n; left > 0; r++) { const c = Math.min(r % 2 ? 2 : 3, left); for (let j = 0; j < c; j++) narrow.push([(r % 2) + 2 * j, r]); left -= c; }
  const last = narrow[narrow.length - 1], above = narrow.filter((p) => last && p[1] === last[1] - 1), alone = narrow.filter((p) => last && p[1] === last[1]).length === 1;
  if (last && alone && above.length === 3) { last[0] = 2; last[1] += 1; }
  const rows = (pts: number[][]) => pts.reduce((m, p) => Math.max(m, p[1]), 0) + 1;
  return { wide, narrow, k, wideRows: rows(wide), narrowRows: rows(narrow) };
}

export function Dealers() {
  const root = useRef<HTMLDivElement>(null);
  const isHidden = useHidden();
  const [dealers, setDealers] = useState<Dealer[] | null>(null);

  // The published dealers from the dashboard replace the built-in list when there are any.
  useEffect(() => {
    let off = false;
    fetch('/api/v1/public/dealers', { cache: 'no-store', headers: { Accept: 'application/json' } })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((b) => { if (!off && Array.isArray(b.items) && b.items.length) setDealers(b.items.map((i: { name: string; mediaId: string }) => ({ name: i.name, src: mediaUrl(i.mediaId), live: true }))); })
      .catch(() => undefined);
    return () => { off = true; };
  }, []);

  const shown: Dealer[] = dealers ?? partners.filter((p) => !p.src || !isHidden(p.src));
  const L = layout(shown.length);

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const tiles = gsap.utils.toArray<HTMLElement>('.dl__dia');
      if (!tiles.length) return;
      gsap.set(tiles, { scale: .86, opacity: 0, transformOrigin: '50% 50%' });
      const enter = ScrollTrigger.create({ trigger: '.dl__lat', start: 'top 85%', once: true, onEnter: () => {
        gsap.to(tiles, { scale: 1, opacity: 1, duration: .7, ease: 'expo.out', stagger: { each: .05, from: 'center' } });
      } });
      return () => { enter.kill(); tiles.forEach((t) => gsap.killTweensOf(t)); gsap.set(tiles, { clearProps: 'transform,scale,opacity,transformOrigin' }); };
    });
    return () => mm.revert();
  }, { scope: root, dependencies: [shown.length] });

  return (
    <div className="dl" ref={root}>
      <ul className="dl__sr">{shown.map((p) => <li key={p.name}>{p.name}</li>)}</ul>
      <div className="dl__lat" aria-hidden="true" style={{ ['--wcols' as string]: L.k, ['--wrows' as string]: L.wideRows, ['--nrows' as string]: L.narrowRows }}>
        {shown.map((p, i) => {
          const n = L.narrow[i] ?? [0, 0], w = L.wide[i] ?? [0, 0];
          return (
            <div className="dl__dia" key={p.name + i} style={{ ['--mc' as string]: n[0], ['--mr' as string]: n[1], ['--wc' as string]: w[0], ['--wr' as string]: w[1] }}>
              <span className="dl__s">
                {p.src ? <img className={p.name.includes('McGee') ? 'is-crop' : p.wide ? 'is-wide' : undefined} src={p.src} alt="" loading="lazy" onLoad={p.live ? undefined : (e) => e.currentTarget.classList.toggle('is-wide', e.currentTarget.naturalWidth / e.currentTarget.naturalHeight > 6)} /> : <span className="dl__name">{p.name}</span>}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
