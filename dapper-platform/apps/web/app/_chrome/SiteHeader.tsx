'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { ArrowRight, CalendarCheck, ChevronDown, Menu, Phone, UserRound, Wrench, X } from 'lucide-react';
import '@fontsource-variable/unbounded/index.css';
import { categories } from '../(site)/content';
import { contact } from '../(site)/dapper';
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
  const [mega, setMega] = useState(false);
  const onVehicle = categories.some((c) => '/' + c.slug === pathname);
  useEffect(() => { setOpen(false); setMega(false); }, [pathname]);
  useEffect(() => {
    if (!mega) return;
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') setMega(false); };
    addEventListener('keydown', esc);
    return () => removeEventListener('keydown', esc);
  }, [mega]);
  useEffect(() => { document.documentElement.style.overflow = open ? 'hidden' : ''; return () => { document.documentElement.style.overflow = ''; }; }, [open]);
  const current = (href: string) => (pathname === href ? 'page' : href === '/services' && onVehicle ? 'true' : undefined);
  return (
    <>
      <header className={s.header} onMouseLeave={(e) => { if (!(e.relatedTarget instanceof Node) || !document.getElementById('mega-services')?.contains(e.relatedTarget)) setMega(false); }}>
        <Link href="/" className={s.brand} aria-label="Dapper Detailing Pros, home"><img src="/site/assets/logo.webp" alt="" width={48} height={48} /></Link>
        <nav className={s.nav} aria-label="Main">
          {links.map((l) => l.href === '/services'
            ? <span key={l.href} className={s.navItem} onMouseEnter={() => setMega(true)}>
                <Link href={l.href} aria-current={current(l.href)} onFocus={() => setMega(true)}>{l.label}</Link>
                <button type="button" className={s.megaBtn} aria-expanded={mega} aria-controls="mega-services" aria-label={mega ? 'Close the services menu' : 'Open the services menu'} onClick={() => setMega((m) => !m)}><ChevronDown size={16} aria-hidden="true" /></button>
              </span>
            : <Link key={l.href} href={l.href} aria-current={current(l.href)} onMouseEnter={() => setMega(false)} onFocus={() => setMega(false)}>{l.label}</Link>)}
        </nav>
        <div className={s.actions}>
          <Link href="/account" className={s.account}><UserRound size={18} aria-hidden="true" />Account</Link>
          <a className={s.book} href="/booking">Book now<ArrowRight size={16} aria-hidden="true" /></a>
        </div>
        <button type="button" className={s.menuBtn} aria-expanded={open} aria-controls="site-menu" aria-label={open ? 'Close menu' : 'Open menu'} onClick={() => setOpen((o) => !o)}>
          {open ? <X size={26} aria-hidden="true" /> : <Menu size={26} aria-hidden="true" />}
        </button>
      </header>
      {mega && <div id="mega-services" className={s.mega} onMouseLeave={() => setMega(false)}>
        <div className={s.megaGrid}>
          {categories.map((cat) => <Link key={cat.key} href={'/' + cat.slug} className={s.megaCard} onClick={() => setMega(false)}>
            <span className={s.pic}><img src={cat.image} alt="" loading="lazy" width={320} height={200} /></span>
            <span><strong>{cat.title}</strong>{cat.from ? <small>From ${cat.from}</small> : <small>Ask for a price</small>}</span>
          </Link>)}
        </div>
        <div className={s.megaFoot}><Link href="/services" onClick={() => setMega(false)}>See all services</Link><a href="/booking" className={s.book}>Book now<ArrowRight size={16} aria-hidden="true" /></a></div>
      </div>}
      <div id="site-menu" className={s.menu + (open ? ' ' + s.open : '')} inert={!open}>
        {links.map((l) => <Link key={l.href} href={l.href}>{l.label}</Link>)}
        <Link href="/account">My account</Link>
        <a className={s.book + ' ' + s.menuBook} href="/booking">Book now<ArrowRight size={18} aria-hidden="true" /></a>
      </div>
      {/* The booking pages have their own bar at the bottom of the screen, so the quick-action bar steps aside there. */}
      {!pathname.startsWith('/booking') && <nav className={s.dock} aria-label="Quick actions">
        <a href="/services"><Wrench size={20} aria-hidden="true" />Services</a>
        <a className={s.dockBook} href="/booking"><CalendarCheck size={20} aria-hidden="true" />Book now</a>
        <a href={'tel:' + contact.tel}><Phone size={20} aria-hidden="true" />Call</a>
        <a href="/account"><UserRound size={20} aria-hidden="true" />Account</a>
      </nav>}
    </>
  );
}
