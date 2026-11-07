'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { ArrowRight, ArrowUpRight, CalendarCheck, ChevronDown, Mail, MapPin, Menu, Phone, UserRound, Wrench, X } from 'lucide-react';
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

/* The one header of the customer site.
   Wide screens: a slim info strip (directions, phone, email, account) over the main bar (logo, centred links, call, Book now). The header slides
   away while you scroll down and comes back as soon as you scroll up. Services opens a two-part menu: the list on the left, a photo preview of
   the one you point at on the right.
   Phones: logo and menu button; the menu lists the services, and a bar along the bottom keeps Services, Book now, Call and Account in reach.
   Every "Book now" goes to the booking page, so the ad tags can follow the visit through to the thank-you page. */
export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [mega, setMega] = useState(false);
  const [svcOpen, setSvcOpen] = useState(false);
  const [pick, setPick] = useState(categories[0].key);
  const [hidden, setHidden] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const lastY = useRef(0);
  const onVehicle = categories.some((c) => '/' + c.slug === pathname);
  const shown = categories.find((c) => c.key === pick) ?? categories[0];

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
  const cls = [s.header, scrolled ? s.scrolled : '', hidden && !mega && !open ? s.hidden : ''].filter(Boolean).join(' ');

  return (
    <>
      <header className={cls} onMouseLeave={(e) => { if (!(e.relatedTarget instanceof Node) || !document.getElementById('mega-services')?.contains(e.relatedTarget)) setMega(false); }}>
        <div className={s.strip}>
          <span className={s.stripNote}>Mobile and in-shop detailing</span>
          <a href={contact.directions} target="_blank" rel="noopener noreferrer"><MapPin size={14} aria-hidden="true" />Get directions</a>
          <span className={s.stripEnd}>
            <a href={'mailto:' + contact.email}><Mail size={14} aria-hidden="true" />{contact.email}</a>
            <Link href="/account"><UserRound size={14} aria-hidden="true" />My account</Link>
          </span>
        </div>
        <div className={s.bar}>
          <Link href="/" className={s.brand} aria-label="Dapper Detailing Pros, home"><img src="/site/assets/logo.webp" alt="" width={52} height={52} /></Link>
          <nav className={s.nav} aria-label="Main">
            {links.map((l) => l.href === '/services'
              ? <span key={l.href} className={s.navItem} onMouseEnter={() => setMega(true)}>
                  <Link href={l.href} aria-current={current(l.href)}>{l.label}</Link>
                  <button type="button" className={s.megaBtn} aria-expanded={mega} aria-controls="mega-services" aria-label={mega ? 'Close the services menu' : 'Open the services menu'} onClick={() => setMega((m) => !m)}><ChevronDown size={16} aria-hidden="true" /></button>
                </span>
              : <Link key={l.href} href={l.href} aria-current={current(l.href)} onMouseEnter={() => setMega(false)} onFocus={() => setMega(false)}>{l.label}</Link>)}
          </nav>
          <div className={s.actions}>
            <a className={s.callBtn} href={'tel:' + contact.tel}><Phone size={16} aria-hidden="true" />{contact.phone}</a>
            <a className={s.book} href="/booking">Book now<ArrowRight size={16} aria-hidden="true" /></a>
          </div>
          <button type="button" className={s.menuBtn} aria-expanded={open} aria-controls="site-menu" aria-label={open ? 'Close menu' : 'Open menu'} onClick={() => setOpen((o) => !o)}>
            {open ? <X size={26} aria-hidden="true" /> : <Menu size={26} aria-hidden="true" />}
          </button>
        </div>
        {mega && <div id="mega-services" className={s.mega}>
          <ul className={s.megaList}>
            {categories.map((c) => <li key={c.key}>
              <Link href={'/' + c.slug} aria-current={pathname === '/' + c.slug ? 'page' : undefined} className={c.key === shown.key ? s.megaOn : undefined} onMouseEnter={() => setPick(c.key)} onFocus={() => setPick(c.key)} onClick={() => setMega(false)}>
                <span>{c.title}</span><small>{c.from ? 'From $' + c.from : 'Ask for a price'}</small>
              </Link>
            </li>)}
          </ul>
          <div className={s.megaPreview}>
            <img key={shown.key} src={shown.image} alt="" width={720} height={450} />
            <div className={s.megaCopy}>
              <strong>{shown.title}</strong>
              <p>{shown.summary}</p>
              <span className={s.megaActs}>
                <Link href={'/' + shown.slug} onClick={() => setMega(false)}>View packages<ArrowUpRight size={15} aria-hidden="true" /></Link>
                <a className={s.book} href={'/booking?category=' + shown.key}>Book {shown.short.toLowerCase()}<ArrowRight size={16} aria-hidden="true" /></a>
              </span>
            </div>
          </div>
          <div className={s.megaFoot}><Link href="/services" onClick={() => setMega(false)}>See all services<ArrowUpRight size={15} aria-hidden="true" /></Link><span>Not sure what you need? Call <a href={'tel:' + contact.tel}>{contact.phone}</a></span></div>
        </div>}
      </header>

      <div id="site-menu" className={s.menu + (open ? ' ' + s.open : '')} inert={!open}>
        <nav className={s.menuNav} aria-label="Menu">
          {links.map((l) => l.href === '/services'
            ? <div key={l.href} className={s.menuSvc}>
                <button type="button" aria-expanded={svcOpen} onClick={() => setSvcOpen((v) => !v)}>Services<ChevronDown size={26} aria-hidden="true" /></button>
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
      {!pathname.startsWith('/booking') && <nav className={s.dock} aria-label="Quick actions">
        <a href="/services"><Wrench size={20} aria-hidden="true" />Services</a>
        <a className={s.dockBook} href="/booking"><CalendarCheck size={20} aria-hidden="true" />Book now</a>
        <a href={'tel:' + contact.tel}><Phone size={20} aria-hidden="true" />Call</a>
        <a href="/account"><UserRound size={20} aria-hidden="true" />Account</a>
      </nav>}
    </>
  );
}
