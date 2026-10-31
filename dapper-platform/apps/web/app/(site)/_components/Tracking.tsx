'use client';
import { useEffect } from 'react';

/* Loads the marketing tags the owner saved under Settings > Website tracking: Google Tag Manager, the Meta Pixel and the
   Mailchimp popup. Nothing loads for an identifier that is empty. The server accepts only identifiers and a Mailchimp
   address, never pasted scripts. */
type Settings = { gtmId?: string; metaPixelId?: string; mailchimpScriptUrl?: string };
type Win = Window & { dataLayer?: unknown[]; fbq?: ((...args: unknown[]) => void) & { callMethod?: (...args: unknown[]) => void; queue?: unknown[]; loaded?: boolean; version?: string; push?: unknown }; _fbq?: unknown };

function addScript(src: string, id: string) {
  if (document.getElementById(id)) return;
  const el = document.createElement('script');
  el.id = id; el.async = true; el.src = src;
  document.head.appendChild(el);
}

export default function Tracking() {
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/tracking', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((s: Settings | null) => {
        if (!live || !s) return;
        const w = window as Win;
        if (s.gtmId && /^GTM-[A-Z0-9]{4,12}$/.test(s.gtmId)) {
          w.dataLayer = w.dataLayer || [];
          w.dataLayer.push({ 'gtm.start': Date.now(), event: 'gtm.js' });
          addScript('https://www.googletagmanager.com/gtm.js?id=' + encodeURIComponent(s.gtmId), 'dd-gtm');
        }
        if (s.metaPixelId && /^[0-9]{8,20}$/.test(s.metaPixelId) && !w.fbq) {
          const q: unknown[] = [];
          const fbq = ((...args: unknown[]) => { if (fbq.callMethod) fbq.callMethod(...args); else q.push(args); }) as NonNullable<Win['fbq']>;
          fbq.push = fbq; fbq.loaded = true; fbq.version = '2.0'; fbq.queue = q;
          w.fbq = fbq; w._fbq = fbq;
          addScript('https://connect.facebook.net/en_US/fbevents.js', 'dd-meta-pixel');
          fbq('init', s.metaPixelId);
          fbq('track', 'PageView');
        }
        if (s.mailchimpScriptUrl && /^https:\/\/(chimpstatic\.com|[a-z0-9-]+\.list-manage\.com)\//.test(s.mailchimpScriptUrl)) {
          addScript(s.mailchimpScriptUrl, 'dd-mailchimp');
        }
      })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);
  return null;
}
