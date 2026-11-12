'use client';
import { useEffect, useRef, useState } from 'react';
import { ArrowRight } from 'lucide-react';
import { SITE_SLUG } from './site';
import { ScrollTrigger } from './gsap';

type Banner = { text: string; linkLabel: string; linkUrl: string; tone: string };

/* The sales banner set in the dashboard: a strip across the very top of every page. The site only receives it while it is switched on and
   before its end date. The floating header sits under the strip and moves up as the strip scrolls away (--banner-left, read by the header). */
export default function SalesStrip() {
  const [banner, setBanner] = useState<Banner | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/sites/' + SITE_SLUG, { cache: 'no-store', headers: { Accept: 'application/json' } })
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => { if (live && d?.banner?.text) setBanner(d.banner); })
      .catch(() => {});
    return () => { live = false; };
  }, []);
  useEffect(() => {
    const el = ref.current;
    if (!banner || !el) return;
    const root = document.documentElement.style;
    const place = () => {
      const h = el.offsetHeight;
      root.setProperty('--banner-h', h + 'px');
      root.setProperty('--banner-left', Math.max(0, h - scrollY) + 'px');
    };
    place();
    // The strip arrives after the page has laid out, so the pinned scroll scenes measure again.
    ScrollTrigger.refresh();
    addEventListener('scroll', place, { passive: true });
    addEventListener('resize', place);
    return () => {
      removeEventListener('scroll', place);
      removeEventListener('resize', place);
      root.removeProperty('--banner-h');
      root.removeProperty('--banner-left');
    };
  }, [banner]);
  if (!banner) return null;
  const external = banner.linkUrl.startsWith('https://');
  return (
    <div ref={ref} className="sales-strip" data-tone={banner.tone === 'black' ? 'black' : 'red'} role="region" aria-label="Offer">
      <p>{banner.text}</p>
      {banner.linkUrl && banner.linkLabel && (
        <a href={banner.linkUrl} {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}>{banner.linkLabel}<ArrowRight size={15} aria-hidden="true" /></a>
      )}
    </div>
  );
}
