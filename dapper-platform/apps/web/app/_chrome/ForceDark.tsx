'use client';
import { useEffect } from 'react';

/* Pages that sit under the site's black header use the dark theme, and a popup must not pass its scrolling on to the page behind it. */
export default function ForceDark() {
  useEffect(() => {
    const root = document.documentElement;
    const was = root.dataset.theme;
    root.dataset.theme = 'dark';
    root.style.overscrollBehavior = 'contain';
    return () => { if (was) root.dataset.theme = was; else delete root.dataset.theme; root.style.overscrollBehavior = ''; };
  }, []);
  return null;
}
