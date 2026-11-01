'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import Select from '../GlassSelect';
import { DatePicker } from '../Pickers';

type Kind = 'appointments' | 'revenue' | 'customers';
type Report = {
  kind: Kind; from: string; to: string; columns: string[]; rows: string[][]; stats: { label: string; value: string }[];
  series: { day: string; count: number; cents: number }[]; serviceNames: string[]; truncated: boolean;
};

const KINDS: [Kind, string][] = [['appointments', 'Appointments'], ['revenue', 'Revenue'], ['customers', 'Customers']];
const STATUSES: Record<Kind, [string, string][]> = {
  appointments: [['', 'All statuses'], ['confirmed', 'Approved'], ['completed', 'Completed'], ['cancelled', 'Cancelled']],
  customers: [['', 'All statuses'], ['confirmed', 'Approved'], ['completed', 'Completed'], ['cancelled', 'Cancelled']],
  revenue: [['', 'All invoices'], ['paid', 'Paid'], ['partially_paid', 'Partly paid'], ['issued', 'Not paid yet']],
};
const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const daysAgo = (n: number) => { const d = new Date(); d.setDate(d.getDate() - n); return iso(d); };
const PRESETS: [string, string, () => [string, string]][] = [
  ['7', 'Last 7 days', () => [daysAgo(6), iso(new Date())]],
  ['30', 'Last 30 days', () => [daysAgo(29), iso(new Date())]],
  ['month', 'This month', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth(), 1)), iso(n)]; }],
  ['lastmonth', 'Last month', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth() - 1, 1)), iso(new Date(n.getFullYear(), n.getMonth(), 0))]; }],
  ['year', 'This year', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), 0, 1)), iso(n)]; }],
];

const esc = (v: string) => v.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);

/* Appointments, revenue and customers reports for the selected franchise, filtered by dates, service and status, with a chart,
   headline figures and a download as CSV (a spreadsheet) or PDF (print the page and choose Save as PDF). */
export default function Reports({ base }: { base: string }) {
  const [kind, setKind] = useState<Kind>('appointments');
  const [from, setFrom] = useState(() => daysAgo(29));
  const [to, setTo] = useState(() => iso(new Date()));
  const [service, setService] = useState('');
  const [status, setStatus] = useState('');
  const [report, setReport] = useState<Report | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const tz = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const q = new URLSearchParams({ from, to, tz });
      if (service) q.set('service', service);
      if (status) q.set('status', status);
      const r = await fetch(`${base}/reports/${kind}?${q}`, { cache: 'no-store' });
      const b = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(b.error ?? 'The report could not be loaded. You may not have access to it.');
      setReport(b); setError('');
    } catch (e) { setReport(null); setError(e instanceof Error ? e.message : 'The report could not be loaded.'); }
    finally { setLoading(false); }
  }, [base, kind, from, to, tz, service, status]);
  useEffect(() => { const id = setTimeout(() => void load(), 200); return () => clearTimeout(id); }, [load]);

  const title = `${KINDS.find(([k]) => k === kind)?.[1]} report`;
  const filterText = [`${from} to ${to}`, service || 'All services', STATUSES[kind].find(([v]) => v === status)?.[1] ?? ''].join(' · ');

  function downloadCsv() {
    if (!report) return;
    // Cells that start like a formula are prefixed so a spreadsheet shows them as text instead of running them.
    const lead = ['=', '+', '-', '@', String.fromCharCode(9), String.fromCharCode(13)];
    const cell = (v: string) => { let t = String(v ?? ''); if (lead.includes(t.charAt(0)) && !/^-?\$?[\d,.]+$/.test(t)) t = "'" + t; return '"' + t.replaceAll('"', '""') + '"'; };
    const lines = [report.columns, ...report.rows].map((row) => row.map(cell).join(','));
    const blob = new Blob([String.fromCharCode(0xfeff) + lines.join(String.fromCharCode(13, 10))], { type: 'text/csv;charset=utf-8' });
    const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = `${kind}-report-${from}-to-${to}.csv`; a.click(); URL.revokeObjectURL(a.href);
  }
  function downloadPdf() {
    if (!report) return;
    const w = window.open('', '_blank');
    if (!w) { setError('Allow pop-ups for this site to open the PDF page.'); return; }
    const stats = report.stats.map((s) => `<div class="s"><b>${esc(s.value)}</b><span>${esc(s.label)}</span></div>`).join('');
    const head = report.columns.map((c) => `<th>${esc(c)}</th>`).join('');
    const body = report.rows.map((r) => `<tr>${r.map((c) => `<td>${esc(c)}</td>`).join('')}</tr>`).join('');
    w.document.open();
    w.document.write(`<!doctype html><html><head><meta charset="utf-8"><title>${esc(title)}</title><style>@page{size:landscape;margin:12mm}body{font:12px system-ui,sans-serif;color:#17171a;margin:0}.bar{position:sticky;top:0;padding:10px;background:#17171a;text-align:center}.bar button{font:600 13px system-ui;padding:8px 16px;border:0;border-radius:999px;background:#d22b2b;color:#fff;cursor:pointer}.page{padding:18px}h1{font-size:20px;margin:0 0 2px}.f{color:#555;margin:0 0 14px}.stats{display:flex;gap:12px;flex-wrap:wrap;margin-bottom:14px}.s{border:1px solid #ccc;border-radius:8px;padding:8px 12px;min-width:110px}.s b{display:block;font-size:16px}.s span{color:#555}table{width:100%;border-collapse:collapse}th{text-align:left;background:#17171a;color:#fff;padding:6px 8px;font-size:11px;text-transform:uppercase;letter-spacing:.04em}td{padding:5px 8px;border-bottom:1px solid #ddd}tr:nth-child(even) td{background:#f6f6f7}@media print{.bar{display:none}th{-webkit-print-color-adjust:exact;print-color-adjust:exact}}</style></head><body><div class="bar"><button onclick="window.print()">Print or save as PDF</button></div><div class="page"><h1>${esc(title)}</h1><p class="f">${esc(filterText)} · made ${esc(new Date().toLocaleString('en-CA'))}</p><div class="stats">${stats}</div><table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>${report.truncated ? '<p>Only the first 5,000 rows are shown. Narrow the dates to see the rest.</p>' : ''}</div></body></html>`);
    w.document.close();
  }

  const max = Math.max(1, ...(report?.series ?? []).map((p) => (kind === 'revenue' ? p.cents : p.count)));
  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <h2 className={styles.setupTitle} style={{ fontSize: 20, margin: 0 }}>Reports</h2>
        <div className={styles.svcActions}>
          <button type="button" className="secondary" disabled={!report || report.rows.length === 0} onClick={downloadCsv}>Download CSV</button>
          <button type="button" className="secondary" disabled={!report || report.rows.length === 0} onClick={downloadPdf}>Download PDF</button>
        </div>
      </div>
      <div className={styles.segments} role="group" aria-label="Report">
        {KINDS.map(([k, label]) => <button key={k} type="button" className={styles.segment} aria-pressed={kind === k} onClick={() => { setKind(k); setStatus(''); }}>{label}</button>)}
      </div>
      <div className={styles.custFilters}>
        <DatePicker value={from} max={to} onChange={(e) => e.target.value && setFrom(e.target.value)} aria-label="From date" />
        <DatePicker value={to} min={from} onChange={(e) => e.target.value && setTo(e.target.value)} aria-label="To date" />
        <Select value={service} onChange={(e) => setService(e.target.value)} aria-label="Service"><option value="">All services</option>{(report?.serviceNames ?? []).map((n) => <option key={n} value={n}>{n}</option>)}</Select>
        <Select value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Status">{STATUSES[kind].map(([v, l]) => <option key={v} value={v}>{l}</option>)}</Select>
        <Select value="" onChange={(e) => { const p = PRESETS.find(([id]) => id === e.target.value); if (p) { const [a, b] = p[2](); setFrom(a); setTo(b); } }} aria-label="Quick range"><option value="">Quick range</option>{PRESETS.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</Select>
      </div>
      {error && <p role="alert" className="field-error">{error}</p>}
      {loading && !report && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading report" />}
      {report && (
        <>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 10, opacity: loading ? 0.6 : 1 }}>
            {report.stats.map((s) => (
              <div key={s.label} className={styles.resource} style={{ padding: '12px 14px' }}>
                <div style={{ fontSize: 22, fontWeight: 800, fontVariantNumeric: 'tabular-nums' }}>{s.value}</div>
                <div className={styles.personSub}>{s.label}</div>
              </div>
            ))}
          </div>
          {kind !== 'customers' && report.series.length > 0 && (
            <div className={styles.resource} aria-label={kind === 'revenue' ? 'Net payments per day' : 'Appointments per day'}>
              <div style={{ display: 'flex', alignItems: 'flex-end', gap: report.series.length > 60 ? 1 : 3, height: 140, overflowX: 'auto' }}>
                {report.series.map((p) => {
                  const v = kind === 'revenue' ? p.cents : p.count;
                  return <div key={p.day} title={`${p.day}: ${kind === 'revenue' ? '$' + (p.cents / 100).toFixed(2) : p.count}`} style={{ flex: '1 0 6px', minWidth: 6, height: `${Math.max(2, (v / max) * 100)}%`, background: v ? 'var(--accent, #d22b2b)' : 'rgba(255,255,255,.12)', borderRadius: '3px 3px 0 0' }} />;
                })}
              </div>
              <div className={styles.rowBetween} style={{ marginTop: 6 }}><span className={styles.personSub}>{report.series[0].day}</span><span className={styles.personSub}>{report.series[report.series.length - 1].day}</span></div>
            </div>
          )}
          {report.rows.length === 0 ? <p className={styles.status}>Nothing matches these filters.</p> : (
            <div style={{ overflowX: 'auto' }}>
              <table className={styles.reportTable}>
                <thead><tr>{report.columns.map((c) => <th key={c}>{c}</th>)}</tr></thead>
                <tbody>{report.rows.map((row, i) => <tr key={i}>{row.map((cell, j) => <td key={j}>{cell}</td>)}</tr>)}</tbody>
              </table>
            </div>
          )}
          {report.truncated && <p className={styles.status}>Only the first 5,000 rows are shown. Narrow the dates to see the rest.</p>}
        </>
      )}
    </div>
  );
}
