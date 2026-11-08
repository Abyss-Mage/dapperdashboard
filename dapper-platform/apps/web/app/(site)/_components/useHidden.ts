'use client';
import { useEffect, useState } from 'react';

/* The built-in pictures, logos and films the owner has taken off the website (paths like "portfolio/portfolio-03.jpg"). */
export function useHidden() {
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/site-assets-hidden', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((b: { paths?: string[] } | null) => { if (live && b && Array.isArray(b.paths) && b.paths.length) setHidden(new Set(b.paths)); })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);
  return (src: string) => hidden.has(src.replace(/^\/site\/assets\//, ''));
}

/* The size the owner chose for each dealership logo: built-in logos by path, uploaded ones as "dealers/<id>". */
export function useLogoSizes() {
  const [sizes, setSizes] = useState<Record<string, string>>({});
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/site-assets-hidden', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((b: { logoSizes?: Record<string, string> } | null) => { if (live && b?.logoSizes) setSizes(b.logoSizes); })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);
  return sizes;
}
