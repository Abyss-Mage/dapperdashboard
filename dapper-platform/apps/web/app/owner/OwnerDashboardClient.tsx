'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import ThemeToggle from '../ThemeToggle';
import { OwnerIcon, type OwnerIconName } from './OwnerIcons';

// Operational & Management Component Imports
import OwnerOverview from './OwnerOverview';
import PeopleAccess from './PeopleAccess';
import LocationsSetup from './LocationsSetup';
import SectionDashboard from './SectionDashboard';
import AttendancePanel from '../workspace/AttendancePanel';
import GpsPanel from '../workspace/GpsPanel';
import EmployeeUploadsReview from '../workspace/EmployeeUploadsReview';
import BookingManager from '../workspace/BookingManager';
import SubPages from './SubPages';
import CustomerManager from './CustomerManager';
import StaffBookingWizard from '../workspace/StaffBookingWizard';
import BookingOfferManager from '../workspace/BookingOfferManager';
import CustomerBookingRequestList from '../account/CustomerBookingRequestList';
import InvoiceManager from '../workspace/InvoiceManager';
import SOPCompliancePanel from '../workspace/SOPCompliancePanel';
import InvoiceIssuerEditor from '../workspace/InvoiceIssuerEditor';
import InvoiceTemplateEditor from '../workspace/InvoiceTemplateEditor';
import MyAccount from './MyAccount';
import SquareMerchantPanel from '../workspace/SquareMerchantPanel';
import { MfaSetup } from '../workspace/AccountSecurity';
import SchedulingEditor from '../workspace/SchedulingEditor';
import QuoteManager from '../workspace/QuoteManager';
import DealershipManager from '../workspace/DealershipManager';
import MarketingPopupManager from '../workspace/MarketingPopupManager';

import styles from './owner.module.css';

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

type Job = {
  id: string;
  serviceName: string;
  description: string;
  status: string;
  completedBy: string | null;
  reservationId?: string | null;
  addons: { serviceId: string; name: string }[];
  contributors: { principalId: string; startedAt: string }[];
};

type TabId =
  | 'overview'
  | 'bookings'
  | 'customers'
  | 'operations'
  | 'finance'
  | 'dealership'
  | 'marketing'
  | 'people'
  | 'setup'
  | 'account';

interface TabDefinition {
  id: TabId;
  label: string;
  icon: OwnerIconName;
  requiredGrants: string[];
  description: string;
}

const TABS: TabDefinition[] = [
  { id: 'overview', label: 'Overview', icon: 'overview', requiredGrants: [], description: 'How the business is doing' },
  { id: 'bookings', label: 'Bookings', icon: 'calendar', requiredGrants: ['bookings.read', 'bookings.create', 'scheduling.manage'], description: 'Who is booked and when' },
  { id: 'customers', label: 'Customers', icon: 'customers', requiredGrants: ['customers.read', 'customers.create', 'customer_accounts.manage'], description: 'Your customers' },
  { id: 'operations', label: 'Work & Staff', icon: 'work', requiredGrants: ['jobs.read', 'jobs.create', 'attendance.read', 'sop.read'], description: 'Jobs and your team' },
  { id: 'finance', label: 'Invoices & Payments', icon: 'finance', requiredGrants: ['invoices.read', 'invoices.create', 'quotes.read', 'invoices.configure'], description: 'Invoices, quotes and card payments' },
  { id: 'dealership', label: 'Partner Dealerships', icon: 'dealership', requiredGrants: ['dealership.read', 'dealership.manage'], description: 'Dealerships you work with' },
  { id: 'marketing', label: 'Marketing', icon: 'marketing', requiredGrants: ['marketing.read', 'marketing.manage'], description: 'Pop-up sign-up forms and email list' },
  { id: 'people', label: 'People', icon: 'customers', requiredGrants: ['access.manage'], description: 'Your team, invitations and what each person can do' },
  { id: 'setup', label: 'Setup', icon: 'admin', requiredGrants: ['structure.read', 'catalogue.manage', 'scheduling.manage'], description: 'Locations, services, prices and opening hours' },
  { id: 'account', label: 'My account', icon: 'customers', requiredGrants: [], description: 'Your name, phone and two-factor sign-in' },
];

const NAV_GROUPS: { label: string; ids: TabId[] }[] = [
  { label: 'Operate', ids: ['overview', 'bookings', 'customers', 'operations'] },
  { label: 'Business', ids: ['finance', 'dealership', 'marketing'] },
  { label: 'Administer', ids: ['people', 'setup'] },
];

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
  const [mobileOpen, setMobileOpen] = useState(false);

  const [jobs, setJobs] = useState<Job[]>([]);
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
    setPrincipal(who.principalId);

    const response = await fetch('/api/v1/auth/access');
    if (response.status === 403 && (await response.clone().json().catch(() => ({}))).code === 'mfa_setup_required') {
      setMfaBlocked(true);
      return;
    }
    if (!response.ok) throw new Error('Could not load granted locations');
    setMfaBlocked(false);
    const access = await response.json();
    setLocations(access.locations || []);
    setOrganizations(access.organizations || []);
    setAccessRevision((v) => v + 1);

    setSelectedLocationId((current) =>
      access.locations?.some((l: Access) => l.locationId === current)
        ? current
        : access.locations?.[0]?.locationId ?? ''
    );
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

  const loadJobs = useCallback(async () => {
    if (!base) {
      setJobs([]);
      return;
    }
    try {
      const r = await fetch(base + '/jobs');
      if (!r.ok) return;
      const body = await r.json();
      setJobs(body.jobs ?? []);
    } catch {
      // Non-fatal background fetch
    }
  }, [base]);

  useEffect(() => {
    if (scope?.actions.includes('jobs.read')) {
      loadJobs();
    } else {
      setJobs([]);
    }
  }, [scope, loadJobs]);

  useEffect(() => {
    if (!mobileOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMobileOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [mobileOpen]);

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
      setJobs([]);
    } catch {
      setError('Could not sign out. Retry before leaving this device.');
    } finally {
      setBusy(false);
    }
  }

  async function contribute(id: string) {
    if (!base) return;
    setBusy(true);
    setError('');
    try {
      const r = await fetch(base + '/jobs/' + id + '/contribute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not join this work');
      }
      await loadJobs();
    } catch {
      setError('Could not confirm contribution. Refresh before retrying.');
    } finally {
      setBusy(false);
    }
  }

  async function complete(id: string) {
    if (!base) return;
    setBusy(true);
    setError('');
    try {
      const r = await fetch(base + '/jobs/' + id + '/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not complete this job');
      }
      await loadJobs();
    } catch {
      setError('Could not reach workspace. Refresh to check completion.');
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
      <main className="secure-workspace">
        <div className="page-title">
          <div>
            <p className="eyebrow">DAPPER DETAILING PROS</p>
            <h1>Owner & Business Dashboard</h1>
            <p className="subtitle">Sign in with authorized administrator or manager credentials</p>
          </div>
          <div className="workspace-actions">
            <ThemeToggle />
          </div>
        </div>
        {error && <p role="alert" className="field-error">{error}</p>}
        <section className="panel detail-card secure-login">
          <h2>Manager Sign-in</h2>
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
        </section>
      </main>
    );
  }

  const currentTabDef = TABS.find((t) => t.id === activeTab) || TABS[0];
  const canConfigureInvoices = !!scope && organizations.some((org) => org.id === scope.organizationId && org.actions.includes('invoices.configure'));
  const hasTabPermission =
    activeTab === 'overview' ||
    currentTabDef.requiredGrants.length === 0 ||
    currentTabDef.requiredGrants.some((g) => scope?.actions.includes(g)) ||
    ((activeTab === 'setup' || activeTab === 'people') && organizations.some((o) => o.actions.some((a) => currentTabDef.requiredGrants.includes(a))));

  const goTo = (id: TabId) => {
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
        <Link className={styles.brand} href="/owner" aria-label="Dapper Detailing Pros, Master dashboard">
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
              <select
                id="owner-location-select"
                className={styles.scopeSelect}
                value={selectedLocationId}
                onChange={(e) => {
                  setSelectedLocationId(e.target.value);
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
              </select>
            </>
          ) : (
            <>
              <p className={styles.scopeLabel}>Franchise</p>
              <p className={styles.scopeNote}>No franchise access has been granted to this account.</p>
            </>
          )}
        </div>

        <nav className={styles.nav} aria-label="Master sections">
          {NAV_GROUPS.map((group) => (
            <div key={group.label} className={styles.navGroup}>
              <p className={styles.navGroupLabel}>{group.label}</p>
              {group.ids.map((id) => {
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
              <h1 className={styles.topBarTitle}>{currentTabDef.label}</h1>
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

        <main id={`panel-${activeTab}`} aria-label={currentTabDef.label} tabIndex={-1} className={styles.tabPanel}>
          {!hasTabPermission && scope && (
            <div className={styles.accessDeniedCard}>
              <h3>Access needed for {currentTabDef.label}</h3>
              <p>
                Your account does not have the permissions this section needs at <strong>{scope.locationName}</strong>.
              </p>
              <p>
                Needs one of: {currentTabDef.requiredGrants.map((g) => (
                  <code key={g}>{g} </code>
                ))}
              </p>
              <p>Ask an organization administrator to review your role.</p>
            </div>
          )}

          {/* TAB 1: EXECUTIVE OVERVIEW */}
          {activeTab === 'overview' && scope && (
            <OwnerOverview
              base={base}
              location={scope.locationName}
              organizationName={scope.organizationName}
              onNavigate={(tab) => setActiveTab(tab as TabId)}
            />
          )}

          {/* BOOKINGS */}
          {activeTab === 'bookings' && scope && (
            <SubPages
              key={'bookings-' + base}
              label="Booking pages"
              pages={[
                {
                  id: 'list', label: 'All bookings', show: scope.actions.includes('bookings.read'),
                  node: () => (
                    <BookingManager revision={accessRevision} base={base} canCreate={scope.actions.includes('bookings.create')} canManage={scope.actions.includes('bookings.manage')} canCreateWorkOrders={scope.actions.includes('jobs.create')} onJobSpawned={loadJobs} />
                  ),
                },
                {
                  id: 'new', label: 'New booking', show: ['bookings.create', 'scheduling.reserve', 'customers.read', 'vehicles.read'].every((a) => scope.actions.includes(a)),
                  node: () => <StaffBookingWizard mode="master" base={base} timezone={Intl.DateTimeFormat().resolvedOptions().timeZone} onBooked={refresh} />,
                },
                {
                  id: 'requests', label: 'Online requests', show: scope.actions.includes('bookings.read'),
                  node: () => <CustomerBookingRequestList endpoint={`${base}/customer-booking-requests`} staff />,
                },
                {
                  id: 'times', label: 'Opening times', show: scope.actions.includes('scheduling.manage') && scope.actions.includes('catalogue.read'),
                  node: () => <SchedulingEditor base={base} />,
                },
                {
                  id: 'packages', label: 'Packages', show: ['catalogue.manage', 'scheduling.manage'].every((a) => scope.actions.includes(a)),
                  node: () => <BookingOfferManager base={base} />,
                },
                {
                  id: 'calendar', label: 'Phone calendar', show: scope.actions.includes('bookings.read'),
                  node: () => (
                    <section className="panel detail-card">
                      <h2>See bookings on your phone calendar</h2>
                      <p>Add this link to Apple Calendar or Google Calendar. New bookings show up on their own.</p>
                      <a href={`${base}/calendar.ics`} target="_blank" rel="noreferrer" className="button secondary">Open the calendar link</a>
                    </section>
                  ),
                },
              ]}
            />
          )}

          {/* CUSTOMERS */}
          {activeTab === 'customers' && scope && (
            <SubPages
              key={'customers-' + base}
              label="Customer pages"
              pages={[
                {
                  id: 'list', label: 'Customer list', show: scope.actions.includes('customers.read'),
                  node: () => (
                    <CustomerManager base={base} canCreate={scope.actions.includes('customers.create')} canDelete={scope.actions.includes('customer_accounts.manage')} onChanged={() => setCustomerRevision((v) => v + 1)} />
                  ),
                },
                {
                  id: 'numbers', label: 'Numbers', show: scope.actions.includes('customers.read'),
                  node: () => <SectionDashboard section="customers" base={base} />,
                },
              ]}
            />
          )}

          {/* WORK AND STAFF */}
          {activeTab === 'operations' && scope && (
            <SubPages
              key={'work-' + base}
              label="Work and staff pages"
              pages={[
                {
                  id: 'jobs', label: 'Jobs', show: scope.actions.includes('jobs.read'),
                  node: () => (
                    <section className="panel detail-card">
                      <h2>Jobs</h2>
                      <p>Work the team is doing or has finished.</p>
                      <div className="management-list">
                        {jobs.map((job) => (
                          <article key={job.id}>
                            <div>
                              <strong>{job.serviceName}</strong>
                              <small>{job.description}</small>
                              {job.addons?.length > 0 && <small>Extras: {job.addons.map((a) => a.name).join(', ')}</small>}
                              <small>{job.status === 'completed' ? 'Finished' : 'Open for the team'}{job.contributors?.length ? ` · ${job.contributors.length} working on it` : ''}</small>
                            </div>
                            {scope.actions.includes('jobs.contribute') && job.status === 'available' && (
                              <button className="secondary" disabled={busy || job.contributors?.some((p) => p.principalId === principal)} onClick={() => contribute(job.id)}>
                                {job.contributors?.some((p) => p.principalId === principal) ? 'You are on it' : 'Join this job'}
                              </button>
                            )}
                            {scope.actions.includes('jobs.complete') && (
                              <button className="primary" disabled={busy || job.status === 'completed'} onClick={() => complete(job.id)}>
                                {job.status === 'completed' ? 'Finished' : 'Mark finished'}
                              </button>
                            )}
                          </article>
                        ))}
                        {!jobs.length && <p>No jobs yet.</p>}
                      </div>
                    </section>
                  ),
                },
                {
                  id: 'attendance', label: 'Staff hours', show: scope.actions.includes('attendance.record') || scope.actions.includes('attendance.read'),
                  node: () => <AttendancePanel base={base} canRecord={scope.actions.includes('attendance.record')} />,
                },
                {
                  id: 'where', label: 'Where staff are', show: scope.actions.includes('attendance.read'),
                  node: () => <GpsPanel base={base} canManage={scope.actions.includes('attendance.manage')} />,
                },
                {
                  id: 'photos', label: 'Staff photos', show: scope.actions.includes('attendance.read'),
                  node: () => <EmployeeUploadsReview base={base} canSeeLog={scope.actions.includes('attendance.manage')} />,
                },
                {
                  id: 'procedures', label: 'Work procedures', show: scope.actions.includes('sop.read'),
                  node: () => <SOPCompliancePanel base={base} canAcknowledge={scope.actions.includes('sop.acknowledge')} />,
                },
                {
                  id: 'numbers', label: 'Numbers', show: scope.actions.includes('jobs.read'),
                  node: () => <SectionDashboard section="work" base={base} />,
                },
              ]}
            />
          )}

          {/* INVOICES AND PAYMENTS */}
          {activeTab === 'finance' && scope && (
            <SubPages
              key={'finance-' + base}
              label="Invoice pages"
              pages={[
                {
                  id: 'invoices', label: 'Invoices', show: scope.actions.includes('invoices.read'),
                  node: () => (
                    <InvoiceManager base={base} canCreate={scope.actions.includes('invoices.create')} canManage={scope.actions.includes('invoices.manage')} canRecordPayment={scope.actions.includes('payments.record')} canIssue={scope.actions.includes('invoices.issue')} />
                  ),
                },
                {
                  id: 'quotes', label: 'Quotes', show: scope.actions.includes('quotes.read'),
                  node: () => (
                    <QuoteManager organizationId={scope.organizationId} locationId={scope.locationId} canCreate={scope.actions.includes('quotes.create')} canManage={scope.actions.includes('quotes.manage')} canIssue={scope.actions.includes('quotes.issue')} />
                  ),
                },
                {
                  id: 'numbers', label: 'Numbers', show: scope.actions.includes('invoices.read') || scope.actions.includes('payments.read'),
                  node: () => <SectionDashboard section="finance" base={base} />,
                },
                {
                  id: 'business', label: 'Business details', show: canConfigureInvoices,
                  node: () => (
                    <>
                      {organizations.filter((org) => org.id === scope.organizationId).map((org) => (
                        <InvoiceIssuerEditor key={'issuer-' + org.id} organizationId={org.id} revision={accessRevision} />
                      ))}
                    </>
                  ),
                },
                {
                  id: 'template', label: 'Printed invoice', show: canConfigureInvoices,
                  node: () => (
                    <>
                      {organizations.filter((org) => org.id === scope.organizationId).map((org) => (
                        <InvoiceTemplateEditor key={'template-' + org.id} organizationId={org.id} revision={accessRevision} />
                      ))}
                    </>
                  ),
                },
                {
                  id: 'cards', label: 'Card payments', show: canConfigureInvoices && scope.actions.includes('payments.read'),
                  node: () => <SquareMerchantPanel organizationId={scope.organizationId} locationId={scope.locationId} canManage />,
                },
              ]}
            />
          )}

          {/* TAB 6: B2B DEALERSHIP FLEET */}
          {activeTab === 'dealership' && scope && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
              {scope.actions.includes('dealership.read') && (
                <DealershipManager
                  key={'dealership-' + base}
                  organizationId={scope.organizationId}
                  locationId={scope.locationId}
                  canManage={scope.actions.includes('dealership.manage')}
                />
              )}
            </div>
          )}

          {/* TAB 7: MARKETING & POPUPS */}
          {activeTab === 'marketing' && scope && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
              {scope.actions.includes('marketing.read') && (
                <MarketingPopupManager
                  key={'marketing-' + base}
                  organizationId={scope.organizationId}
                  locationId={scope.locationId}
                  canManage={scope.actions.includes('marketing.manage')}
                />
              )}
            </div>
          )}

          {/* PEOPLE & ACCESS */}
          {activeTab === 'people' &&
            organizations
              .filter((org) => org.actions.includes('access.manage') && (!scope || org.id === scope.organizationId))
              .map((org) => (
                <PeopleAccess key={'people-' + org.id} organizationId={org.id} revision={accessRevision} onSaved={refresh} />
              ))}

          {/* MY ACCOUNT */}
          {activeTab === 'account' && <MyAccount />}

          {/* LOCATIONS & SETUP */}
          {activeTab === 'setup' &&
            organizations
              .filter((org) => scope ? org.id === scope.organizationId : true)
              .map((org) => (
                <LocationsSetup key={'setup-' + org.id} organization={org} scope={scope} base={base} onSaved={refresh} />
              ))}
        </main>
      </div>
    </div>
  );
}
