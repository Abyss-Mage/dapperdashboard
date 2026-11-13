'use client';

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import gsap from 'gsap';
import Link from 'next/link';
import ThemeToggle from '../ThemeToggle';
import AuthShell from '../AuthShell';
import { OwnerIcon, type OwnerIconName } from './OwnerIcons';

// Operational & Management Component Imports
import OwnerOverview from './OwnerOverview';
import PeopleAccess from './PeopleAccess';
import EmployeeUploadsReview from '../workspace/EmployeeUploadsReview';
import CustomerManager from './CustomerManager';
import StaffBookingWizard from '../workspace/StaffBookingWizard';
import InvoiceManager from '../workspace/InvoiceManager';
import SOPCompliancePanel from '../workspace/SOPCompliancePanel';
import InvoiceTemplateEditor from '../workspace/InvoiceTemplateEditor';
import WebsiteTrackingEditor from './WebsiteTrackingEditor';
import InvoiceIssuerEditor from '../workspace/InvoiceIssuerEditor';
import Enquiries from './Enquiries';
import SalesBanner from './SalesBanner';
import DealerInvoices from './DealerInvoices';
import UpgradeOffers from './UpgradeOffers';
import Coupons from './Coupons';
import Reports from './Reports';
import CustomScripts from './CustomScripts';
import WebsiteImages from './WebsiteImages';
import MessagingSettings from './MessagingSettings';
import Employees from './Employees';
import FranchiseEmployees from './FranchiseEmployees';
import ActivityLog from './ActivityLog';
import BookingManager from '../workspace/BookingManager';
import CalendarFeeds from '../workspace/CalendarFeeds';
import MyAccount from './MyAccount';
import SquareMerchantPanel from '../workspace/SquareMerchantPanel';
import SearchSettings from './SearchSettings';
import BackupPanel from './BackupPanel';
import { MfaSetup } from '../workspace/AccountSecurity';
import QuoteManager from '../workspace/QuoteManager';
import MarketingPopupManager from '../workspace/MarketingPopupManager';
import BookingCalendar from './BookingCalendar';
import StaffHours from './StaffHours';
import AvailabilityReview from './AvailabilityReview';
import Dealerships from './Dealerships';
import OpeningHours from './OpeningHours';
import ServicesManager from './ServicesManager';
import TaxSettings from './TaxSettings';
import FranchiseManager from './FranchiseManager';

import styles from './owner.module.css';
import Select from '../GlassSelect';

type Access = {
  organizationId: string;
  organizationName: string;
  locationId: string;
  locationName: string;
  actions: string[];
};

type Org = {
  id: string;
  name: string;
  actions: string[];
};

// Every section is its own page with its own address (/owner#customers), so one screen does one job.
type TabId =
  | 'overview' | 'calendar' | 'new-booking' | 'requests' | 'customers' | 'staff-hours' | 'availability' | 'dealership-tags' | 'customer-cars'
  | 'invoices' | 'quotes' | 'dealerships' | 'marketing'
  | 'team' | 'franchises' | 'services' | 'hours' | 'taxes' | 'sop' | 'invoice-template' | 'payments' | 'backups' | 'tracking' | 'enquiries' | 'employees' | 'activity' | 'coupons' | 'reports' | 'scripts' | 'website-images' | 'messaging' | 'sales-banner' | 'dealer-invoices' | 'search'
  | 'account';

type Ctx = { scope: Access | undefined; organizations: Org[]; orgActions: string[] };
interface TabDefinition {
  id: TabId;
  label: string;
  icon: OwnerIconName;
  description: string;
  show: (c: Ctx) => boolean;
}

const has = (c: Ctx, ...actions: string[]) => actions.some((a) => c.scope?.actions.includes(a));
const org = (c: Ctx, ...actions: string[]) => actions.some((a) => c.orgActions.includes(a));

const TABS: TabDefinition[] = [
  { id: 'overview', label: 'Overview', icon: 'overview', description: 'How the business is doing', show: () => true },
  { id: 'calendar', label: 'Calendar', icon: 'calendar', description: 'Who is booked and when', show: (c) => has(c, 'bookings.read') },
  { id: 'new-booking', label: 'New booking', icon: 'calendar', description: 'Book a walk-in or a phone call', show: (c) => ['bookings.create', 'scheduling.reserve', 'customers.read', 'vehicles.read'].every((a) => c.scope?.actions.includes(a)) },
  { id: 'requests', label: 'Appointments', icon: 'calendar', description: 'Every appointment, online or booked by your team', show: (c) => has(c, 'bookings.read') },
  { id: 'customers', label: 'Customers', icon: 'customers', description: 'Your customers', show: (c) => has(c, 'customers.read') },
  { id: 'staff-hours', label: 'Staff hours', icon: 'work', description: 'Where your team is and how long they worked', show: (c) => has(c, 'attendance.read') },
  { id: 'availability', label: 'Staff availability', icon: 'work', description: 'Approve or reject the times staff can work', show: (c) => has(c, 'attendance.manage') || org(c, 'access.manage') },
  { id: 'dealership-tags', label: 'Dealership tags', icon: 'dealership', description: 'Key-tag photos your team posted at dealerships', show: (c) => has(c, 'attendance.read') },
  { id: 'customer-cars', label: 'Customer cars', icon: 'work', description: 'Before and after photos of customers\' cars', show: (c) => has(c, 'attendance.read') },
  { id: 'quotes', label: 'Quotes', icon: 'finance', description: 'Price quotes for customers', show: (c) => has(c, 'quotes.read') },
  { id: 'dealer-invoices', label: 'Dealership invoices', icon: 'finance', description: 'B2B invoices to your dealerships', show: (c) => has(c, 'invoices.read') && has(c, 'dealership.read') },
  { id: 'dealerships', label: 'Dealerships', icon: 'dealership', description: 'Car dealerships you work with', show: (c) => has(c, 'dealership.read') },
  { id: 'messaging', label: 'Email & SMS', icon: 'marketing', description: 'Outgoing email (SMTP) and text messages (Twilio)', show: (c) => org(c, 'access.manage') },
  { id: 'employees', label: 'Employees', icon: 'customers', description: 'The people who work at this franchise', show: (c) => org(c, 'access.manage') || has(c, 'attendance.manage') },
  { id: 'activity', label: 'Activity log', icon: 'admin', description: 'Who signed in and what they changed', show: (c) => org(c, 'access.manage') },
  { id: 'team', label: 'Team', icon: 'customers', description: 'Your team, invitations and roles', show: (c) => org(c, 'access.manage') },
  { id: 'franchises', label: 'Franchises', icon: 'admin', description: 'Your franchises and their details', show: (c) => org(c, 'structure.read') },
  { id: 'services', label: 'Services', icon: 'admin', description: 'Services and prices', show: (c) => has(c, 'catalogue.read') || org(c, 'catalogue.read') },
  { id: 'hours', label: 'Opening hours', icon: 'admin', description: 'When this franchise takes bookings', show: (c) => has(c, 'scheduling.manage') },
  { id: 'taxes', label: 'Taxes', icon: 'finance', description: 'Tax rates for this franchise', show: (c) => !!c.scope && org(c, 'invoices.configure') },
  { id: 'sop', label: 'SOP & Compliance', icon: 'admin', description: 'Standard operating procedures your team must read', show: (c) => has(c, 'sop.read') },
  { id: 'invoice-template', label: 'Invoice template', icon: 'finance', description: 'How printed invoices look', show: (c) => org(c, 'invoices.configure') },
  { id: 'reports', label: 'Reports', icon: 'overview', description: 'Appointments, revenue and customers', show: (c) => has(c, 'bookings.read', 'payments.read', 'customers.read') },
  { id: 'coupons', label: 'Coupons', icon: 'marketing', description: 'Discount codes for your customers', show: (c) => has(c, 'marketing.read') },
  { id: 'enquiries', label: 'Enquiries', icon: 'marketing', description: 'Messages from the website contact form', show: (c) => org(c, 'marketing.read') },
  { id: 'sales-banner', label: 'Sales banner', icon: 'marketing', description: 'A strip above the website header for a sale or offer', show: (c) => has(c, 'content.manage') },
  { id: 'website-images', label: 'Website images', icon: 'marketing', description: 'Portfolio photos shown on the website', show: (c) => has(c, 'content.manage') },
  { id: 'search', label: 'Search & sitemaps', icon: 'marketing', description: 'SEO keywords and descriptions, and the XML sitemaps', show: (c) => org(c, 'access.manage') },
  { id: 'scripts', label: 'Custom scripts', icon: 'marketing', description: 'Extra code for the public website', show: (c) => org(c, 'access.manage') },
  { id: 'backups', label: 'Backups', icon: 'admin', description: 'Download or bring back the whole system', show: (c) => org(c, 'access.manage') && org(c, 'invoices.configure') },
  { id: 'tracking', label: 'Website tracking', icon: 'marketing', description: 'Google Tag, Meta Pixel and the Mailchimp popup', show: (c) => org(c, 'content.manage') },
  { id: 'payments', label: 'Card payments', icon: 'finance', description: 'This franchise’s Square account', show: (c) => org(c, 'invoices.configure') && has(c, 'payments.read') },
  { id: 'account', label: 'My account', icon: 'customers', description: 'Your name, phone and two-factor sign-in', show: () => true },
];

const NAV_GROUPS: { label: string; ids: TabId[] }[] = [
  { label: 'Every day', ids: ['overview', 'calendar', 'new-booking', 'requests', 'customers', 'reports'] },
  { label: 'Team', ids: ['employees', 'staff-hours', 'availability', 'dealership-tags', 'customer-cars', 'sop'] },
  { label: 'Money', ids: ['quotes', 'payments', 'dealerships', 'dealer-invoices', 'messaging', 'coupons', 'sales-banner', 'enquiries'] },
  { label: 'Settings', ids: ['services', 'hours', 'taxes', 'franchises', 'team', 'activity', 'invoice-template', 'website-images', 'search', 'tracking', 'scripts', 'backups'] },
];
const isTab = (v: string): v is TabId => TABS.some((t) => t.id === v);
const ORDER: TabId[] = [...NAV_GROUPS.flatMap((g) => g.ids), 'account'];

export default function OwnerDashboardClient() {
  const [principal, setPrincipal] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [ready, setReady] = useState(false);
  const [locations, setLocations] = useState<Access[]>([]);
  const [organizations, setOrganizations] = useState<Org[]>([]);
  const [selectedLocationId, setSelectedLocationId] = useState('');
  const [activeTab, setActiveTab] = useState<TabId>('overview');
  // The page lives in the address (#customers), so each page can be bookmarked and Back works.
  useEffect(() => {
    const read = () => { const h = window.location.hash.slice(1); setActiveTab(isTab(h) ? h : 'overview'); };
    read();
    window.addEventListener('hashchange', read);
    return () => window.removeEventListener('hashchange', read);
  }, []);
  const [mobileOpen, setMobileOpen] = useState(false);
  const navRef = useRef<HTMLElement>(null);
  const barRef = useRef<HTMLSpanElement>(null);
  const panelRef = useRef<HTMLElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const prevTab = useRef<TabId | null>(null);
  const barPlaced = useRef(false);
  // The franchise's own time zone (from its booking calendar), so times read the same as in the shop.
  const [franchiseZone, setFranchiseZone] = useState('');

  const [customerRevision, setCustomerRevision] = useState(0);
  const [accessRevision, setAccessRevision] = useState(0);
  const [mfaBlocked, setMfaBlocked] = useState(false);
  const [code, setCode] = useState('');
  const [needsCode, setNeedsCode] = useState(false);

  const scope = locations.find((l) => l.locationId === selectedLocationId);
  const base = scope ? `/api/v1/organizations/${scope.organizationId}/locations/${scope.locationId}` : '';

  const refresh = useCallback(async () => {
    const me = await fetch('/api/v1/auth/me');
    if (me.status === 401) {
      setPrincipal('');
      setLocations([]);
      setOrganizations([]);
      return;
    }
    if (!me.ok) throw new Error('Workspace session unavailable');
    const who = await me.json();

    const response = await fetch('/api/v1/auth/access');
    const access = response.ok ? await response.json() : null;
    // This page is for staff, on page load and straight after signing in: customers go to their account, employees to their workspace.
    if (access && !access.locations?.length && !access.organizations?.length) {
      window.location.replace('/account');
      return;
    }
    const home = await fetch('/api/v1/account/home', { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null)).catch(() => null);
    if (home?.workspaceOnly) {
      window.location.replace('/workspace');
      return;
    }
    setPrincipal(who.principalId);
    if (response.status === 403 && (await response.clone().json().catch(() => ({}))).code === 'mfa_setup_required') {
      setMfaBlocked(true);
      return;
    }
    if (!access) throw new Error('Could not load granted locations');
    setMfaBlocked(false);
    setLocations(access.locations || []);
    setOrganizations(access.organizations || []);
    setAccessRevision((v) => v + 1);

    // Keep the franchise chosen before a refresh (remembered in this browser), else the first one.
    setSelectedLocationId((current) => {
      const has = (id: string | null) => !!id && access.locations?.some((l: Access) => l.locationId === id);
      if (has(current)) return current;
      const remembered = typeof window === 'undefined' ? null : window.localStorage.getItem('dapper:owner-location');
      return has(remembered) ? remembered! : access.locations?.[0]?.locationId ?? '';
    });
  }, []);

  useEffect(() => {
    let live = true;
    async function checkHealthAndAuth() {
      try {
        const health = await fetch('/api/v1/health');
        const body = await health.json();
        const available =
          window.location.protocol === 'https:' &&
          body.mode === 'scoped-postgres-foundation' &&
          body.identityProviderConfigured === true;
        if (live) setReady(available);
        if (available) await refresh();
      } catch {
        if (live) setError('The secure workspace is unavailable. Ensure the API and HTTPS gateway are running.');
      }
    }
    checkHealthAndAuth();
    return () => {
      live = false;
    };
  }, [refresh]);

  useEffect(() => {
    setFranchiseZone('');
    if (!base || !scope?.actions.includes('scheduling.read') && !scope?.actions.includes('scheduling.manage')) return;
    let live = true;
    fetch(`${base}/scheduling/resources`, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null)).then((b) => { if (live && b?.items?.[0]?.timezone) setFranchiseZone(b.items[0].timezone); }).catch(() => undefined);
    return () => { live = false; };
  }, [base, scope]);

  useEffect(() => {
    if (!mobileOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMobileOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [mobileOpen]);

  // The glowing bar beside the sidebar glides to the open page; the page slides in from the direction you are moving.
  useLayoutEffect(() => {
    const nav = navRef.current, bar = barRef.current;
    if (!nav || !bar) return;
    const place = (instant: boolean) => {
      const current = nav.querySelector<HTMLElement>('[aria-current="page"]');
      if (!current) { gsap.set(bar, { autoAlpha: 0 }); return; }
      let top = 0, el: HTMLElement | null = current;
      while (el && el !== nav) { top += el.offsetTop; el = el.offsetParent as HTMLElement | null; }
      const vars = { y: top + 8, height: current.offsetHeight - 16, autoAlpha: 1 };
      if (instant || !barPlaced.current || matchMedia('(prefers-reduced-motion: reduce)').matches) { gsap.set(bar, vars); barPlaced.current = true; }
      else gsap.to(bar, { ...vars, duration: 0.55, ease: 'power3.out', overwrite: 'auto' });
    };
    place(false);
    const ro = new ResizeObserver(() => place(true));
    ro.observe(nav);
    return () => ro.disconnect();
  });
  useEffect(() => {
    const from = prevTab.current;
    prevTab.current = activeTab;
    if (from === null || from === activeTab || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const dir = ORDER.indexOf(activeTab) >= ORDER.indexOf(from) ? 1 : -1;
    if (panelRef.current) gsap.fromTo(panelRef.current, { x: dir * 36 }, { x: 0, duration: 0.6, ease: 'power3.out', clearProps: 'transform' });
    if (titleRef.current) gsap.fromTo(titleRef.current, { yPercent: 45, autoAlpha: 0 }, { yPercent: 0, autoAlpha: 1, duration: 0.5, ease: 'power3.out', clearProps: 'transform,opacity,visibility' });
  }, [activeTab]);

  async function signIn(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const response = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(needsCode ? { email, password, code: code.trim() } : { email, password }),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'Sign in failed' }));
        if (body.mfaRequired) {
          setNeedsCode(true);
          setCode('');
        }
        setError(body.error ?? 'Invalid email or password');
        return;
      }
      setPassword('');
      setCode('');
      setNeedsCode(false);
      await refresh();
    } catch {
      setError('Could not reach the workspace. Check your connection before retrying.');
    } finally {
      setBusy(false);
    }
  }

  async function logout() {
    setBusy(true);
    setError('');
    try {
      await fetch('/api/v1/auth/logout', { method: 'POST' });
      setPrincipal('');
      setLocations([]);
      setOrganizations([]);
    } catch {
      setError('Could not sign out. Retry before leaving this device.');
    } finally {
      setBusy(false);
    }
  }

  // Administrators must use an authenticator app; until it is on, this is the only screen they can reach.
  if (principal && mfaBlocked) {
    return (
      <main className="secure-workspace">
        <div className="page-title">
          <div>
            <p className="eyebrow">DAPPER DETAILING PROS</p>
            <h1>Set up two-factor authentication</h1>
            <p className="subtitle">Finish this once and your dashboard opens.</p>
          </div>
        </div>
        <section className="panel detail-card secure-login">
          <MfaSetup
            required
            onDone={() => {
              setMfaBlocked(false);
              void refresh();
            }}
          />
        </section>
      </main>
    );
  }

  // Handle unauthenticated state
  if (!principal) {
    return (
      <AuthShell kind="Business dashboard" title="Sign in to the dashboard" lead="For owners, administrators and managers.">
        {error && <p role="alert" className="field-error">{error}</p>}
          {!ready && (
            <p>
              Secure gateway connection is required. Access via <code>https://127.0.0.1:8443/owner</code>.
            </p>
          )}
          <form onSubmit={signIn}>
            <label className="form-field">
              Email address
              <input
                type="email"
                autoComplete="username"
                disabled={!ready || busy}
                required
                maxLength={254}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </label>
            <label className="form-field">
              Password
              <input
                type="password"
                autoComplete="current-password"
                disabled={!ready || busy}
                required
                maxLength={1024}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </label>
            <p><a href="/reset-password" style={{ textDecoration: 'underline' }}>Forgot your password?</a></p>
            {needsCode && (
              <label className="form-field">
                Authenticator code
                <input
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  autoFocus
                  required
                  maxLength={16}
                  disabled={busy}
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                />
                <small>Enter the 6-digit code from your authenticator app, or a recovery code.</small>
              </label>
            )}
            <button className="primary" disabled={busy || !ready}>
              {busy ? 'Signing in…' : needsCode ? 'Verify and sign in' : 'Sign in to Dashboard'}
            </button>
          </form>
      </AuthShell>
    );
  }

  const orgActions = organizations.find((o) => o.id === scope?.organizationId)?.actions ?? organizations[0]?.actions ?? [];
  const ctx: Ctx = { scope, organizations, orgActions };
  const currentTabDef = TABS.find((t) => t.id === activeTab) || TABS[0];
  const hasTabPermission = currentTabDef.show(ctx);
  const currentOrg = organizations.find((o) => o.id === scope?.organizationId);
  const timezone = franchiseZone || Intl.DateTimeFormat().resolvedOptions().timeZone;

  const goTo = (id: TabId) => {
    if (window.location.hash !== '#' + id) window.location.hash = id;
    setActiveTab(id);
    setMobileOpen(false);
  };

  return (
    <div className={styles.shell}>
      <button
        type="button"
        tabIndex={-1}
        aria-hidden="true"
        className={`${styles.scrim} ${mobileOpen ? styles.scrimOn : ''}`}
        onClick={() => setMobileOpen(false)}
      />
      <aside id="owner-sidebar" className={`${styles.sidebar} ${mobileOpen ? styles.mobileOpen : ''}`} aria-label="Master navigation">
        <Link className={styles.brand} href="/dashboard" aria-label="Dapper Detailing Pros, Master dashboard">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img className={styles.brandLogo} src="/site/assets/logo.webp" alt="" width={44} height={44} />
          <span>
            <strong>Dapper Detailing Pros</strong>
            <em>Master dashboard</em>
          </span>
        </Link>

        <div className={styles.scopeBox}>
          {locations.length > 0 ? (
            <>
              <label htmlFor="owner-location-select" className={styles.scopeLabel}>
                Franchise
              </label>
              <Select
                id="owner-location-select"
                className={styles.scopeSelect}
                value={selectedLocationId}
                onChange={(e) => {
                  setSelectedLocationId(e.target.value);
                  try { window.localStorage.setItem('dapper:owner-location', e.target.value); } catch { /* private mode: just not remembered */ }
                  setError('');
                }}
              >
                {[...new Set(locations.map((l) => l.organizationName))].map((orgName) => (
                  <optgroup key={orgName} label={orgName}>
                    {locations
                      .filter((l) => l.organizationName === orgName)
                      .map((loc) => (
                        <option key={loc.locationId} value={loc.locationId}>
                          {loc.locationName}
                        </option>
                      ))}
                  </optgroup>
                ))}
              </Select>
            </>
          ) : (
            <>
              <p className={styles.scopeLabel}>Franchise</p>
              <p className={styles.scopeNote}>No franchise access has been granted to this account.</p>
            </>
          )}
        </div>

        <nav ref={navRef} className={styles.nav} aria-label="Master sections">
          <span ref={barRef} className={styles.navBar} aria-hidden="true" />
          {NAV_GROUPS.filter((group) => group.ids.some((id) => TABS.find((t) => t.id === id)!.show(ctx))).map((group) => (
            <div key={group.label} className={styles.navGroup}>
              <p className={styles.navGroupLabel}>{group.label}</p>
              {group.ids.filter((id) => TABS.find((t) => t.id === id)!.show(ctx)).map((id) => {
                const tab = TABS.find((t) => t.id === id)!;
                return (
                  <button
                    key={tab.id}
                    type="button"
                    className={styles.navItem}
                    aria-current={activeTab === tab.id ? 'page' : undefined}
                    onClick={() => goTo(tab.id)}
                  >
                    <OwnerIcon name={tab.icon} />
                    <span>{tab.label}</span>
                  </button>
                );
              })}
            </div>
          ))}
        </nav>

        <div className={styles.sidebarFooter}>
          <div className={styles.footerLinks}>
            <button type="button" className={styles.footerLink} aria-current={activeTab === 'account' ? 'page' : undefined} onClick={() => goTo('account')}>
              My account <OwnerIcon name="arrow" size={16} />
            </button>
            <button type="button" className={styles.footerLink} disabled={busy} onClick={logout}>
              Sign out <OwnerIcon name="out" size={16} />
            </button>
          </div>
          <div className={styles.footerRow}>
            <span className={styles.scopeLabel}>Theme</span>
            <ThemeToggle />
          </div>
        </div>
      </aside>

      <div className={styles.main}>
        {/* The brand mark, faint in the corner where pages leave empty space (error404 #35); content sits above it. */}
        <img className={styles.watermark} src="/site/assets/logo.webp" alt="" aria-hidden="true" width={240} height={240} />
        <header className={styles.topBar}>
          <div className={styles.topBarLeft}>
            <button
              type="button"
              className={`${styles.iconBtn} ${styles.menuBtn}`}
              aria-label={mobileOpen ? 'Close navigation menu' : 'Open navigation menu'}
              aria-expanded={mobileOpen}
              aria-controls="owner-sidebar"
              onClick={() => setMobileOpen(!mobileOpen)}
            >
              <OwnerIcon name={mobileOpen ? 'close' : 'menu'} />
            </button>
            <div>
              <h1 ref={titleRef} className={styles.topBarTitle}>{currentTabDef.label}</h1>
              <span className={styles.topBarSub}>{currentTabDef.description}</span>
            </div>
          </div>
          <div className={styles.topBarActions}>
            {scope && (
              <span className={styles.locationBadge} title={`${scope.organizationName} · ${scope.locationName}`}>
                <OwnerIcon name="pin" size={14} /> {scope.locationName}
              </span>
            )}
          </div>
        </header>

        {error && (
          <div className={styles.bannerWrap}>
            <p role="alert" className="field-error">
              {error}
            </p>
          </div>
        )}

        <main ref={panelRef} id={`panel-${activeTab}`} aria-label={currentTabDef.label} tabIndex={-1} className={styles.tabPanel}>
          {!hasTabPermission && scope && (
            <div className={styles.accessDeniedCard}>
              <h3>Access needed for {currentTabDef.label}</h3>
              <p>
                Your account does not have the permissions this section needs at <strong>{scope.locationName}</strong>.
              </p>
              <p>Ask an organization administrator to review your role.</p>
            </div>
          )}

          {activeTab === 'overview' && scope && (
            <OwnerOverview base={base} location={scope.locationName} organizationName={scope.organizationName} onNavigate={(tab) => goTo(isTab(tab) ? tab : tab === 'operations' ? 'calendar' : tab === 'finance' ? 'invoices' : tab === 'bookings' ? 'calendar' : 'overview')} />
          )}
          {hasTabPermission && scope && <>
            {activeTab === 'calendar' && <><BookingCalendar key={base} base={base} timezone={timezone} calendarLink={`${base}/calendar.ics`} canManage={!!scope?.actions.includes('bookings.manage')} /><CalendarFeeds key={'feeds-' + base} endpoint={`${base}/calendar-feeds`} title="Add this franchise’s bookings to Apple Calendar" hint="Make a private link and add it once to Apple Calendar, Google Calendar or Outlook. It then keeps updating by itself. Anyone with the link can see these bookings, so turn it off if it is shared by mistake." /></>}
            {activeTab === 'new-booking' && <StaffBookingWizard mode="master" base={base} timezone={timezone} onBooked={refresh} />}
            {activeTab === 'requests' && <BookingManager key={'booking-mgr-' + base} base={base} canPurge={orgActions.includes('access.manage')} canCreate={scope.actions.includes('bookings.create')} canManage={scope.actions.includes('bookings.manage')} canCreateWorkOrders={false} />}
            {activeTab === 'customers' && <CustomerManager key={base} base={base} canCreate={scope.actions.includes('customers.create')} canDelete={scope.actions.includes('customer_accounts.manage')} canExport={!!currentOrg?.actions.includes('access.manage')} onChanged={() => setCustomerRevision((v) => v + 1)} />}
            {activeTab === 'staff-hours' && <StaffHours key={base} base={base} timezone={timezone} canManage={scope.actions.includes('attendance.manage')} />}
            {activeTab === 'availability' && <AvailabilityReview key={scope.organizationId} organizationId={scope.organizationId} locationId={scope.locationId} />}
            {activeTab === 'dealership-tags' && <EmployeeUploadsReview key={'tags-' + base} base={base} fixedKind="dealership_tag" title="Dealership tags" canSeeLog={scope.actions.includes('attendance.manage')} />}
            {activeTab === 'customer-cars' && <EmployeeUploadsReview key={'cars-' + base} base={base} fixedKind="customer_car" title="Customer cars" canSeeLog={scope.actions.includes('attendance.manage')} />}
            {activeTab === 'sop' && <SOPCompliancePanel base={base} canManage={scope.actions.includes('sop.manage')} />}
            {activeTab === 'invoices' && <InvoiceManager base={base} canCreate={scope.actions.includes('invoices.create')} canManage={scope.actions.includes('invoices.manage')} canRecordPayment={scope.actions.includes('payments.record')} canIssue={scope.actions.includes('invoices.issue')} />}
            {activeTab === 'quotes' && <QuoteManager organizationId={scope.organizationId} locationId={scope.locationId} canCreate={scope.actions.includes('quotes.create')} canManage={scope.actions.includes('quotes.manage')} canIssue={scope.actions.includes('quotes.issue')} />}
            {activeTab === 'backups' && <BackupPanel organizationId={scope.organizationId} />}
            {activeTab === 'search' && <SearchSettings organizationId={scope.organizationId} />}
            {activeTab === 'payments' && (
              <div className={styles.stack}>
                <SquareMerchantPanel organizationId={scope.organizationId} locationId={scope.locationId} canManage />
              </div>
            )}
            {activeTab === 'dealer-invoices' && <DealerInvoices key={'dlinv-' + base} base={base} organizationId={scope.organizationId} canCreate={scope.actions.includes('invoices.create') && scope.actions.includes('invoices.manage')} canIssue={scope.actions.includes('invoices.issue')} canManage={scope.actions.includes('invoices.manage')} canConfigure={orgActions.includes('invoices.configure')} />}
            {activeTab === 'dealerships' && <Dealerships key={base} organizationId={scope.organizationId} locationId={scope.locationId} canManage={scope.actions.includes('dealership.manage')} canConfigure={orgActions.includes('invoices.configure')} />}
            {activeTab === 'marketing' && <MarketingPopupManager key={'marketing-' + base} organizationId={scope.organizationId} locationId={scope.locationId} canManage={scope.actions.includes('marketing.manage')} />}
            {activeTab === 'hours' && <OpeningHours key={base} base={base} franchiseName={scope.locationName} canManage={scope.actions.includes('scheduling.manage')} />}
            {activeTab === 'taxes' && <TaxSettings key={base} base={base} franchiseName={scope.locationName} canManage={orgActions.includes('invoices.configure')} />}
            {activeTab === 'services' && (
              <div className={styles.stack}>
                {/* The central catalogue comes first: every change there applies to all franchises on its own. */}
                {currentOrg?.actions.includes('catalogue.read') && (
                  <ServicesManager key={'central-' + currentOrg.id} mode="central" listUrl={`/api/v1/organizations/${currentOrg.id}/central-services`} organizationId={currentOrg.id} canManage={currentOrg.actions.includes('catalogue.manage')} title="Central catalogue" locationBase={base} intro="Services and prices for every franchise. A change here applies to all franchises automatically, and a new service is switched on everywhere." />
                )}
                {currentOrg?.actions.includes('catalogue.read') && <UpgradeOffers key={'upsells-' + currentOrg.id} organizationId={currentOrg.id} canManage={currentOrg.actions.includes('catalogue.manage')} />}
                {scope.actions.includes('catalogue.read') && (
                  <ServicesManager key={'extras-' + base} mode="franchise" canSlots={scope.actions.includes('scheduling.manage')} extrasOnly={!!currentOrg?.actions.includes('catalogue.read')} listUrl={`${base}/services`} organizationId={scope.organizationId} canManage={scope.actions.includes('catalogue.manage')} title={currentOrg?.actions.includes('catalogue.read') ? `Extra services only at ${scope.locationName}` : `Services at ${scope.locationName}`} intro={currentOrg?.actions.includes('catalogue.read') ? 'Services this franchise offers on top of the central catalogue.' : 'Everything this franchise offers. You can set each service\'s slots here; the services themselves are managed centrally.'} />
                )}
              </div>
            )}
          </>}
          {hasTabPermission && activeTab === 'franchises' && currentOrg && <FranchiseManager organization={currentOrg} selectedId={scope?.locationId} onSaved={refresh} />}
          {hasTabPermission && activeTab === 'team' && currentOrg && <PeopleAccess key={'people-' + currentOrg.id} organizationId={currentOrg.id} revision={accessRevision} onSaved={refresh} />}
          {hasTabPermission && activeTab === 'invoice-template' && currentOrg && <InvoiceIssuerEditor key={'issuer-' + currentOrg.id} organizationId={currentOrg.id} revision={accessRevision} />}
          {hasTabPermission && activeTab === 'invoice-template' && currentOrg && <InvoiceTemplateEditor key={'template-' + currentOrg.id} organizationId={currentOrg.id} revision={accessRevision} locationId={scope?.locationId} locationName={scope?.locationName} />}
          {hasTabPermission && activeTab === 'messaging' && currentOrg && <MessagingSettings key={'messaging-' + currentOrg.id} organizationId={currentOrg.id} revision={accessRevision} />}
          {hasTabPermission && activeTab === 'sales-banner' && scope && <SalesBanner key={'banner-' + base} base={base} />}
          {hasTabPermission && activeTab === 'website-images' && scope && <WebsiteImages key={'images-' + base} base={base} franchiseName={scope.locationName} franchiseCode="" />}
          {hasTabPermission && activeTab === 'scripts' && currentOrg && <CustomScripts key={'scripts-' + currentOrg.id} organizationId={currentOrg.id} revision={accessRevision} />}
          {hasTabPermission && activeTab === 'tracking' && currentOrg && <WebsiteTrackingEditor key={'tracking-' + currentOrg.id} organizationId={currentOrg.id} revision={accessRevision} />}
          {hasTabPermission && activeTab === 'enquiries' && currentOrg && <Enquiries key={'enquiries-' + currentOrg.id} organizationId={currentOrg.id} canManage={currentOrg.actions.includes('marketing.manage')} />}
          {hasTabPermission && activeTab === 'employees' && (currentOrg?.actions.includes('access.manage') ? <Employees key={'employees-' + currentOrg.id} organizationId={currentOrg.id} franchiseName={scope?.locationName} onChanged={refresh} /> : scope && <FranchiseEmployees key={'team-' + base} base={base} franchiseName={scope.locationName} />)}
          {hasTabPermission && activeTab === 'activity' && currentOrg && <ActivityLog key={'activity-' + currentOrg.id} organizationId={currentOrg.id} />}
          {activeTab === 'reports' && scope && <Reports key={'reports-' + base} base={base} />}
          {activeTab === 'coupons' && scope && <Coupons key={'coupons-' + base} base={base} canManage={scope.actions.includes('marketing.manage')} />}
          {activeTab === 'account' && <MyAccount />}
        </main>
      </div>
    </div>
  );
}
