'use client';
import { useEffect, useState } from 'react';

/* Whatever the owner has taken off the website stays off everywhere it appears, without each section having to know about it. */
export default function RemovedImages() {
  const [css, setCss] = useState('');
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/site-assets-hidden', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((b: { paths?: string[] } | null) => {
        const safe = (b?.paths ?? []).filter((p) => /^[A-Za-z0-9/_.-]+$/.test(p));
        if (live && safe.length) setCss(safe.map((p) => `img[src^="/site/assets/${p}"],video[src^="/site/assets/${p}"]`).join(',') + '{display:none!important}');
      })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);
  return css ? <style>{css}</style> : null;
}
