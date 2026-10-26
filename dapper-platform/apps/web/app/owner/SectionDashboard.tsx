'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import styles from './owner.module.css';
import AnimatedNumber from '../motion/AnimatedNumber';
import RangeBar, { resolveRange, type Custom, type RangePresetId } from './DateRange';
import { BarList, ColumnChart, DataTable, LineChart, Sparkline, compact, dollars, dollarsCompact, shortDate, wholeNumber } from './charts';

type Key = { key: string; count: number; cents: number; dueCents: number };
type Day = { date: string; count: number };
type Trends = {
  generatedAt: string;
  payments: { byDay: { date: string; cents: number }[]; byMethod: Key[]; netCents: number } | null;
  invoices: { byStatus: Key[] } | null;
  jobs: { createdByDay: Day[]; completedByDay: Day[]; created: number; completed: number; open: number } | null;
};
type Insights = {
  finance: { invoicedByDay: { date: string; count: number; cents: number }[]; aging: Key[]; invoicedCents: number; quotes: Key[] | null; refundedCents: number } | null;
  work: { byService: Key[]; openAge: Key[]; avgHoursToComplete: number | null; completed: number } | null;
  customers: { newByDay: Day[]; new: number; total: number; vehicles: number | null; bookingsSplit: Key[] | null } | null;
};
export type DashSection = 'finance' | 'work' | 'customers';

const PRESETS: RangePresetId[] = ['last7', 'last30', 'last90', 'month', 'year'];
const nice = (s: string) => s.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase());
// Fixed display order for the overdue buckets so the chart reads from healthy to late.
const AGING_ORDER = ['Not yet due', 'No due date', '1–30 days overdue', '31–60 days overdue', 'Over 60 days overdue'];
const OPEN_ORDER = ['Under 1 day', '1–3 days', '4–7 days', 'Over 7 days'];
const SPLIT_ORDER = ['No bookings', '1 booking', '2–3 bookings', '4 or more'];
const order = (rows: Key[], by: string[]) => [...rows].sort((a, b) => by.indexOf(a.key) - by.indexOf(b.key));

function Card({ title, sub, span = 1, children }: { title: string; sub?: string; span?: 1 | 2; children: ReactNode }) {
  return (
    <section className={`${styles.chartCard} ${span === 2 ? styles.span2 : ''}`}>
      <div className={styles.chartHead}>
        <div>
          <h3>{title}</h3>
          {sub && <p>{sub}</p>}
        </div>
      </div>
      {children}
    </section>
  );
}
function Locked({ title, grant }: { title: string; grant: string }) {
  return (
    <section className={`${styles.chartCard} ${styles.locked}`}>
      <h3>{title}</h3>
      <p className={styles.chartEmpty}>Access needed. Your role does not include <code>{grant}</code> here, so nothing is shown rather than a misleading zero.</p>
    </section>
  );
}
function Tile({ label, value, foot, spark, grant }: { label: string; value: string | null; foot: string; spark?: number[]; grant?: string }) {
  return (
    <div className={styles.tile} style={{ cursor: 'default' }}>
      <span className={styles.tileLabel}>{label}</span>
      <span className={`${styles.tileValue} ${value === null ? styles.valueMuted : ''}`}>{value === null ? 'Access needed' : <AnimatedNumber text={value} />}</span>
      <span className={styles.tileFoot}>
        <span>{value === null && grant ? `Needs ${grant}` : foot}</span>
        {value !== null && spark && <Sparkline values={spark} />}
      </span>
    </div>
  );
}

export default function SectionDashboard({ section, base }: { section: DashSection; base: string }) {
  const [preset, setPreset] = useState<RangePresetId>('last30');
  const [custom, setCustom] = useState<Custom>(() => resolveRange('last30', { from: '', to: '' }));
  const [trends, setTrends] = useState<Trends | null>(null);
  const [ins, setIns] = useState<Insights | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [refresh, setRefresh] = useState(0);
  const range = useMemo(() => resolveRange(preset, custom), [preset, custom]);
  const tz = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', []);

  useEffect(() => {
    let live = true;
    setLoading(true);
    setError('');
    const q = new URLSearchParams({ from: range.from, to: range.to, tz });
    Promise.all([
      fetch(`${base}/overview/trends?${q}`, { cache: 'no-store' }),
      fetch(`${base}/overview/insights?${new URLSearchParams({ section, from: range.from, to: range.to, tz })}`, { cache: 'no-store' }),
    ])
      .then(async ([t, i]) => {
        if (!t.ok && !i.ok) throw new Error(t.status === 403 && i.status === 403 ? 'Your role has no read access for these figures at this franchise.' : 'The figures could not be loaded. Please retry.');
        const [tb, ib] = await Promise.all([t.ok ? t.json() : null, i.ok ? i.json() : null]);
        if (live) { setTrends(tb); setIns(ib); }
      })
      .catch((e) => live && setError(e.message))
      .finally(() => live && setLoading(false));
    return () => { live = false; };
  }, [base, section, range, tz, refresh]);

  const rangeText = `${shortDate(range.from)} – ${shortDate(range.to)}`;
  const f = ins?.finance;
  const w = ins?.work;
  const c = ins?.customers;
  const payments = trends?.payments;
  const jobs = trends?.jobs;

  return (
    <details className={styles.insights} open>
      <summary>Insights</summary>
      <div className={styles.stack}>
        <div className={styles.filterBar} role="group" aria-label="Insight filters">
          <RangeBar presets={PRESETS} preset={preset} custom={custom} onPreset={setPreset} onCustom={setCustom} />
          <button type="button" className="secondary" disabled={loading} onClick={() => setRefresh((n) => n + 1)}>{loading ? 'Refreshing…' : 'Refresh'}</button>
        </div>
        {error && <p role="alert" className="field-error">{error}</p>}
        {!trends && !ins && loading && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading insights" />}

        {(trends || ins) && (
          <div className={`${styles.dash} ${loading ? styles.refetching : ''}`} aria-busy={loading}>
            {section === 'finance' && (
              <>
                <div className={styles.tiles}>
                  <Tile label="Invoiced" value={f ? dollars(f.invoicedCents) : null} foot={`Issued ${rangeText}`} spark={f?.invoicedByDay.map((d) => d.cents)} grant="invoices.read" />
                  <Tile label="Net recorded payments" value={payments ? dollars(payments.netCents) : null} foot={rangeText} spark={payments?.byDay.map((d) => d.cents)} grant="payments.read" />
                  <Tile label="Outstanding now" value={f ? dollars(f.aging.reduce((a, k) => a + k.dueCents, 0)) : null} foot={`${f?.aging.reduce((a, k) => a + k.count, 0) ?? 0} invoices with a balance`} grant="invoices.read" />
                  <Tile label="Refunded" value={f && payments ? dollars(f.refundedCents) : null} foot={`On payments recorded ${rangeText}`} grant="payments.read" />
                </div>
                <div className={styles.chartGrid}>
                  {f ? (
                    <Card title="Invoiced per day" sub={`${rangeText} · issued invoices, draft and void excluded`} span={2}>
                      <ColumnChart points={f.invoicedByDay.map((d) => ({ label: d.date, value: d.cents / 100 }))} fmt={(n) => '$' + compact.format(n)} unit="invoiced" ariaLabel={`Invoiced per day, ${rangeText}`} />
                      <DataTable caption="Invoiced per day" head={['Date', 'Invoices', 'Total']} rows={f.invoicedByDay.map((d) => [d.date, d.count, dollars(d.cents)])} />
                    </Card>
                  ) : <Locked title="Invoices" grant="invoices.read" />}
                  {f ? (
                    <Card title="Outstanding by age" sub="Balance due now, by days past the due date">
                      <BarList rows={order(f.aging, AGING_ORDER).map((k) => ({ label: k.key, value: k.dueCents, display: dollarsCompact(k.dueCents), sub: `${k.count} invoices` }))} emptyText="No outstanding balances." />
                    </Card>
                  ) : null}
                  {payments ? (
                    <Card title="Net recorded payments per day" sub={`${rangeText} · completed payments less refunds`} span={2}>
                      <LineChart labels={payments.byDay.map((p) => p.date)} series={[{ name: 'Net payments', values: payments.byDay.map((p) => p.cents / 100), cls: 'seriesStroke1', dot: 'seriesFill1' }]} fmt={(n) => '$' + compact.format(n)} wash ariaLabel={`Net recorded payments per day, ${rangeText}`} />
                      <DataTable caption="Net recorded payments per day" head={['Date', 'Net payments']} rows={payments.byDay.map((p) => [p.date, dollars(p.cents)])} />
                    </Card>
                  ) : <Locked title="Payments" grant="payments.read" />}
                  {payments && (
                    <Card title="Payments by method" sub="Net of refunds">
                      <BarList rows={payments.byMethod.map((x) => ({ label: nice(x.key), value: x.cents, display: dollarsCompact(x.cents), sub: `${x.count} payments` }))} emptyText="No payments in this period." />
                    </Card>
                  )}
                  {trends?.invoices && (
                    <Card title="Invoices by status" sub="Created in range · count and value">
                      <BarList rows={trends.invoices.byStatus.map((x) => ({ label: nice(x.key), value: x.count, display: `${x.count} · ${dollarsCompact(x.cents)}` }))} emptyText="No invoices created in this period." />
                    </Card>
                  )}
                  {f?.quotes ? (
                    <Card title="Quotes by status" sub="Created in range · count and value">
                      <BarList rows={f.quotes.map((x) => ({ label: nice(x.key), value: x.count, display: `${x.count} · ${dollarsCompact(x.cents)}` }))} emptyText="No quotes created in this period." />
                    </Card>
                  ) : f ? <Locked title="Quotes" grant="quotes.read" /> : null}
                </div>
              </>
            )}

            {section === 'work' && (
              <>
                <div className={styles.tiles}>
                  <Tile label="Created" value={jobs ? wholeNumber.format(jobs.created) : null} foot={rangeText} spark={jobs?.createdByDay.map((d) => d.count)} grant="jobs.read" />
                  <Tile label="Completed" value={jobs ? wholeNumber.format(jobs.completed) : null} foot={rangeText} spark={jobs?.completedByDay.map((d) => d.count)} grant="jobs.read" />
                  <Tile label="Average time to complete" value={w ? (w.avgHoursToComplete === null ? 'No data' : w.avgHoursToComplete < 48 ? `${w.avgHoursToComplete.toFixed(1)} h` : `${(w.avgHoursToComplete / 24).toFixed(1)} days`) : null} foot={`${w?.completed ?? 0} completed ${rangeText}`} grant="jobs.read" />
                </div>
                <div className={styles.chartGrid}>
                  {jobs ? (
                    <Card title="Bookings started and completed" sub={rangeText} span={2}>
                      <ul className={styles.legend} aria-label="Legend"><li><i className={styles.seriesKey1} aria-hidden="true" />Created</li><li><i className={styles.seriesKey2} aria-hidden="true" />Completed</li></ul>
                      <LineChart labels={jobs.createdByDay.map((d) => d.date)} series={[{ name: 'Created', values: jobs.createdByDay.map((d) => d.count), cls: 'seriesStroke1', dot: 'seriesFill1' }, { name: 'Completed', values: jobs.completedByDay.map((d) => d.count), cls: 'seriesStroke2', dot: 'seriesFill2' }]} ariaLabel={`Bookings started and completed, ${rangeText}`} />
                      <DataTable caption="Bookings per day" head={['Date', 'Created', 'Completed']} rows={jobs.createdByDay.map((d, i) => [d.date, d.count, jobs.completedByDay[i]?.count ?? 0])} />
                    </Card>
                  ) : <Locked title="Bookings completed" grant="jobs.read" />}
                  {w && (
                    <>
                      <Card title="Top services" sub={`Bookings started ${rangeText}`}>
                        <BarList rows={w.byService.map((k) => ({ label: k.key, value: k.count }))} emptyText="No bookings started in this period." />
                      </Card>
                    </>
                  )}
                </div>
              </>
            )}

            {section === 'customers' && (
              <>
                <div className={styles.tiles}>
                  <Tile label="Customers" value={c ? wholeNumber.format(c.total) : null} foot="Recorded at this franchise" grant="customers.read" />
                  <Tile label="New customers" value={c ? wholeNumber.format(c.new) : null} foot={rangeText} spark={c?.newByDay.map((d) => d.count)} grant="customers.read" />
                  <Tile label="Vehicles and equipment" value={c ? (c.vehicles === null ? null : wholeNumber.format(c.vehicles)) : null} foot="Recorded at this franchise" grant="vehicles.read" />
                </div>
                <div className={styles.chartGrid}>
                  {c ? (
                    <Card title="New customers per day" sub={rangeText} span={2}>
                      <ColumnChart points={c.newByDay.map((d) => ({ label: d.date, value: d.count }))} unit="new customers" ariaLabel={`New customers per day, ${rangeText}`} />
                      <DataTable caption="New customers per day" head={['Date', 'New customers']} rows={c.newByDay.map((d) => [d.date, d.count])} />
                    </Card>
                  ) : <Locked title="Customers" grant="customers.read" />}
                  {c?.bookingsSplit ? (
                    <Card title="Customers by bookings" sub="Non-cancelled bookings, all time">
                      <BarList rows={order(c.bookingsSplit, SPLIT_ORDER).map((k) => ({ label: k.key, value: k.count }))} emptyText="No customers yet." />
                    </Card>
                  ) : c ? <Locked title="Bookings per customer" grant="bookings.read" /> : null}
                </div>
              </>
            )}
            <p className={styles.footnote}>Updated {new Date((ins?.finance ? trends?.generatedAt : trends?.generatedAt) ?? Date.now()).toLocaleString()}. Figures cover this franchise only, come from internal records and do not reconcile to bank or card-processor statements. Days follow your browser timezone ({tz}).</p>
          </div>
        )}
      </div>
    </details>
  );
}
