'use client';

import { useMemo } from 'react';

/* A small live map of where clocked-in staff are, drawn from OpenStreetMap tiles (no map library).
   The view fits every marker; each marker shows the person's name. Tiles © OpenStreetMap contributors. */

export type MapPoint = { id: string; label: string; lat: number; lon: number; stale?: boolean };

const TILE = 256;
const project = (lat: number, lon: number, z: number) => {
  const s = Math.sin((Math.max(-85, Math.min(85, lat)) * Math.PI) / 180);
  const n = TILE * 2 ** z;
  return { x: ((lon + 180) / 360) * n, y: (0.5 - Math.log((1 + s) / (1 - s)) / (4 * Math.PI)) * n };
};

export default function StaffMap({ points, height = 420 }: { points: MapPoint[]; height?: number }) {
  const width = 960; // drawn at this width and scaled to the box
  const view = useMemo(() => {
    if (!points.length) return null;
    // Highest zoom (max 16) at which every point fits with a margin.
    let z = 16;
    for (; z > 2; z--) {
      const p = points.map((q) => project(q.lat, q.lon, z));
      const w = Math.max(...p.map((q) => q.x)) - Math.min(...p.map((q) => q.x));
      const h = Math.max(...p.map((q) => q.y)) - Math.min(...p.map((q) => q.y));
      if (w < width - 160 && h < height - 120) break;
    }
    const p = points.map((q) => project(q.lat, q.lon, z));
    const cx = (Math.max(...p.map((q) => q.x)) + Math.min(...p.map((q) => q.x))) / 2;
    const cy = (Math.max(...p.map((q) => q.y)) + Math.min(...p.map((q) => q.y))) / 2;
    const left = cx - width / 2, top = cy - height / 2;
    const tiles: { key: string; src: string; x: number; y: number }[] = [];
    const max = 2 ** z;
    for (let tx = Math.floor(left / TILE); tx <= Math.floor((left + width) / TILE); tx++)
      for (let ty = Math.floor(top / TILE); ty <= Math.floor((top + height) / TILE); ty++) {
        if (ty < 0 || ty >= max) continue;
        const wx = ((tx % max) + max) % max;
        tiles.push({ key: `${z}/${tx}/${ty}`, src: `/api/v1/map-tiles/${z}/${wx}/${ty}`, x: tx * TILE - left, y: ty * TILE - top });
      }
    return { tiles, marks: points.map((q, i) => ({ ...q, x: p[i].x - left, y: p[i].y - top })) };
  }, [points, height]);

  if (!view) return <div style={{ height: 160, display: 'grid', placeItems: 'center', borderRadius: 16, border: '1px dashed var(--border)', color: 'var(--text-muted)' }}>No one is sharing their location right now.</div>;
  return (
    <figure style={{ margin: 0 }}>
      <div style={{ position: 'relative', width: '100%', aspectRatio: `${width} / ${height}`, overflow: 'hidden', borderRadius: 16, border: '1px solid var(--border)', background: '#dfe6ea' }}>
        <svg viewBox={`0 0 ${width} ${height}`} style={{ position: 'absolute', inset: 0, width: '100%', height: '100%' }} role="img" aria-label={`Map with ${points.length} staff location${points.length === 1 ? '' : 's'}`}>
          {view.tiles.map((t) => <image key={t.key} href={t.src} x={t.x} y={t.y} width={TILE} height={TILE} />)}
          {view.marks.map((m) => (
            <g key={m.id} transform={`translate(${m.x},${m.y})`} opacity={m.stale ? 0.55 : 1}>
              <circle r="9" fill={m.stale ? '#8a8f96' : '#fb1a2b'} stroke="#fff" strokeWidth="3" />
              <rect x="13" y="-13" rx="6" width={Math.max(40, m.label.length * 7.4 + 14)} height="24" fill="rgba(17,17,17,.82)" />
              <text x="20" y="4" fontSize="13" fontWeight="600" fill="#fff">{m.label}</text>
            </g>
          ))}
        </svg>
      </div>
      <figcaption style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 6 }}>Map tiles © <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap</a> contributors. Grey markers have not updated recently.</figcaption>
    </figure>
  );
}
