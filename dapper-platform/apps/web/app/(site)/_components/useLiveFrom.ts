'use client';
import { useEffect, useState } from 'react';

/* The "starting from" price of each vehicle category, taken live from the dashboard: the lowest price among the published base services in
   that category. Returns dollars by category key, and nothing until the website entry is published and a service is priced, so the pages
   keep their built-in prices as the fallback. Prices are computed by the backend; the website only reads them. */
type SiteService = { serviceId: string; kind: string; category: string };
type OptionService = { id: string; kind: string; priceCents: number };

export function useLiveFrom(): Record<string, number> {
  const [from, setFrom] = useState<Record<string, number>>({});
  useEffect(() => {
    let off = false;
    const get = (u: string) => fetch(u, { cache: 'no-store', headers: { Accept: 'application/json' } }).then((r) => (r.ok ? r.json() : Promise.reject()));
    Promise.all([get('/api/v1/public/sites/dapper'), get('/api/v1/public/booking/dapper/options')])
      .then(([site, options]) => {
        const price = new Map<string, number>((options.services as OptionService[]).filter((s) => s.kind === 'main' && s.priceCents > 0).map((s) => [s.id, s.priceCents]));
        const out: Record<string, number> = {};
        for (const s of site.services as SiteService[]) {
          const cents = price.get(s.serviceId);
          if (s.kind !== 'main' || !s.category || cents === undefined) continue;
          const dollars = cents / 100;
          if (out[s.category] === undefined || dollars < out[s.category]) out[s.category] = dollars;
        }
        if (!off) setFrom(out);
      })
      .catch(() => undefined);
    return () => { off = true; };
  }, []);
  return from;
}

export const money = (n: number) => (Number.isInteger(n) ? String(n) : n.toFixed(2));
