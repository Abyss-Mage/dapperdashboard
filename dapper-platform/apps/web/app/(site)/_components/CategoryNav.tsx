'use client';
import Link from 'next/link';
import { useEffect, useRef } from 'react';
import { categories } from '../content';

// The category list. On phones it is a sideways scroller, so the current category is scrolled into view on arrival.
export default function CategoryNav({ current }: { current: string }) {
  const nav = useRef<HTMLElement>(null);
  useEffect(() => {
    const el = nav.current;
    const active = el?.querySelector<HTMLElement>('[aria-current=page]');
    if (el && active && el.scrollWidth > el.clientWidth) el.scrollLeft = active.offsetLeft - 16;
  }, [current]);
  return (
    <nav ref={nav} className="cat-nav" aria-label="Service categories">
      {categories.map((x) => <Link key={x.slug} href={'/' + x.slug} aria-current={x.slug === current ? 'page' : undefined}>{x.title}</Link>)}
    </nav>
  );
}
