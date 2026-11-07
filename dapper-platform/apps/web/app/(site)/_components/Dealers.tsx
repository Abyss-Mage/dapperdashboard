'use client';

import { useEffect, useRef, useState } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsapContact';
import { partners } from '../dapper';
import { mediaUrl } from './usePublished';
import { useHidden } from './useHidden';
import './dealers.css';

/* The dealerships as a tidy grid of white logo cards with the site's cut corner. Used wherever the site shows the dealers: the cards rise in
   one after another when they arrive.
   The dealers come from the dashboard (Website images > Dealerships on the website, published ones only); until the dashboard has any, the
   built-in list is shown. Subaru and Hyundai in the built-in list are white marks, so they are drawn in their own brand blue on the white diamond.
   Phones get two cards across. */

type Dealer = { name: string; src?: string; live?: boolean; wide?: boolean };

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

  useGSAP(() => {
    const mm = gsap.matchMedia();
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const tiles = gsap.utils.toArray<HTMLElement>('.dl__card');
      if (!tiles.length) return;
      gsap.set(tiles, { y: 24, opacity: 0 });
      const enter = ScrollTrigger.create({ trigger: '.dl__grid', start: 'top 85%', once: true, onEnter: () => {
        gsap.to(tiles, { y: 0, opacity: 1, duration: .7, ease: 'expo.out', stagger: .05 });
      } });
      return () => { enter.kill(); tiles.forEach((t) => gsap.killTweensOf(t)); gsap.set(tiles, { clearProps: 'transform,opacity' }); };
    });
    return () => mm.revert();
  }, { scope: root, dependencies: [shown.length] });

  return (
    <div className="dl" ref={root}>
      <ul className="dl__sr">{shown.map((p) => <li key={p.name}>{p.name}</li>)}</ul>
      <div className="dl__grid" aria-hidden="true">
        {shown.map((p, i) => (
          <div className="dl__card" key={p.name + i}>
            {p.src ? <img className={p.name.includes('McGee') ? 'is-crop' : p.wide ? 'is-wide' : undefined} src={p.src} alt="" loading="lazy" onLoad={p.live ? undefined : (e) => e.currentTarget.classList.toggle('is-wide', e.currentTarget.naturalWidth / e.currentTarget.naturalHeight > 6)} /> : <span className="dl__name">{p.name}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}
