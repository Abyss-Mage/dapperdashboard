'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { ArrowRight, Menu, UserRound, X } from 'lucide-react';
import '@fontsource-variable/unbounded/index.css';
import { categories } from '../(site)/content';
import s from './chrome.module.css';

const links = [
  { href: '/', label: 'Home' },
  { href: '/services', label: 'Services' },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/about-us', label: 'About' },
  { href: '/contact-us', label: 'Contact' },
];

/* The one header: a black bar on a wide screen (links, Account, Book now) and, on a phone, the name with a full-screen menu plus a bottom
   bar whose big button is Book now. Every "Book now" goes to the booking page, so the ad tags can follow the visit through to the thank-you page. */
export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const onVehicle = categories.some((c) => '/' + c.slug === pathname);
  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => { document.documentElement.style.overflow = open ? 'hidden' : ''; return () => { document.documentElement.style.overflow = ''; }; }, [open]);
  const current = (href: string) => (pathname === href ? 'page' : href === '/services' && onVehicle ? 'true' : undefined);
  return (
    <>
      <header className={s.header}>
        <Link href="/" className={s.brand} aria-label="Dapper Detailing Pros, home">Dapper</Link>
        <nav className={s.nav} aria-label="Main">
          {links.map((l) => <Link key={l.href} href={l.href} aria-current={current(l.href)}>{l.label}</Link>)}
        </nav>
        <div className={s.actions}>
          <Link href="/account" className={s.account}><UserRound size={18} aria-hidden="true" />Account</Link>
          <a className={s.book} href="/booking">Book now<ArrowRight size={16} aria-hidden="true" /></a>
        </div>
        <a className={s.bookMini} href="/booking">Book now</a>
        <button type="button" className={s.menuBtn} aria-expanded={open} aria-controls="site-menu" aria-label={open ? 'Close menu' : 'Open menu'} onClick={() => setOpen((o) => !o)}>
          {open ? <X size={26} aria-hidden="true" /> : <Menu size={26} aria-hidden="true" />}
        </button>
      </header>
      <div id="site-menu" className={s.menu + (open ? ' ' + s.open : '')} inert={!open}>
        {links.map((l) => <Link key={l.href} href={l.href}>{l.label}</Link>)}
        <Link href="/account">My account</Link>
        <a className={s.book + ' ' + s.menuBook} href="/booking">Book now<ArrowRight size={18} aria-hidden="true" /></a>
      </div>
    </>
  );
}
