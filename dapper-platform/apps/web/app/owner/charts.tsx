'use client';

import { useEffect, useRef, useState } from 'react';
import styles from './owner.module.css';

/* Hand-built SVG charts (no dependency). Specs follow the dataviz method: thin marks (bars <= 24px, 4px rounded data end,
   2px lines, 10% area wash), hairline recessive grid, text in text tokens (never the series colour), a hover layer on every
   chart, and a table view for every chart so nothing depends on colour or hover. */

export type Point = { label: string; value: number };

function useWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(640);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const update = () => setWidth(Math.max(240, Math.floor(el.getBoundingClientRect().width)));
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return [ref, width] as const;
}

export function niceMax(max: number, steps = 4) {
  if (max <= 0) return steps;
  const raw = max / steps;
  const pow = Math.pow(10, Math.floor(Math.log10(raw)));
  const norm = raw / pow;
  const step = (norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 5 ? 5 : 10) * pow;
  return step * steps;
}

export const compact = new Intl.NumberFormat('en-CA', { notation: 'compact', maximumFractionDigits: 1 });
export const wholeNumber = new Intl.NumberFormat('en-CA');
export const dollars = (cents: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(cents / 100);
export const dollarsCompact = (cents: number) => '$' + compact.format(cents / 100);

export function shortDate(iso: string) {
  const d = new Date(iso + 'T00:00:00Z');
  return d.toLocaleDateString('en-CA', { month: 'short', day: 'numeric', timeZone: 'UTC' });
}

const M = { top: 12, right: 14, bottom: 26, left: 40 };

function Axes({ w, h, max, fmt, labels, ticks }: { w: number; h: number; max: number; fmt: (n: number) => string; labels: string[]; ticks: number }) {
  const iw = w - M.left - M.right;
  const ih = h - M.top - M.bottom;
  const every = Math.max(1, Math.ceil(labels.length / Math.max(2, Math.floor(iw / 70))));
  return (
    <g>
      {Array.from({ length: ticks + 1 }, (_, i) => {
        const y = M.top + ih - (ih * i) / ticks;
        return (
          <g key={i}>
            <line x1={M.left} x2={w - M.right} y1={y} y2={y} className={styles.grid} />
            <text x={M.left - 8} y={y + 4} textAnchor="end" className={styles.tick}>{fmt((max * i) / ticks)}</text>
          </g>
        );
      })}
      {labels.map((l, i) =>
        i % every === 0 ? (
          <text key={i} x={M.left + (iw * (i + 0.5)) / labels.length} y={h - 6} textAnchor="middle" className={styles.tick}>{shortDate(l)}</text>
        ) : null
      )}
    </g>
  );
}

export function ColumnChart({ points, fmt = (n) => wholeNumber.format(n), unit, height = 220, ariaLabel }: { points: Point[]; fmt?: (n: number) => string; unit: string; height?: number; ariaLabel: string }) {
  const [ref, w] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const max = niceMax(Math.max(0, ...points.map((p) => p.value)));
  const iw = w - M.left - M.right;
  const ih = height - M.top - M.bottom;
  const band = iw / Math.max(1, points.length);
  const bar = Math.min(24, Math.max(2, band - 2));
  return (
    <div ref={ref} className={styles.chartBox}>
      <svg width={w} height={height} role="img" aria-label={ariaLabel} onPointerLeave={() => setHover(null)}>
        <Axes w={w} h={height} max={max} fmt={fmt} labels={points.map((p) => p.label)} ticks={4} />
        {points.map((p, i) => {
          const bh = Math.max(p.value > 0 ? 2 : 0, (p.value / max) * ih);
          const x = M.left + band * i + (band - bar) / 2;
          const y = M.top + ih - bh;
          const r = Math.min(4, bar / 2, bh);
          return (
            <g key={p.label}>
              {bh > 0 && (
                <path
                  d={`M${x},${M.top + ih} V${y + r} Q${x},${y} ${x + r},${y} H${x + bar - r} Q${x + bar},${y} ${x + bar},${y + r} V${M.top + ih} Z`}
                  className={styles.seriesFill1}
                  opacity={hover === null || hover === i ? 1 : 0.55}
                />
              )}
              <rect x={M.left + band * i} y={M.top} width={band} height={ih} fill="transparent" onPointerEnter={() => setHover(i)} onPointerMove={() => setHover(i)} />
            </g>
          );
        })}
      </svg>
      {hover !== null && points[hover] && (
        <div className={styles.tooltip} style={{ left: Math.min(w - 150, Math.max(4, M.left + band * (hover + 0.5) - 70)), top: 4 }} role="status">
          <strong>{fmt(points[hover].value)}</strong>
          <span>{unit}</span>
          <small>{shortDate(points[hover].label)}</small>
        </div>
      )}
    </div>
  );
}

export type Series = { name: string; values: number[]; cls: 'seriesStroke1' | 'seriesStroke2'; dot: 'seriesFill1' | 'seriesFill2' };

export function LineChart({ labels, series, fmt = (n) => wholeNumber.format(n), height = 220, ariaLabel, wash = false }: { labels: string[]; series: Series[]; fmt?: (n: number) => string; height?: number; ariaLabel: string; wash?: boolean }) {
  const [ref, w] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const max = niceMax(Math.max(0, ...series.flatMap((s) => s.values)));
  const iw = w - M.left - M.right;
  const ih = height - M.top - M.bottom;
  const n = labels.length;
  const x = (i: number) => M.left + (iw * (i + 0.5)) / Math.max(1, n);
  const y = (v: number) => M.top + ih - (v / max) * ih;
  const path = (vals: number[]) => vals.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
  const move = (e: React.PointerEvent<SVGRectElement>) => {
    const box = e.currentTarget.getBoundingClientRect();
    setHover(Math.min(n - 1, Math.max(0, Math.floor(((e.clientX - box.left) / box.width) * n))));
  };
  return (
    <div ref={ref} className={styles.chartBox}>
      <svg width={w} height={height} role="img" aria-label={ariaLabel} onPointerLeave={() => setHover(null)}>
        <Axes w={w} h={height} max={max} fmt={fmt} labels={labels} ticks={4} />
        {wash && series[0] && n > 1 && (
          <path d={`${path(series[0].values)} L${x(n - 1)},${M.top + ih} L${x(0)},${M.top + ih} Z`} className={styles.seriesFill1} opacity={0.1} />
        )}
        {series.map((s) => (
          <g key={s.name}>
            <path d={path(s.values)} fill="none" className={styles[s.cls]} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
            {n > 0 && <circle cx={x(n - 1)} cy={y(s.values[n - 1] ?? 0)} r={4} className={styles[s.dot]} stroke="var(--surface)" strokeWidth={2} />}
          </g>
        ))}
        {hover !== null && <line x1={x(hover)} x2={x(hover)} y1={M.top} y2={M.top + ih} className={styles.crosshair} />}
        {hover !== null && series.map((s) => <circle key={s.name} cx={x(hover)} cy={y(s.values[hover] ?? 0)} r={4} className={styles[s.dot]} stroke="var(--surface)" strokeWidth={2} />)}
        <rect x={M.left} y={M.top} width={iw} height={ih} fill="transparent" onPointerMove={move} onPointerEnter={move} />
      </svg>
      {hover !== null && (
        <div className={styles.tooltip} style={{ left: Math.min(w - 170, Math.max(4, x(hover) + 10)), top: 4 }} role="status">
          <small>{shortDate(labels[hover])}</small>
          {series.map((s) => (
            <span key={s.name} className={styles.tipRow}>
              <i className={styles[s.cls.replace('Stroke', 'Key')]} aria-hidden="true" />
              <strong>{fmt(s.values[hover] ?? 0)}</strong>
              <span>{s.name}</span>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

export type Row = { label: string; value: number; display?: string; sub?: string };

/* Ranked categories: one colour (slot 1), value at the bar tip, no value-ramp on nominal categories. */
export function BarList({ rows, emptyText, highlight }: { rows: Row[]; emptyText: string; highlight?: string }) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  if (!rows.length) return <p className={styles.chartEmpty}>{emptyText}</p>;
  return (
    <ul className={styles.barList}>
      {rows.map((r) => (
        <li key={r.label} title={r.sub ? `${r.label}: ${r.display ?? r.value} · ${r.sub}` : `${r.label}: ${r.display ?? r.value}`}>
          <span className={styles.barLabel}>{r.label}</span>
          <span className={styles.barTrack}>
            <span className={`${styles.barFill} ${highlight && highlight !== r.label ? styles.barDim : ''}`} style={{ width: `${Math.max(r.value > 0 ? 2 : 0, (r.value / max) * 100)}%` }} />
          </span>
          <span className={styles.barValue}>{r.display ?? wholeNumber.format(r.value)}</span>
        </li>
      ))}
    </ul>
  );
}

export function Sparkline({ values }: { values: number[] }) {
  if (values.length < 2) return null;
  const w = 96;
  const h = 28;
  const max = Math.max(1, ...values);
  const pts = values.map((v, i) => `${i ? 'L' : 'M'}${((w - 6) * i) / (values.length - 1) + 3},${h - 4 - ((h - 8) * v) / max}`).join(' ');
  const lastY = h - 4 - ((h - 8) * values[values.length - 1]) / max;
  return (
    <svg width={w} height={h} aria-hidden="true" className={styles.spark}>
      <path d={pts} fill="none" className={styles.seriesStroke1} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={w - 3} cy={lastY} r={3} className={styles.seriesFill1} stroke="var(--surface)" strokeWidth={2} />
    </svg>
  );
}

export function DataTable({ caption, head, rows }: { caption: string; head: string[]; rows: (string | number)[][] }) {
  return (
    <details className={styles.tableView}>
      <summary>View as table</summary>
      <div className={styles.tableScroll}>
        <table>
          <caption>{caption}</caption>
          <thead>
            <tr>{head.map((h) => <th key={h} scope="col">{h}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>{r.map((c, j) => (j === 0 ? <th key={j} scope="row">{c}</th> : <td key={j}>{c}</td>))}</tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  );
}
