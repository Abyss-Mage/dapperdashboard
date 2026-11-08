'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { ArrowRight, Bike, Bus, CalendarCheck, Car, CarFront, ChevronDown, Menu, Phone, Plane, Sailboat, Shield, Sparkles, Sun, Truck, UserRound, Wrench, X } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
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

const icons: Record<string, LucideIcon> = { '2-wheeler': Bike, hatchback: Car, sedan: Car, suv: CarFront, pickup: Truck, van: Bus, ceramic: Sparkles, ppf: Shield, aircraft: Plane, 'window-tint': Sun, boat: Sailboat };

/* The one header of the customer site: a white bar floating at the top (logo, centred links, Book now) that slides away while you scroll down
   and comes back as soon as you scroll up. Services opens a white panel under it with every service, its icon and a short line, and a help card.
   Phones: logo and menu button; the menu lists the services, and a bar along the bottom keeps Services, Book now, Call and Account in reach.
   Every "Book now" goes to the booking page, so the ad tags can follow the visit through to the thank-you page. */
export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [mega, setMega] = useState(false);
  const [svcOpen, setSvcOpen] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const lastY = useRef(0);
  const openedAt = useRef(0);
  const openMega = () => { setMega((m) => { if (!m) openedAt.current = Date.now(); return true; }); };
  const onVehicle = categories.some((c) => '/' + c.slug === pathname);

  useEffect(() => { setOpen(false); setMega(false); setSvcOpen(false); }, [pathname]);
  useEffect(() => {
    if (!mega) return;
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') setMega(false); };
    addEventListener('keydown', esc);
    return () => removeEventListener('keydown', esc);
  }, [mega]);
  useEffect(() => { document.documentElement.style.overflow = open ? 'hidden' : ''; return () => { document.documentElement.style.overflow = ''; }; }, [open]);
  // Slide away on the way down, come back on the way up; never while a menu is open.
  useEffect(() => {
    const on = () => {
      const y = scrollY;
      setScrolled(y > 8);
      if (Math.abs(y - lastY.current) > 6) { setHidden(y > lastY.current && y > 160); lastY.current = y; }
    };
    on();
    addEventListener('scroll', on, { passive: true });
    return () => removeEventListener('scroll', on);
  }, []);

  const current = (href: string) => (pathname === href ? 'page' : href === '/services' && onVehicle ? 'true' : undefined);
  // The booking page opens on a photo: the header sits on it with no bar, and the white bar returns once you scroll or open a menu.
  const onImage = pathname === '/booking' && !scrolled && !mega && !open;
  const cls = [s.header, scrolled ? s.scrolled : '', hidden && !mega && !open ? s.hidden : '', onImage ? s.onImage : ''].filter(Boolean).join(' ');

  return (
    <>
      <header className={cls} onMouseLeave={() => setMega(false)}>
        <div className={s.bar}>
          <Link href="/" className={s.brand} aria-label="Dapper Detailing Pros, home"><img src="/site/assets/logo.webp" alt="" width={46} height={46} /></Link>
          <nav className={s.nav} aria-label="Main">
            {links.map((l) => l.href === '/services'
              ? <span key={l.href} className={s.navItem + (mega ? ' ' + s.navOpen : '')} onMouseEnter={openMega}>
                  <Link href={l.href} aria-current={current(l.href)}>{l.label}</Link>
                  <button type="button" className={s.megaBtn} aria-expanded={mega} aria-controls="mega-services" aria-label={mega ? 'Close the services menu' : 'Open the services menu'} onClick={() => { if (Date.now() - openedAt.current < 400) { setMega(true); return; } setMega((m) => !m); }}><ChevronDown size={15} aria-hidden="true" /></button>
                </span>
              : <Link key={l.href} href={l.href} aria-current={current(l.href)} onMouseEnter={() => setMega(false)} onFocus={() => setMega(false)}>{l.label}</Link>)}
          </nav>
          <div className={s.actions}>
            <Link className={s.callBtn} href="/account" aria-current={pathname === '/account' ? 'page' : undefined}><UserRound size={15} aria-hidden="true" />My account</Link>
            <a className={s.book} href="/booking">Book now<ArrowRight size={16} aria-hidden="true" /></a>
          </div>
          <button type="button" className={s.menuBtn} aria-expanded={open} aria-controls="site-menu" aria-label={open ? 'Close menu' : 'Open menu'} onClick={() => setOpen((o) => !o)}>
            {open ? <X size={24} aria-hidden="true" /> : <Menu size={24} aria-hidden="true" />}
          </button>
        </div>
        {mega && <div id="mega-services" className={s.mega}>
          <div className={s.megaMain}>
            <p className={s.megaLabel}>Our services</p>
            <ul className={s.megaList}>
              {categories.map((c) => {
                const Icon = icons[c.key] ?? Car;
                return <li key={c.key}>
                  <Link href={'/' + c.slug} aria-current={pathname === '/' + c.slug ? 'page' : undefined} onClick={() => setMega(false)}>
                    <Icon size={18} aria-hidden="true" />
                    <span><strong>{c.title}</strong><small>{c.from ? 'From $' + c.from + ' · ' : ''}{c.summary}</small></span>
                  </Link>
                </li>;
              })}
            </ul>
          </div>
          <aside className={s.megaSide}>
            <p className={s.megaLabel}>Need a hand?</p>
            <div className={s.megaCard}>
              <strong>Not sure which package?</strong>
              <p>Tell us what you drive and how it looks today. We will recommend the right service and give you a price.</p>
              <a href={'tel:' + contact.tel}>Call {contact.phone}<ArrowRight size={14} aria-hidden="true" /></a>
              <Link href="/services" onClick={() => setMega(false)}>See all services<ArrowRight size={14} aria-hidden="true" /></Link>
            </div>
          </aside>
        </div>}
      </header>

      <div id="site-menu" className={s.menu + (open ? ' ' + s.open : '')} inert={!open}>
        <nav className={s.menuNav} aria-label="Menu">
          {links.map((l) => l.href === '/services'
            ? <div key={l.href} className={s.menuSvc}>
                <button type="button" aria-expanded={svcOpen} onClick={() => setSvcOpen((v) => !v)}>Services<ChevronDown size={24} aria-hidden="true" /></button>
                {svcOpen && <div className={s.menuSvcList}>
                  {categories.map((c) => <Link key={c.key} href={'/' + c.slug}>{c.title}</Link>)}
                  <Link href="/services">All services</Link>
                </div>}
              </div>
            : <Link key={l.href} href={l.href} aria-current={current(l.href)}>{l.label}</Link>)}
          <Link href="/account">My account</Link>
        </nav>
        <div className={s.menuActs}>
          <a className={s.callBtn} href={'tel:' + contact.tel}><Phone size={18} aria-hidden="true" />{contact.phone}</a>
          <a className={s.book + ' ' + s.menuBook} href="/booking">Book now<ArrowRight size={18} aria-hidden="true" /></a>
        </div>
      </div>

      {/* The booking pages have their own bar at the bottom of the screen, so the quick-action bar steps aside there. */}
      {!pathname.startsWith('/booking') && !open && <nav className={s.dock} aria-label="Quick actions">
        <a href="/services"><Wrench size={20} aria-hidden="true" />Services</a>
        <a className={s.dockBook} href="/booking"><CalendarCheck size={20} aria-hidden="true" />Book now</a>
        <a href={'tel:' + contact.tel}><Phone size={20} aria-hidden="true" />Call</a>
        <a href="/account"><UserRound size={20} aria-hidden="true" />Account</a>
      </nav>}
    </>
  );
}
