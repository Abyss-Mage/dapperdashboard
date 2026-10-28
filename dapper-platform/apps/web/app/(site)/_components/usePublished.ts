'use client';
import { useCallback, useEffect, useState } from 'react';

// Only published public projections enter the website: service summaries and portfolio photos. No staff or customer records.
export type PublishedService = { name: string; summary: string; category: string };
export type PublishedImage = { mediaId: string; title: string; alt: string; category: string };
type State = { status: 'loading' | 'ready' | 'error'; services: PublishedService[]; portfolio: PublishedImage[] };

export function usePublished() {
  const [state, setState] = useState<State>({ status: 'loading', services: [], portfolio: [] });
  const load = useCallback(() => {
    setState((s) => ({ ...s, status: 'loading' }));
    fetch('/api/v1/public/sites/dapper', { cache: 'no-store', headers: { Accept: 'application/json' } })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((data) => {
        if (!Array.isArray(data.services) || !Array.isArray(data.portfolio)) throw Error();
        setState({ status: 'ready', services: data.services, portfolio: data.portfolio });
      })
      .catch(() => setState({ status: 'error', services: [], portfolio: [] }));
  }, []);
  useEffect(load, [load]);
  return { ...state, retry: load };
}

export const mediaUrl = (id: string) => '/api/v1/public/media/' + encodeURIComponent(id);
