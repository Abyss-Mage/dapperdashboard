'use client';
import { useEffect } from 'react';

/* Pages that sit under the site's black header use the dark theme, and a popup must not pass its scrolling on to the page behind it. */
export default function ForceDark({ theme = 'dark' }: { theme?: 'dark' | 'light' }) {
  useEffect(() => {
    const root = document.documentElement;
    const was = root.dataset.theme;
    root.dataset.theme = theme;
    root.style.overscrollBehavior = 'contain';
    return () => { if (was) root.dataset.theme = was; else delete root.dataset.theme; root.style.overscrollBehavior = ''; };
  }, [theme]);
  return null;
}
