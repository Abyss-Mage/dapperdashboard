'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { CreditCard, FileText, LogOut, Receipt, Wallet } from 'lucide-react';
import styles from './dealer.module.css';
import { API, readJson } from './lib';
import ForceDark from '../../_chrome/ForceDark';

/* The dealership portal's frame (error404 #26/#30): a sidebar (Invoices, Payments, Payment methods, Statement) and a top bar with the
   business and who is signed in. The sign-in page has no frame. */

type Me = { dealershipName: string; userName: string; viaLink: boolean; businessName: string };
const NAV = [
  { href: '/dealer', label: 'Invoices', icon: FileText },
  { href: '/dealer/payments', label: 'Payments', icon: Wallet },
  { href: '/dealer/cards', label: 'Payment methods', icon: CreditCard },
  { href: '/dealer/statement', label: 'Statement', icon: Receipt },
];

export default function DealerLayout({ children }: { children: React.ReactNode }) {
  const path = usePathname() ?? '';
  const bare = path.startsWith('/dealer/sign-in');
  const [me, setMe] = useState<Me | null>(null);
  useEffect(() => { if (!bare) fetch(`${API}/me`, { cache: 'no-store' }).then((r) => readJson<Me>(r)).then(setMe).catch(() => undefined); }, [bare]);

  if (bare) return <div className={styles.portal} data-no-motion=""><ForceDark theme="light" />{children}</div>;
  const active = (href: string) => (href === '/dealer' ? path === '/dealer' || path.startsWith('/dealer/invoices') : path.startsWith(href));
  const title = NAV.find((n) => active(n.href))?.label ?? 'Invoices';
  return (
    <div className={styles.portal} data-no-motion="">
      <ForceDark theme="light" />
      <aside className={styles.side}>
        <p className={styles.brand}>{me?.businessName || 'Dapper Detailing'}<span>Dealership account</span></p>
        <nav aria-label="Dealership account">
          {NAV.map(({ href, label, icon: Icon }) => (
            <Link key={href} href={href} className={active(href) ? styles.navOn : styles.nav} aria-current={active(href) ? 'page' : undefined}><Icon size={19} aria-hidden="true" />{label}</Link>
          ))}
        </nav>
      </aside>
      <div className={styles.body}>
        <div className={styles.top}>
          <h1>{title}</h1>
          <div className={styles.who}>
            <span><strong>{me?.dealershipName ?? ''}</strong>{me ? (me.viaLink ? 'Opened from your email link' : me.userName) : ''}</span>
            <button type="button" className={styles.ghost} onClick={async () => { await fetch(`${API}/logout`, { method: 'POST' }).catch(() => undefined); window.location.assign('/dealer/sign-in'); }}><LogOut size={17} aria-hidden="true" />Sign out</button>
          </div>
        </div>
        <div className={styles.main} role="main">{children}</div>
        <div className={styles.foot}>© {new Date().getFullYear()} {me?.businessName || 'Dapper Detailing'}</div>
      </div>
    </div>
  );
}
