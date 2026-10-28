'use client';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { nav } from '../content';
import { gsap, LAY, useGSAP } from './gsap';
import { ArrowUpRight, Menu, X } from './Icons';

// The header is one long strip of tape laid across the top of the page. It tucks away while reading down and returns on the way up.
export default function Header() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [hidden, setHidden] = useState(false);
  const root = useRef<HTMLElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const toggle = useRef<HTMLButtonElement>(null);
  const current = (href: string) => (pathname === href || pathname.startsWith(href + '/') ? 'page' : undefined);

  useGSAP(() => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    gsap.from('.site-header__strip', { scaleX: 0, duration: 1.1, ease: 'expo.inOut', delay: 0.1 });
    gsap.from('.site-header__strip > *', { opacity: 0, y: 8, stagger: 0.05, duration: 0.6, delay: 0.7, ease: 'power3.out' });
    gsap.from('.site-header__book', { y: -70, rotate: 9, duration: 0.9, delay: 0.9, ease: LAY });
  }, { scope: root });

  useEffect(() => {
    let last = scrollY;
    const onScroll = () => {
      const y = scrollY;
      if (y > 240 && y > last + 4) setHidden(true);
      else if (y < last - 4) setHidden(false);
      last = y;
    };
    addEventListener('scroll', onScroll, { passive: true });
    return () => removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => setOpen(false), [pathname]);

  useGSAP(() => {
    const el = menu.current;
    if (!el) return;
    const items = el.querySelectorAll('a');
    if (open) {
      gsap.set(el, { visibility: 'visible' });
      gsap.fromTo(el, { clipPath: 'inset(0 0 100% 0)' }, { clipPath: 'inset(0 0 0% 0)', duration: 0.7, ease: 'expo.out' });
      gsap.fromTo(items, { y: 40, opacity: 0 }, { y: 0, opacity: 1, stagger: 0.05, duration: 0.7, delay: 0.1 });
      items[0]?.focus();
    } else if (getComputedStyle(el).visibility === 'visible') {
      gsap.to(el, { clipPath: 'inset(0 0 100% 0)', duration: 0.45, ease: 'power3.in', onComplete: () => { gsap.set(el, { visibility: 'hidden' }); } });
    }
  }, { dependencies: [open] });

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') { setOpen(false); toggle.current?.focus(); } };
    addEventListener('keydown', onKey);
    return () => removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <>
      <header ref={root} className={'site-header' + (hidden && !open ? ' is-hidden' : '')}>
        <div className="site-header__strip">
          <Link href="/" className="brand" aria-label="Dapper Detailing Pros, home">
            <img src="/site/assets/logo.webp" width={30} height={30} alt="" />
            <span>DAPPER<small>DETAILING PROS</small></span>
          </Link>
          <nav className="site-nav" aria-label="Main">
            {nav.map((item) => <Link key={item.href} href={item.href} aria-current={current(item.href)}>{item.label}</Link>)}
          </nav>
          <a className="site-header__account" href="/account">Account</a>
          <button ref={toggle} type="button" className="menu-btn" aria-expanded={open} aria-controls="mobile-menu" onClick={() => setOpen((o) => !o)}>
            {open ? <X size={20} aria-hidden /> : <Menu size={20} aria-hidden />}{open ? 'Close' : 'Menu'}
          </button>
          <a className="site-header__book" href="/booking">Book<ArrowUpRight size={18} aria-hidden /></a>
        </div>
      </header>
      <div ref={menu} id="mobile-menu" className="mobile-menu" aria-hidden={!open} inert={!open}>
        <Link href="/">Home</Link>
        {nav.map((item) => <Link key={item.href} href={item.href} aria-current={current(item.href)}>{item.label}</Link>)}
        <a href="/account">Account</a>
        <a className="btn btn--red" href="/booking">Book an appointment<ArrowUpRight size={20} aria-hidden /></a>
      </div>
    </>
  );
}
