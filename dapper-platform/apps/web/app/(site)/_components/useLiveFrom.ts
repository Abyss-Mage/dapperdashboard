'use client';
import { useEffect, useState } from 'react';
import { SITE_SLUG } from './site';

/* Prices taken live from the dashboard. `from` is the "starting from" price of each vehicle category: the lowest price among the published
   base (main) services in that category. `price` is each service's own price by service id. Both are dollars. Nothing is returned until the
   website entry is published and services are priced, so the pages keep their built-in prices as the fallback. Prices are computed by the
   backend; the website only reads them. */
type SiteService = { serviceId: string; kind: string; category: string };
type OptionService = { id: string; name: string; kind: string; parentId: string | null; parentIds?: string[]; priceCents: number };
export type LiveAddon = { id: string; name: string; parentIds: string[]; priceCents: number };
type Live = { from: Record<string, number>; price: Record<string, number>; addons: LiveAddon[] };
const none: Live = { from: {}, price: {}, addons: [] };

export function useLiveCatalog(): Live {
  const [live, setLive] = useState<Live>(none);
  useEffect(() => {
    let off = false;
    const get = (u: string) => fetch(u, { cache: 'no-store', headers: { Accept: 'application/json' } }).then((r) => (r.ok ? r.json() : Promise.reject()));
    Promise.all([get('/api/v1/public/sites/' + SITE_SLUG), get('/api/v1/public/booking/' + SITE_SLUG + '/options')])
      .then(([site, options]) => {
        const price: Record<string, number> = {};
        for (const s of options.services as OptionService[]) if (s.priceCents > 0) price[s.id] = s.priceCents / 100;
        const from: Record<string, number> = {};
        for (const s of site.services as SiteService[]) {
          const dollars = price[s.serviceId];
          if (s.kind !== 'main' || !s.category || dollars === undefined) continue;
          if (from[s.category] === undefined || dollars < from[s.category]) from[s.category] = dollars;
        }
        const addons = (options.services as OptionService[]).filter((s) => s.kind === 'addon' && s.parentId && s.priceCents > 0).map((s) => ({ id: s.id, name: s.name, parentIds: s.parentIds?.length ? s.parentIds : [s.parentId as string], priceCents: s.priceCents }));
        if (!off) setLive({ from, price, addons });
      })
      .catch(() => undefined);
    return () => { off = true; };
  }, []);
  return live;
}

export const useLiveFrom = () => useLiveCatalog().from;
export const money = (n: number) => (Number.isInteger(n) ? String(n) : n.toFixed(2));
