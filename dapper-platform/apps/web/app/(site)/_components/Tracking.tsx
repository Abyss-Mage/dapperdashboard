'use client';
import { useEffect, useRef, useState } from 'react';
import { usePathname } from 'next/navigation';

/* Loads the marketing tags the owner saved under Settings > Website tracking: Google Tag Manager, the Meta Pixel and the
   Mailchimp popup. Nothing loads for an identifier that is empty. The server accepts only identifiers and a Mailchimp
   address, never pasted scripts. */
type Snippet = { id: string; placement: 'head' | 'body'; code: string; paths: string[] };
type Settings = { gtmId?: string; metaPixelId?: string; mailchimpScriptUrl?: string; scripts?: Snippet[] };
type Win = Window & { dataLayer?: unknown[]; fbq?: ((...args: unknown[]) => void) & { callMethod?: (...args: unknown[]) => void; queue?: unknown[]; loaded?: boolean; version?: string; push?: unknown }; _fbq?: unknown };

function addScript(src: string, id: string) {
  if (document.getElementById(id)) return;
  const el = document.createElement('script');
  el.id = id; el.async = true; el.src = src;
  document.head.appendChild(el);
}

/* Adds one custom snippet (Settings > Custom scripts) to the page. Markup is parsed in an inert template, then script tags are
   rebuilt so the browser runs them, because scripts inserted as plain HTML never run. Each snippet runs once per page load. */
function runSnippet(sn: Snippet) {
  const target = sn.placement === 'head' ? document.head : document.body;
  const tpl = document.createElement('template');
  tpl.innerHTML = sn.code;
  tpl.content.childNodes.forEach((node) => {
    if (node.nodeType === Node.ELEMENT_NODE && (node as Element).tagName === 'SCRIPT') {
      const old = node as HTMLScriptElement;
      const el = document.createElement('script');
      for (const a of Array.from(old.attributes)) el.setAttribute(a.name, a.value);
      el.text = old.text;
      el.setAttribute('data-dd-custom', sn.id);
      target.appendChild(el);
    } else if (node.nodeType === Node.ELEMENT_NODE) {
      const el = (node as Element).cloneNode(true) as Element;
      el.setAttribute('data-dd-custom', sn.id);
      target.appendChild(el);
    }
  });
}

const pathMatches = (paths: string[], current: string) => paths.length === 0 || paths.some((p) => (p === '/' ? current === '/' : current === p || current.startsWith(p.endsWith('/') ? p : p + '/')));

export default function Tracking() {
  const pathname = usePathname();
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const ran = useRef<Set<string>>(new Set());
  // Snippets that match the current page run once; moving to another page runs the ones that now match.
  useEffect(() => {
    for (const sn of snippets) {
      if (ran.current.has(sn.id) || !pathMatches(sn.paths, pathname)) continue;
      ran.current.add(sn.id);
      try { runSnippet(sn); } catch { /* a faulty snippet must not break the page */ }
    }
  }, [snippets, pathname]);
  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/tracking', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((s: Settings | null) => {
        if (!live || !s) return;
        if (Array.isArray(s.scripts)) setSnippets(s.scripts);
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
