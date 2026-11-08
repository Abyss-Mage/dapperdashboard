'use client';

import { useEffect, useRef, useState } from 'react';
import { gsap, ScrollTrigger, useGSAP } from './gsapContact';
import { partners } from '../dapper';
import { mediaUrl } from './usePublished';
import { useHidden, useLogoSizes } from './useHidden';
import './dealers.css';

/* The dealerships as a tidy grid of white logo cards with the site's cut corner. Used wherever the site shows the dealers: the cards rise in
   one after another when they arrive.
   The built-in dealers the owner has not removed come first, then the ones added in the dashboard (Website images > Dealership logos).
   Each logo is shown at the size chosen there. Phones get two cards across. */

type Dealer = { name: string; src?: string; live?: boolean; wide?: boolean; id?: string };

export function Dealers() {
  const root = useRef<HTMLDivElement>(null);
  const isHidden = useHidden();
  const sizes = useLogoSizes();
  const [dealers, setDealers] = useState<Dealer[]>([]);

  useEffect(() => {
    let off = false;
    fetch('/api/v1/public/dealers', { cache: 'no-store', headers: { Accept: 'application/json' } })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((b) => { if (!off && Array.isArray(b.items) && b.items.length) setDealers(b.items.map((i: { id: string; name: string; mediaId: string }) => ({ id: i.id, name: i.name, src: mediaUrl(i.mediaId), live: true }))); })
      .catch(() => undefined);
    return () => { off = true; };
  }, []);

  const shown: Dealer[] = [...partners.filter((p) => !p.src || !isHidden(p.src)), ...dealers];
  const sizeOf = (p: Dealer) => sizes[p.id ? 'dealers/' + p.id : (p.src ?? '').replace('/site/assets/', '')];

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
            {p.src ? <img className={sizeOf(p) ? 'dl-' + sizeOf(p) : p.name.includes('McGee') ? 'is-crop' : p.wide ? 'is-wide' : undefined} src={p.src} alt="" loading="lazy" onLoad={p.live || sizeOf(p) ? undefined : (e) => e.currentTarget.classList.toggle('is-wide', e.currentTarget.naturalWidth / e.currentTarget.naturalHeight > 6)} /> : <span className="dl__name">{p.name}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}
