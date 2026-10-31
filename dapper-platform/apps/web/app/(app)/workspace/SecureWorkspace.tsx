'use client';
import {useCallback,useEffect,useRef,useState} from 'react';
import ServiceEditor from './ServiceEditor';
import OwnerOverview from '../owner/OwnerOverview';
import Link from 'next/link';
import CentralServiceEditor from './CentralServiceEditor';
import StructureEditor from './StructureEditor';
import AttendancePanel from './AttendancePanel';
import BookingManager from './BookingManager';
import WorkOrderForm from './WorkOrderForm';
import CustomerForm from './CustomerForm';
import CustomerAccountManager from './CustomerAccountManager';
import ReservedBookingForm from './ReservedBookingForm';
import BookingOfferManager from './BookingOfferManager';
import CustomerBookingRequestList from '../account/CustomerBookingRequestList';
import SOPCompliancePanel from './SOPCompliancePanel';
import ThemeToggle from '../ThemeToggle';
import AccessManager from './AccessManager';
import InvoiceIssuerEditor from './InvoiceIssuerEditor';
import SchedulingEditor from './SchedulingEditor';
import QuoteManager from './QuoteManager';
import DealershipManager from './DealershipManager';
import MarketingPopupManager from './MarketingPopupManager';
import Select from '../GlassSelect';

type Access = {organizationId: string; organizationName: string; locationId: string; locationName: string; actions: string[]};
type Org = {id: string; name: string; actions: string[]};
type Job = {
  id: string;
  serviceName: string;
  description: string;
  status: string;
  completedBy: string | null;
  reservationId?: string | null;
  addons: {serviceId: string; name: string}[];
  contributors: {principalId: string; startedAt: string}[];
};

type LocationOverview = {
  generatedAt: string;
  windowEnd: string;
  metrics: {
    customers: number | null;
    upcomingBookings: number | null;
    openJobs: number | null;
    outstandingCents: number | null;
    netPaymentsCents: number | null;
  };
};

export default function SecureWorkspace({ownerView=false}:{ownerView?:boolean}) {
  const [principal, setPrincipal] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [ready, setReady] = useState(false);
  const [locations, setLocations] = useState<Access[]>([]);
  const [organizations, setOrganizations] = useState<Org[]>([]);
  const [selected, setSelected] = useState('');
  const [jobs, setJobs] = useState<Job[]>([]);
  const [jobsLoading, setJobsLoading] = useState(false);
  const [jobsError, setJobsError] = useState('');
  const [jobFilter, setJobFilter] = useState<'all' | 'available' | 'in_progress' | 'completed'>('all');
  const [jobRevision, setJobRevision] = useState(0);
  const [customerRevision, setCustomerRevision] = useState(0);
  const [accessRevision, setAccessRevision] = useState(0);
  const [overview, setOverview] = useState<LocationOverview | null>(null);
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [overviewError, setOverviewError] = useState('');

  const jobsReqIdRef = useRef(0);
  const overviewReqIdRef = useRef(0);

  const scope = locations.find((l) => l.locationId === selected);
  const base = scope ? `/api/v1/organizations/${scope.organizationId}/locations/${scope.locationId}` : '';

  function classifyJob(job: Job): 'available' | 'in_progress' | 'completed' {
    if (job.status === 'completed') {
      return 'completed';
    }
    if (job.status === 'in_progress' || (job.status === 'available' && (job.contributors?.length ?? 0) > 0)) {
      return 'in_progress';
    }
    return 'available';
  }

  const availableCount = jobs.filter((j) => classifyJob(j) === 'available').length;
  const inProgressCount = jobs.filter((j) => classifyJob(j) === 'in_progress').length;
  const completedCount = jobs.filter((j) => classifyJob(j) === 'completed').length;

  const filteredJobs = jobs.filter((job) => {
    if (jobFilter === 'all') return true;
    return classifyJob(job) === jobFilter;
  });

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
    if (!response.ok) throw new Error('Could not load your granted locations');
    const access = await response.json();
    setLocations(access.locations);
    setOrganizations(access.organizations);
    setAccessRevision((value) => value + 1);
    setSelected((current) =>
      access.locations.some((l: Access) => l.locationId === current)
        ? current
        : access.locations[0]?.locationId ?? ''
    );
  }, []);

  useEffect(() => {
    let live = true;
    async function check() {
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
        if (live) setError('The scoped workspace is unavailable. Start its API and HTTPS gateway.');
      }
    }
    check();
    return () => {
      live = false;
    };
  }, [refresh]);

  const loadJobs = useCallback(async () => {
    const reqId = ++jobsReqIdRef.current;
    if (!base) {
      setJobs([]);
      setJobsLoading(false);
      setJobsError('');
      return;
    }
    setJobsLoading(true);
    setJobsError('');
    try {
      const response = await fetch(`${base}/jobs`);
      if (reqId !== jobsReqIdRef.current) return;
      if (response.status === 403) {
        setJobs([]);
        setJobsError('Access needed (jobs.read)');
        return;
      }
      if (!response.ok) {
        setJobs([]);
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error ?? 'Could not load work orders');
      }
      const body = await response.json();
      if (reqId !== jobsReqIdRef.current) return;
      setJobs(body.items ?? body.jobs ?? []);
      setJobsError('');
    } catch (err) {
      if (reqId !== jobsReqIdRef.current) return;
      setJobs([]);
      setJobsError(err instanceof Error ? err.message : 'Could not load work orders');
    } finally {
      if (reqId === jobsReqIdRef.current) {
        setJobsLoading(false);
      }
    }
  }, [base]);

  useEffect(() => {
    if (scope?.actions.includes('jobs.read')) {
      loadJobs();
    } else {
      jobsReqIdRef.current++;
      setJobs([]);
      setJobsLoading(false);
      setJobsError(scope ? 'Access needed (jobs.read)' : '');
    }
  }, [scope, loadJobs]);

  const loadOverview = useCallback(async () => {
    const reqId = ++overviewReqIdRef.current;
    if (!base) {
      setOverview(null);
      setOverviewLoading(false);
      setOverviewError('');
      return;
    }
    setOverviewLoading(true);
    setOverviewError('');
    try {
      const response = await fetch(`${base}/overview`);
      if (reqId !== overviewReqIdRef.current) return;
      if (response.status === 403) {
        setOverview(null);
        setOverviewError('Location overview requires at least one reporting grant');
        return;
      }
      if (!response.ok) {
        setOverview(null);
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error ?? 'Could not load shift overview');
      }
      const data = await response.json();
      if (reqId !== overviewReqIdRef.current) return;
      setOverview(data);
      setOverviewError('');
    } catch (err) {
      if (reqId !== overviewReqIdRef.current) return;
      setOverview(null);
      setOverviewError(err instanceof Error ? err.message : 'Shift overview unavailable');
    } finally {
      if (reqId === overviewReqIdRef.current) {
        setOverviewLoading(false);
      }
    }
  }, [base]);

  useEffect(() => {
    loadOverview();
  }, [loadOverview, accessRevision, customerRevision, jobRevision]);

  async function signIn(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const response = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({email, password}),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error ?? 'Authentication refused');
      }
      setEmail('');
      setPassword('');
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign in failed');
    } finally {
      setBusy(false);
    }
  }

  async function logout() {
    setBusy(true);
    jobsReqIdRef.current++;
    overviewReqIdRef.current++;
    try {
      await fetch('/api/v1/auth/logout', {method: 'POST'});
      setPrincipal('');
      setLocations([]);
      setOrganizations([]);
      setJobs([]);
      setJobsLoading(false);
      setJobsError('');
      setOverview(null);
      setOverviewLoading(false);
      setOverviewError('');
    } catch {
      setError('Could not sign out. Retry before leaving this device.');
    } finally {
      setBusy(false);
    }
  }

  async function contribute(id: string) {
    setBusy(true);
    setError('');
    try {
      const r = await fetch(base + '/jobs/' + id + '/contribute', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: '{}',
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not join this work');
      }
      await loadJobs();
      setJobRevision((v) => v + 1);
    } catch {
      setError('Could not confirm your contribution. Refresh before retrying.');
    } finally {
      setBusy(false);
    }
  }

  async function complete(id: string) {
    setBusy(true);
    setError('');
    try {
      const r = await fetch(base + '/jobs/' + id + '/complete', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: '{}',
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not complete this job');
      }
      await loadJobs();
      setJobRevision((v) => v + 1);
    } catch {
      setError('Could not reach the workspace. Refresh to check completion before retrying.');
    } finally {
      setBusy(false);
    }
  }

  const hasAdminSections =
    organizations.some((org) =>
      org.actions.some((a) =>
        ['structure.read', 'catalogue.read', 'access.manage', 'invoices.configure'].includes(a)
      )
    ) ||
    Boolean(
      scope?.actions.some((a) => ['customer_accounts.manage', 'marketing.read'].includes(a))
    );

  return (
    <main className="secure-workspace">
      <div className="page-title">
        <div>
          <p className="eyebrow">DAPPER DETAILING PROS</p>
          <h1>{principal ? (ownerView?'Owner dashboard':'Your operations workspace') : (ownerView?'Sign in to Owner dashboard':'Sign in to Dapper')}</h1>
          <p className="subtitle">{ownerView?'An overview of your authorized business locations':'Your account, services and available work'}</p>
        </div>
        <div className="workspace-actions">
          <ThemeToggle />
          {!ownerView && <Link href="/dashboard">Dashboard</Link>}
          {principal && (
            <button className="secondary" disabled={busy} onClick={logout}>
              Sign out
            </button>
          )}
        </div>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {!principal ? (
        <section className="panel detail-card secure-login">
          <h2>Email and password</h2>
          {!ready && (
            <p>
              Secure sign-in is currently unavailable. Open the configured Dapper application address or contact your administrator.
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
            <button className="primary" disabled={busy || !ready}>
              {busy ? 'Signing in…' : 'Sign in'}
            </button>
            <p>Use your Dapper staff account. Contact your administrator if you need access.</p>
          </form>
        </section>
      ) : (
        <>
          <section className="panel detail-card">
            <h2>{ownerView?'Select your location':'Authorized locations'}</h2>
            {locations.length ? (
              <label className="form-field">
                Location
                <Select
                  value={selected}
                  onChange={(e) => {
                    const next = e.target.value;
                    jobsReqIdRef.current++;
                    overviewReqIdRef.current++;
                    setSelected(next);
                    setError('');
                    setJobs([]);
                    setJobsLoading(true);
                    setJobsError('');
                    setOverview(null);
                    setOverviewLoading(true);
                    setOverviewError('');
                  }}
                >
                  {locations.map((l) => (
                    <option key={l.locationId} value={l.locationId}>
                      {l.organizationName} · {l.locationName}
                    </option>
                  ))}
                </Select>
              </label>
            ) : (
              <p>No location actions have been granted to your account. Organization management, if granted, is shown below.</p>
            )}
            {!ownerView && scope && scope.actions.includes('bookings.read') && (
              <div style={{marginTop: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap'}}>
                <span style={{fontSize: '0.85rem', color: 'var(--text-muted)'}}>External Sync:</span>
                <a
                  href={`${base}/calendar.ics`}
                  target="_blank"
                  rel="noreferrer"
                  className="button secondary"
                  style={{display: 'inline-flex', alignItems: 'center', gap: '0.35rem', padding: '0.3rem 0.6rem', fontSize: '0.8rem', textDecoration: 'none'}}
                  title="Subscribe in Apple Calendar, Google Calendar, or Outlook"
                >
                  📅 Apple Calendar (.ics) Feed
                </a>
              </div>
            )}
            {!ownerView && scope && (
              <div style={{marginTop: '1rem', paddingTop: '0.75rem', borderTop: '1px solid var(--border-color, #f0dcde)'}}>
                <div style={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.5rem', flexWrap: 'wrap', gap: '0.5rem'}}>
                  <span style={{fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-secondary, var(--text-muted, #6b5558))'}}>
                    Location Shift Pulse (Module 1)
                  </span>
                  {overview?.generatedAt && (
                    <span style={{fontSize: '0.75rem', color: 'var(--text-muted, var(--text-muted, #6b5558))'}}>
                      Updated {new Date(overview.generatedAt).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'})}
                    </span>
                  )}
                </div>

                {overviewLoading && (
                  <p style={{fontSize: '0.85rem', color: 'var(--text-muted, var(--text-muted, #6b5558))', margin: '0.25rem 0'}}>
                    Loading location shift counts…
                  </p>
                )}

                {overviewError && !overviewLoading && (
                  <p style={{fontSize: '0.85rem', color: 'var(--danger, #a30b18)', margin: '0.25rem 0'}} role="status">
                    Operational counts unavailable ({overviewError})
                  </p>
                )}

                {overview && !overviewLoading && (
                  <div style={{display: 'flex', gap: '1.25rem', flexWrap: 'wrap', fontSize: '0.85rem'}}>
                    <div>
                      <span style={{color: 'var(--text-muted, var(--text-muted, #6b5558))'}}>Active work orders: </span>
                      <strong>
                        {overview.metrics.openJobs !== null
                          ? `${overview.metrics.openJobs} available`
                          : <span style={{color: 'var(--text-muted, var(--text-muted, #6b5558))', fontStyle: 'italic'}}>Access needed (jobs.read)</span>}
                      </strong>
                    </div>
                    <div>
                      <span style={{color: 'var(--text-muted, var(--text-muted, #6b5558))'}}>Upcoming bookings (next 7 days): </span>
                      <strong>
                        {overview.metrics.upcomingBookings !== null
                          ? `${overview.metrics.upcomingBookings} reserved`
                          : <span style={{color: 'var(--text-muted, var(--text-muted, #6b5558))', fontStyle: 'italic'}}>Access needed (bookings.read)</span>}
                      </strong>
                    </div>
                  </div>
                )}
              </div>
            )}
          </section>

          {!ownerView && scope && (
            <nav className="workspace-jump-nav" aria-label="Workspace sections">
              <a href="#operations" className="jump-link">Shop Operations</a>
              <a href="#catalogue" className="jump-link">Services &amp; Scheduling</a>
              <a href="#finance" className="jump-link">Finance &amp; Quotes</a>
              {hasAdminSections && <a href="#administration" className="jump-link">Organization &amp; Setup</a>}
            </nav>
          )}

          {/* 1. Shop Operations */}
          <div id="operations" className="jump-target" />
          {scope?.actions.includes('customers.create') && (
            <CustomerForm
              key={'customer-' + base}
              base={base}
              onSaved={() => setCustomerRevision((value) => value + 1)}
            />
          )}
          {scope?.actions.includes('bookings.read') && (
            <BookingManager
              revision={accessRevision}
              key={'booking-mgr-' + base}
              base={base}
              canCreate={scope.actions.includes('bookings.create')}
              canManage={scope.actions.includes('bookings.manage')}
              canCreateWorkOrders={scope.actions.includes('jobs.create')}
              onJobSpawned={async () => {
                await loadJobs();
                setJobRevision((v) => v + 1);
              }}
            />
          )}
          {!ownerView && scope &&
            ['jobs.create', 'jobs.read', 'customers.read', 'catalogue.read'].every((action) =>
              scope.actions.includes(action)
            ) && (
              <WorkOrderForm
                key={'work-' + base}
                base={base}
                customerRevision={customerRevision}
                onSaved={async () => {
                  await loadJobs();
                  setJobRevision((v) => v + 1);
                }}
              />
            )}
          {scope?.actions.includes('jobs.read') && (
            <section className="panel detail-card">
              <h2>Available work and completion history</h2>
              <p>No fixed employee assignment. Your verified account is recorded when you complete work.</p>

              {jobsLoading && (
                <p style={{color: 'var(--text-muted, var(--text-muted, #6b5558))', margin: '0.75rem 0'}}>
                  Loading work orders…
                </p>
              )}

              {jobsError && !jobsLoading && (
                <p style={{color: 'var(--danger, #a30b18)', margin: '0.75rem 0'}} role="alert">
                  {jobsError}
                </p>
              )}

              {!jobsLoading && !jobsError && (
                <>
                  <div
                    style={{
                      display: 'flex',
                      gap: '0.4rem',
                      flexWrap: 'wrap',
                      margin: '0.75rem 0 1rem',
                      alignItems: 'center',
                    }}
                    role="group"
                    aria-label="Filter loaded shop-floor work orders"
                  >
                    <span style={{fontSize: '0.8rem', color: 'var(--text-muted, var(--text-muted, #6b5558))', marginRight: '0.25rem'}}>
                      Filter loaded ({jobs.length}):
                    </span>
                    <button
                      type="button"
                      className={jobFilter === 'all' ? 'primary' : 'secondary'}
                      style={{padding: '0.3rem 0.65rem', fontSize: '0.8rem', minHeight: 'auto'}}
                      onClick={() => setJobFilter('all')}
                      aria-pressed={jobFilter === 'all'}
                    >
                      All ({jobs.length})
                    </button>
                    <button
                      type="button"
                      className={jobFilter === 'available' ? 'primary' : 'secondary'}
                      style={{padding: '0.3rem 0.65rem', fontSize: '0.8rem', minHeight: 'auto'}}
                      onClick={() => setJobFilter('available')}
                      aria-pressed={jobFilter === 'available'}
                    >
                      Available ({availableCount})
                    </button>
                    <button
                      type="button"
                      className={jobFilter === 'in_progress' ? 'primary' : 'secondary'}
                      style={{padding: '0.3rem 0.65rem', fontSize: '0.8rem', minHeight: 'auto'}}
                      onClick={() => setJobFilter('in_progress')}
                      aria-pressed={jobFilter === 'in_progress'}
                    >
                      In Progress ({inProgressCount})
                    </button>
                    <button
                      type="button"
                      className={jobFilter === 'completed' ? 'primary' : 'secondary'}
                      style={{padding: '0.3rem 0.65rem', fontSize: '0.8rem', minHeight: 'auto'}}
                      onClick={() => setJobFilter('completed')}
                      aria-pressed={jobFilter === 'completed'}
                    >
                      Completed ({completedCount})
                    </button>
                  </div>
                  <div className="management-list">
                    {filteredJobs.map((job) => (
                      <article key={job.id}>
                        <div>
                          <strong>{job.serviceName}</strong>
                          <small>{job.description}</small>
                          {job.reservationId && (
                            <small>Capacity reservation: {job.reservationId.slice(0, 8)}</small>
                          )}
                          {job.addons?.length > 0 && (
                            <small>Add-ons: {job.addons.map((addon) => addon.name).join(', ')}</small>
                          )}
                          <small>
                            Contributors: {job.contributors?.length ?? 0}
                            {job.contributors?.some((person) => person.principalId === principal)
                              ? ' · including you'
                              : ''}
                          </small>
                          <small>
                            {job.status === 'completed'
                              ? job.completedBy === principal
                                ? 'Completed by you'
                                : 'Completed by a team member'
                              : 'Available to this location’s authorized team'}
                          </small>
                        </div>
                        {scope.actions.includes('jobs.contribute') && job.status === 'available' && (
                          <button
                            className="secondary"
                            disabled={busy || job.contributors?.some((person) => person.principalId === principal)}
                            onClick={() => contribute(job.id)}
                          >
                            {job.contributors?.some((person) => person.principalId === principal)
                              ? 'Contributing'
                              : 'Join work'}
                          </button>
                        )}
                        {scope.actions.includes('jobs.complete') && (
                          <button
                            className="primary"
                            disabled={busy || job.status === 'completed'}
                            onClick={() => complete(job.id)}
                          >
                            {job.status === 'completed' ? 'Completed' : 'Mark complete'}
                          </button>
                        )}
                      </article>
                    ))}
                    {!filteredJobs.length && (
                      <p>
                        {jobs.length === 0
                          ? 'No work orders in this location.'
                          : jobFilter === 'available'
                            ? 'No open work orders awaiting contributors in loaded records.'
                            : jobFilter === 'in_progress'
                              ? 'No work orders currently in progress with active contributor evidence.'
                              : jobFilter === 'completed'
                                ? 'No completed work orders in loaded records.'
                                : 'No work orders match the selected filter.'}
                      </p>
                    )}
                  </div>
                </>
              )}
            </section>
          )}
          {!ownerView && scope && ['bookings.create','scheduling.reserve','customers.read','vehicles.read'].every(action=>scope.actions.includes(action)) && (
            <ReservedBookingForm key={'reserved-booking-'+base} base={base} actorId={principal} onSaved={refresh} />
          )}
          {!ownerView && scope && (scope.actions.includes('attendance.record') || scope.actions.includes('attendance.read')) && (
            <AttendancePanel
              key={'att-' + base}
              base={base}
              canRecord={scope.actions.includes('attendance.record')}
            />
          )}
          {!ownerView && scope && scope.actions.includes('sop.read') && (
            <SOPCompliancePanel
              key={'sop-' + base}
              base={base}
              canAcknowledge={scope.actions.includes('sop.acknowledge')}
              canManage={scope.actions.includes('sop.manage')}
            />
          )}
          {ownerView && scope && <OwnerOverview key={base} base={base} location={scope.locationName} />}

          {/* 2. Services & Scheduling */}
          <div id="catalogue" className="jump-target" />
          {scope?.actions.includes('catalogue.read') && (
            <ServiceEditor key={base} base={base} canManage={scope.actions.includes('catalogue.manage')} />
          )}
          {scope?.actions.includes('bookings.read') && (
            <CustomerBookingRequestList key={'customer-requests-'+base} endpoint={`${base}/customer-booking-requests`} staff />
          )}
          {scope?.actions.includes('scheduling.manage') && scope.actions.includes('catalogue.read') && (
            <SchedulingEditor key={'schedule-' + base} base={base} />
          )}
          {scope && ['catalogue.manage','scheduling.manage'].every(action=>scope.actions.includes(action)) && (
            <BookingOfferManager key={'offers-'+base} base={base} />
          )}

          {/* 3. Finance & Quotes */}
          <div id="finance" className="jump-target" />
          {/* Invoices are opened from each appointment (Bookings & Appointments); the separate list lives on the owner dashboard. */}
          {!ownerView && scope && scope.actions.includes('quotes.read') && (
            <QuoteManager
              key={'quotes-' + base}
              organizationId={scope.organizationId}
              locationId={scope.locationId}
              canCreate={scope.actions.includes('quotes.create')}
              canManage={scope.actions.includes('quotes.manage')}
              canIssue={scope.actions.includes('quotes.issue')}
            />
          )}
          {!ownerView && scope && scope.actions.includes('dealership.read') && (
            <DealershipManager
              key={'dealership-' + base}
              organizationId={scope.organizationId}
              locationId={scope.locationId}
              canManage={scope.actions.includes('dealership.manage')}
            />
          )}

          {/* 4. Organization Administration & Setup (Progressive Disclosure) */}
          <div id="administration" className="jump-target" />
          {hasAdminSections && (
            <div style={{marginTop: '2rem', marginBottom: '1rem'}}>
              <h2 style={{fontSize: '16px', fontWeight: 800, margin: '0 0 4px'}}>Organization Administration &amp; Setup</h2>
              <p style={{fontSize: '11px', color: 'var(--text-secondary)', margin: 0}}>
                Manage organizational structure, staff role assignments, central catalog packages, and invoice configurations.
              </p>
            </div>
          )}
          {organizations
            .filter((org) => org.actions.includes('structure.read'))
            .map((org) => (
              <details key={'struct-' + org.id} className="panel detail-card setup-disclosure">
                <summary>
                  <div className="disclosure-header">
                    <h2>Franchises &amp; Locations — {org.name}</h2>
                    <p>Configure operating franchises and physical shop-floor locations</p>
                  </div>
                  <span className="disclosure-toggle">Manage structure ▾</span>
                </summary>
                <div className="disclosure-body">
                  <StructureEditor organization={org} onSaved={refresh} />
                </div>
              </details>
            ))}
          {organizations
            .filter((org) => org.actions.includes('catalogue.read'))
            .map((org) => (
              <details key={'central-cat-' + org.id} className="panel detail-card setup-disclosure">
                <summary>
                  <div className="disclosure-header">
                    <h2>Central Catalog Packages — {org.name}</h2>
                    <p>Manage enterprise service definitions and franchise catalog links</p>
                  </div>
                  <span className="disclosure-toggle">Manage catalog ▾</span>
                </summary>
                <div className="disclosure-body">
                  <CentralServiceEditor organization={org} canManage={org.actions.includes('catalogue.manage')} />
                </div>
              </details>
            ))}
          {organizations.filter((org) => org.actions.includes('access.manage')).map((org) => (
            <details key={'access-' + org.id} className="panel detail-card setup-disclosure">
              <summary>
                <div className="disclosure-header">
                  <h2>Access Authority &amp; Role Assignments — {org.name}</h2>
                  <p>Assign named roles, configure scoped grants, and invite team members</p>
                </div>
                <span className="disclosure-toggle">Manage access ▾</span>
              </summary>
              <div className="disclosure-body">
                <AccessManager organizationId={org.id} revision={accessRevision} onSaved={refresh} />
              </div>
            </details>
          ))}
          {organizations.filter((org) => org.actions.includes('invoices.configure')).map((org) => (
            <details key={'issuer-' + org.id} className="panel detail-card setup-disclosure">
              <summary>
                <div className="disclosure-header">
                  <h2>Invoice Issuer Configuration — {org.name}</h2>
                  <p>Set approved legal invoicing entity, tax numbering, and payment instructions</p>
                </div>
                <span className="disclosure-toggle">Configure issuer ▾</span>
              </summary>
              <div className="disclosure-body">
                <InvoiceIssuerEditor organizationId={org.id} revision={accessRevision} />
              </div>
            </details>
          ))}
          {!ownerView && scope && scope.actions.includes('customer_accounts.manage') && organizations.some(org=>org.id===scope.organizationId&&org.actions.includes('access.manage')) && (
            <details className="panel detail-card setup-disclosure">
              <summary>
                <div className="disclosure-header">
                  <h2>Customer Portal Accounts</h2>
                  <p>Manage verified customer web credentials and access</p>
                </div>
                <span className="disclosure-toggle">Manage accounts ▾</span>
              </summary>
              <div className="disclosure-body">
                <CustomerAccountManager key={'customer-accounts-'+base} base={base} revision={customerRevision+accessRevision} />
              </div>
            </details>
          )}
          {!ownerView && scope && scope.actions.includes('marketing.read') && (
            <details className="panel detail-card setup-disclosure">
              <summary>
                <div className="disclosure-header">
                  <h2>Marketing Popups &amp; Consents</h2>
                  <p>Configure public promotional banners, Mailchimp synchronization, and consent logs</p>
                </div>
                <span className="disclosure-toggle">Manage marketing ▾</span>
              </summary>
              <div className="disclosure-body">
                <MarketingPopupManager
                  key={'marketing-' + base}
                  organizationId={scope.organizationId}
                  locationId={scope.locationId}
                  canManage={scope.actions.includes('marketing.manage')}
                />
              </div>
            </details>
          )}
        </>
      )}
      <footer>Dapper Detailing Pros · Secure operations workspace</footer>
    </main>
  );
}
