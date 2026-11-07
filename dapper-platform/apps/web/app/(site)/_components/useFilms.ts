'use client';
import { useEffect, useState } from 'react';
import { useHidden } from './useHidden';

/* The films shown on Home and Portfolio: the owner's own films (added in the dashboard) when there are any, else the built-in ones. */
export function useFilms(builtIn: string[]) {
  const [clips, setClips] = useState<string[] | null>(null);
  const isHidden = useHidden();
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/films', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((b: { items?: string[] } | null) => { if (live && b && Array.isArray(b.items) && b.items.length) setClips(b.items); })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);
  return clips ?? builtIn.filter((c) => !isHidden(c));
}
