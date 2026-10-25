'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import styles from './owner.module.css';
import { OwnerIcon, type OwnerIconName } from './OwnerIcons';
import RangeBar, { resolveRange, type Custom, type RangePresetId } from './DateRange';
import { BarList, ColumnChart, DataTable, LineChart, Sparkline, compact, dollars, dollarsCompact, shortDate, wholeNumber } from './charts';
import Select from '../GlassSelect';

type Overview = { generatedAt: string; windowEnd: string; metrics: Record<string, number | null> };
type Key = { key: string; count: number; cents: number; dueCents: number };
type Trends = {
  generatedAt: string;
  from: string;
  to: string;
  bookings: { byDay: { date: string; count: number }[]; byStatus: Key[]; byChannel: Key[]; topServices: Key[]; total: number } | null;
  payments: { byDay: { date: string; cents: number }[]; byMethod: Key[]; netCents: number } | null;
  invoices: { byStatus: Key[] } | null;
  jobs: { createdByDay: { date: string; count: number }[]; completedByDay: { date: string; count: number }[]; created: number; completed: number; open: number } | null;
};

interface OwnerOverviewProps {
  base: string;
  location: string;
  organizationName?: string;
  onNavigate?: (tab: string) => void;
}

const PRESETS: RangePresetId[] = ['last7', 'last30', 'last90', 'next7', 'next30', 'month'];

const CHANNELS = [
  { id: '', label: 'All bookings' },
  { id: 'online', label: 'Online' },
  { id: 'walk_in', label: 'Walk-in' },
];
const nice = (s: string) => s.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase());

function Card({ title, sub, legend, span = 1, children }: { title: string; sub?: string; legend?: ReactNode; span?: 1 | 2; children: ReactNode }) {
  return (
    <section className={`${styles.chartCard} ${span === 2 ? styles.span2 : ''}`}>
      <div className={styles.chartHead}>
        <div>
          <h3>{title}</h3>
          {sub && <p>{sub}</p>}
        </div>
        {legend}
      </div>
      {children}
    </section>
  );
}
function Locked({ title, grant }: { title: string; grant: string }) {
  return (
    <section className={`${styles.chartCard} ${styles.locked}`}>
      <h3>{title}</h3>
      <p className={styles.chartEmpty}>Access needed. Your role does not include <code>{grant}</code> for this franchise, so nothing is shown rather than a misleading zero.</p>
    </section>
  );
}

export default function OwnerOverview({ base, location, organizationName, onNavigate }: OwnerOverviewProps) {
  const [preset, setPreset] = useState<RangePresetId>('last30');
  const [custom, setCustom] = useState<Custom>(() => resolveRange('last30', { from: '', to: '' }));
  const [channel, setChannel] = useState('');
  const [overview, setOverview] = useState<Overview | null>(null);
  const [trends, setTrends] = useState<Trends | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshKey, setRefreshKey] = useState(0);
  const errorRef = useRef<HTMLParagraphElement>(null);
  const range = useMemo(() => resolveRange(preset, custom), [preset, custom]);
  const tz = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', []);

  useEffect(() => {
    let live = true;
    setLoading(true);
    setError('');
    const q = new URLSearchParams({ from: range.from, to: range.to, tz, channel });
    Promise.all([
      fetch(`${base}/overview`, { cache: 'no-store' }),
      fetch(`${base}/overview/trends?${q}`, { cache: 'no-store' }),
    ])
      .then(async ([o, t]) => {
        if (o.status === 403 && t.status === 403) throw new Error('Overview access has not been granted for this franchise. Ask an administrator to review your read permissions.');
        if (!t.ok && !o.ok) throw new Error('The overview could not be loaded. Please retry.');
        const [ov, tr] = await Promise.all([o.ok ? o.json() : null, t.ok ? t.json() : null]);
        if (live) {
          setOverview(ov);
          setTrends(tr);
        }
      })
      .catch((e) => live && setError(e.message || 'The overview could not be loaded.'))
      .finally(() => live && setLoading(false));
    return () => {
      live = false;
    };
  }, [base, range, tz, channel, refreshKey]);

  useEffect(() => {
    if (error && !loading) errorRef.current?.focus();
  }, [error, loading]);

  const m = overview?.metrics ?? {};
  const days = trends?.bookings?.byDay ?? trends?.jobs?.createdByDay ?? [];
  const labels = days.map((d) => d.date);
  const bookingPts = (trends?.bookings?.byDay ?? []).map((d) => ({ label: d.date, value: d.count }));
  const payPts = trends?.payments?.byDay ?? [];
  const money = (k: string) => (m[k] === null || m[k] === undefined ? null : (m[k] as number));

  const tile = (label: string, value: string | null, foot: string, tab: string, icon: OwnerIconName, spark?: number[], grant?: string) => (
    <button type="button" className={styles.tile} onClick={() => onNavigate?.(tab)} aria-label={`${label}: ${value ?? 'Access needed'}. Open details.`}>
      <span className={styles.cardTop}>
        <span className={styles.tileLabel}>{label}</span>
        <OwnerIcon name={icon} size={18} />
      </span>
      <span className={`${styles.tileValue} ${value === null ? styles.valueMuted : ''}`}>{value ?? 'Access needed'}</span>
      <span className={styles.tileFoot}>
        <span>{value === null && grant ? `Needs ${grant}` : foot}</span>
        {value !== null && spark && <Sparkline values={spark} />}
      </span>
    </button>
  );

  const rangeText = `${shortDate(range.from)} – ${shortDate(range.to)}`;
  const status = trends?.bookings?.byStatus ?? [];

  return (
    <section className={styles.overview} aria-label="Business overview">
      <div className={styles.overviewHeading}>
        <div>
          <p className={styles.eyebrow}>{organizationName ? `${organizationName} · ` : ''}Franchise overview</p>
          <h2>{location}</h2>
        </div>
      </div>

      <div className={styles.filterBar} role="group" aria-label="Overview filters">
        <RangeBar presets={PRESETS} preset={preset} custom={custom} onPreset={setPreset} onCustom={setCustom} />
        <label className={styles.filterSelect}>
          <span>Booked</span>
          <Select value={channel} onChange={(e) => setChannel(e.target.value)}>
            {CHANNELS.map((c) => (
              <option key={c.id} value={c.id}>{c.label}</option>
            ))}
          </Select>
        </label>
        <button type="button" className="secondary" disabled={loading} onClick={() => setRefreshKey((k) => k + 1)}>
          <OwnerIcon name="refresh" size={16} /> {loading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      {error && (
        <p ref={errorRef} tabIndex={-1} role="alert" className="field-error">{error}</p>
      )}

      {!overview && !trends && loading && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading overview" />}

      {(overview || trends) && (
        <div className={`${styles.dash} ${loading ? styles.refetching : ''}`} aria-busy={loading}>
          <div className={styles.tiles}>
            {tile('Bookings', trends?.bookings ? wholeNumber.format(trends.bookings.total) : null, rangeText, 'bookings', 'calendar', trends?.bookings?.byDay.map((d) => d.count), 'bookings.read')}
            {tile('Net recorded payments', trends?.payments ? dollars(trends.payments.netCents) : null, rangeText, 'finance', 'finance', trends?.payments?.byDay.map((d) => d.cents), 'payments.read')}
            {tile('Bookings completed', trends?.jobs ? wholeNumber.format(trends.jobs.completed) : null, `${trends?.jobs?.created ?? 0} created · ${rangeText}`, 'operations', 'work', trends?.jobs?.completedByDay.map((d) => d.count), 'jobs.read')}
            {tile('Outstanding balance', money('outstandingCents') === null ? null : dollars(money('outstandingCents') as number), 'Issued and partly paid, right now', 'finance', 'finance', undefined, 'invoices.read')}
            {tile('Customers', money('customers') === null ? null : wholeNumber.format(money('customers') as number), 'Recorded at this franchise, all time', 'customers', 'customers', undefined, 'customers.read')}
          </div>

          <div className={styles.chartGrid}>
            {trends?.bookings ? (
              <>
                <Card title="Bookings per day" sub={`${rangeText} · cancelled excluded${channel ? ' · ' + nice(channel) : ''}`} span={2}>
                  <ColumnChart points={bookingPts} unit="bookings" ariaLabel={`Bookings per day, ${rangeText}`} />
                  <DataTable caption="Bookings per day" head={['Date', 'Bookings']} rows={bookingPts.map((p) => [p.label, p.value])} />
                </Card>
                <Card title="Booking status" sub="All statuses in range">
                  <BarList rows={status.map((s) => ({ label: nice(s.key), value: s.count }))} emptyText="No bookings in this period." />
                </Card>
              </>
            ) : (
              <Locked title="Bookings" grant="bookings.read" />
            )}

            {trends?.payments ? (
              <>
                <Card title="Net recorded payments per day" sub={`${rangeText} · completed payments less refunds · not revenue or bank settlement`} span={2}>
                  <LineChart
                    labels={payPts.map((p) => p.date)}
                    series={[{ name: 'Net payments', values: payPts.map((p) => p.cents / 100), cls: 'seriesStroke1', dot: 'seriesFill1' }]}
                    fmt={(n) => '$' + compact.format(n)}
                    wash
                    ariaLabel={`Net recorded payments per day, ${rangeText}`}
                  />
                  <DataTable caption="Net recorded payments per day" head={['Date', 'Net payments']} rows={payPts.map((p) => [p.date, dollars(p.cents)])} />
                </Card>
                <Card title="Payments by method" sub="Net of refunds">
                  <BarList rows={trends.payments.byMethod.map((x) => ({ label: nice(x.key), value: x.cents, display: dollarsCompact(x.cents), sub: `${x.count} payments` }))} emptyText="No payments in this period." />
                </Card>
              </>
            ) : (
              <Locked title="Payments" grant="payments.read" />
            )}

            {trends?.jobs ? (
              <Card title="Bookings started and completed" sub={rangeText} span={2} legend={
                <ul className={styles.legend} aria-label="Legend">
                  <li><i className={styles.seriesKey1} aria-hidden="true" />Created</li>
                  <li><i className={styles.seriesKey2} aria-hidden="true" />Completed</li>
                </ul>
              }>
                <LineChart
                  labels={trends.jobs.createdByDay.map((d) => d.date)}
                  series={[
                    { name: 'Created', values: trends.jobs.createdByDay.map((d) => d.count), cls: 'seriesStroke1', dot: 'seriesFill1' },
                    { name: 'Completed', values: trends.jobs.completedByDay.map((d) => d.count), cls: 'seriesStroke2', dot: 'seriesFill2' },
                  ]}
                  ariaLabel={`Bookings started and completed per day, ${rangeText}`}
                />
                <DataTable caption="Bookings per day" head={['Date', 'Created', 'Completed']} rows={trends.jobs.createdByDay.map((d, i) => [d.date, d.count, trends.jobs?.completedByDay[i]?.count ?? 0])} />
              </Card>
            ) : (
              <Locked title="Bookings completed" grant="jobs.read" />
            )}

            {trends?.invoices ? (
              <Card title="Invoices by status" sub="Created in range · count, with total value">
                <BarList rows={trends.invoices.byStatus.map((x) => ({ label: nice(x.key), value: x.count, display: `${x.count} · ${dollarsCompact(x.cents)}`, sub: `${dollars(x.dueCents)} still due` }))} emptyText="No invoices created in this period." />
              </Card>
            ) : (
              <Locked title="Invoices" grant="invoices.read" />
            )}

            {trends?.bookings && (
              <>
                <Card title="Bookings by channel" sub={channel ? `Filter shows ${nice(channel)} highlighted` : 'Cancelled excluded'}>
                  <BarList highlight={channel ? nice(channel) : undefined} rows={trends.bookings.byChannel.map((c) => ({ label: nice(c.key), value: c.count }))} emptyText="No bookings in this period." />
                </Card>
                <Card title="Top services" sub="Most booked in range">
                  <BarList rows={trends.bookings.topServices.map((s) => ({ label: s.key, value: s.count }))} emptyText="No bookings in this period." />
                </Card>
              </>
            )}
          </div>

          <p className={styles.footnote}>
            Updated {new Date((trends ?? overview)!.generatedAt).toLocaleString()}. Figures cover this franchise only and come from internal records; they do not reconcile to external bank or card-processor statements. Days follow your browser timezone ({tz}).
          </p>
        </div>
      )}
    </section>
  );
}
