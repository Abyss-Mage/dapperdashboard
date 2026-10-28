'use client';

import { useEffect } from 'react';
import gsap from 'gsap';

/* The product's entrance motion, mounted once for every page. Whenever cards, rows, tiles, choices or tables are added to the
   page they rise in with a short stagger; anything below the fold waits and rises as it is scrolled into view. Everything is
   built from transform and opacity (cheap for the browser) and cleaned up afterwards so hover effects keep working.
   Content that was already on the page when it loaded is left alone (no flash), and nothing runs for reduced motion. */

const ENTER = [
  '.panel', '.detail-card', 'article', 'tbody > tr', '.management-list > *',
  '[class*="_card__"]', '[class*="_tile__"]', '[class*="_person__"]', '[class*="_choice__"]', '[class*="_step__"]', '[class*="_slot__"]',
  '[class*="_panel__"]', '[class*="_side__"]', '[class*="_svc__"]', '[class*="_svcGroup__"]', '[class*="_stage__"]', '[class*="_row__"]',
  '[class*="_docItem__"]', '[class*="_invRow__"]', '[class*="_postCard__"]', '[class*="_recentCard__"]', '[class*="_photoTile__"]', '[class*="_addTile__"]', '[class*="_menuGrid__"] > *', '[class*="_accountGrid__"] > *', '[class*="_documentsCard__"]', '[class*="_hero__"]', '[class*="_flatCard__"]', '[class*="_resource__"]', '[class*="_barRow__"]', '[class*="_calendarDetails__"]', '[class*="_stage__"]',
].join(',');
const SKIP = '[data-no-motion], .dd-invoice-print-root, [class*="_pop__"], [class*="_monthGrid__"], [class*="_calendarAgenda__"], [class*="_skeleton__"], [role="listbox"], [class*="_drawer__"]';
const MAX_HIDDEN = 250;

const reduced = () => matchMedia('(prefers-reduced-motion: reduce)').matches;
const CLEAR = 'opacity,visibility,transform';

export default function MotionSystem() {
  useEffect(() => {
    const seen = new WeakSet<Element>();
    const waiting = new Set<Element>();
    const reveal = new IntersectionObserver((entries) => {
      const ready = entries.filter((e) => e.isIntersecting).map((e) => e.target as HTMLElement);
      if (!ready.length) return;
      ready.forEach((el) => { reveal.unobserve(el); waiting.delete(el); });
      gsap.to(ready, { autoAlpha: 1, y: 0, scale: 1, duration: 0.55, ease: 'power3.out', stagger: 0.05, clearProps: CLEAR, onComplete: () => ready.forEach((el) => el.removeAttribute('data-mh')) });
    }, { threshold: 0, rootMargin: '0px 0px 14% 0px' });

    let queue: Element[] = [], scheduled = false;
    const flush = () => {
      scheduled = false;
      const nodes = queue; queue = [];
      if (reduced() || document.hidden) return;
      const found = new Set<HTMLElement>();
      for (const node of nodes) {
        if (!node.isConnected) continue;
        if (node.matches(ENTER)) found.add(node as HTMLElement);
        node.querySelectorAll<HTMLElement>(ENTER).forEach((el) => found.add(el));
      }
      const fresh = [...found].filter((el) => !seen.has(el) && !el.closest(SKIP));
      fresh.forEach((el) => seen.add(el));
      // Only the outermost new element animates; its contents ride along.
      const outer = fresh.filter((el) => !fresh.some((other) => other !== el && other.contains(el)));
      const vh = window.innerHeight, now: HTMLElement[] = [];
      let hidden = waiting.size;
      for (const el of outer) {
        const r = el.getBoundingClientRect();
        if (r.width === 0 && r.height === 0) continue; // not displayed
        if (r.top < vh * 1.02 && r.bottom > -40) now.push(el);
        else if (hidden < MAX_HIDDEN) { hidden++; waiting.add(el); el.setAttribute('data-mh', ''); gsap.set(el, { autoAlpha: 0, y: 22, scale: 0.985 }); reveal.observe(el); }
      }
      if (!now.length) return;
      now.sort((a, b) => { const ra = a.getBoundingClientRect(), rb = b.getBoundingClientRect(); return ra.top - rb.top || ra.left - rb.left; });
      gsap.fromTo(now.slice(0, 60), { autoAlpha: 0, y: 18, scale: 0.985 }, { autoAlpha: 1, y: 0, scale: 1, duration: 0.6, ease: 'power3.out', stagger: { amount: Math.min(0.6, now.length * 0.05) }, clearProps: CLEAR, overwrite: 'auto' });
    };

    const observer = new MutationObserver((records) => {
      for (const rec of records) rec.addedNodes.forEach((n) => { if (n.nodeType === 1) queue.push(n as Element); });
      if (queue.length && !scheduled) { scheduled = true; requestAnimationFrame(flush); }
    });
    // Start after the first paint so server-rendered content is not hidden and re-shown.
    const start = window.setTimeout(() => observer.observe(document.body, { childList: true, subtree: true }), 400);
    return () => { window.clearTimeout(start); observer.disconnect(); reveal.disconnect(); };
  }, []);
  return null;
}
