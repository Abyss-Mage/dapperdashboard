'use client';
import { useEffect, useMemo, useState } from 'react';
import { ArrowUpRight } from './Icons';

/* Instant quote, laid out like the work order on the bay wall. It reads the same public booking catalogue as the booking page,
   so the numbers always match. When online pricing is not published the whole section stays away rather than showing a dead tool. */

type Rule = { serviceId: string; serviceName: string; durationMinutes: number; availableAddons: { serviceId: string; name: string }[] };
type Options = {
  services: { id: string; priceCents: number }[];
  rules: Rule[];
  taxes: { rates: { active: boolean; rateBasisPoints: number }[]; settings: { rounding: string } };
};
type Pack = { rule: Rule; vehicle: string; name: string };

const VEHICLE = /^(hatchback|sedan|suv|pickup|van|2 wheeler|aircraft (?:small|medium|large)|boat (?:small|medium|large))\s*-\s*(.+)$/i;
const money = (c: number) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);
const span = (m: number) => {
  if (m < 60) return m + ' min';
  if (m < 1440) { const h = Math.floor(m / 60), r = m % 60; return h + (h === 1 ? ' hr' : ' hrs') + (r ? ' ' + r + ' min' : ''); }
  const d = Math.floor(m / 1440), h = Math.floor((m % 1440) / 60); return d + (d === 1 ? ' day' : ' days') + (h ? ' ' + h + (h === 1 ? ' hr' : ' hrs') : '');
};
const label = (v: string) => (v.toLowerCase() === 'suv' ? 'SUV' : v.replace(/\b\w/g, (c) => c.toUpperCase()));

export default function Quote() {
  const [data, setData] = useState<Options | null>(null);
  const [vehicle, setVehicle] = useState('');
  const [pack, setPack] = useState<Pack | null>(null);
  const [extras, setExtras] = useState<Set<string>>(new Set());

  useEffect(() => {
    let live = true;
    fetch('/api/v1/public/booking/dapper/options', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((o: Options) => { if (live) setData(o); })
      .catch(() => { /* stays hidden */ });
    return () => { live = false; };
  }, []);

  const model = useMemo(() => {
    if (!data) return null;
    const price = new Map(data.services.map((s) => [s.id, s.priceCents]));
    const packs: Pack[] = data.rules
      .map((rule) => { const m = VEHICLE.exec(rule.serviceName.trim()); return { rule, vehicle: m ? label(m[1]) : '', name: m ? m[2].trim() : rule.serviceName.trim() }; })
      .filter((p) => price.has(p.rule.serviceId));
    const vehicles = [...new Set(packs.map((p) => p.vehicle).filter(Boolean))].sort();
    return { price, packs, vehicles };
  }, [data]);

  useEffect(() => { if (model && !vehicle && model.vehicles[0]) setVehicle(model.vehicles[0]); }, [model, vehicle]);

  if (!data || !model || !model.packs.length) return null;
  const { price, packs, vehicles } = model;
  const shown = packs.filter((p) => !p.vehicle || p.vehicle === vehicle);
  const chosen = pack && shown.includes(pack) ? pack : null;
  const addOns = chosen ? chosen.rule.availableAddons.filter((a) => price.has(a.serviceId)) : [];
  const sub = (chosen ? price.get(chosen.rule.serviceId)! : 0) + [...extras].reduce((s, id) => s + (price.get(id) || 0), 0);
  const tax = data.taxes.rates.filter((t) => t.active).reduce((s, t) => {
    const n = sub * t.rateBasisPoints; let q = Math.floor(n / 10000); const r = n % 10000;
    if (data.taxes.settings.rounding === 'half_even') q += r > 5000 || (r === 5000 && q % 2) ? 1 : 0;
    else if (data.taxes.settings.rounding !== 'down') q += r >= 5000 ? 1 : 0;
    return s + q;
  }, 0);

  return (
    <section className="section" id="quote" aria-labelledby="quote-title">
      <div className="wrap">
        <div className="head-row">
          <div>
            <h2 id="quote-title" className="headline">See your price.<br /><span className="after">Before you book.</span></h2>
            <p className="lede">Choose your vehicle and package for an estimate with tax, from our current prices.</p>
          </div>
        </div>
        <div className="ticket">
          <div>
            {vehicles.length > 0 && (
              <div className="ticket__block">
                <h3>What are we caring for?</h3>
                <div className="chips" role="group" aria-label="Vehicle type">
                  {vehicles.map((v) => <button key={v} type="button" className="chip" aria-pressed={v === vehicle} onClick={() => { setVehicle(v); setPack(null); setExtras(new Set()); }}>{v}</button>)}
                </div>
              </div>
            )}
            <div className="ticket__block">
              <h3>Choose a package</h3>
              <div className="lines">
                {shown.map((p) => (
                  <button key={p.rule.serviceId} type="button" className="line-item" aria-pressed={p === chosen} onClick={() => { setPack(p); setExtras(new Set()); }}>
                    <span><strong>{p.name}</strong><small>About {span(p.rule.durationMinutes)}</small></span>
                    <b>{money(price.get(p.rule.serviceId)!)}</b>
                  </button>
                ))}
              </div>
            </div>
            {addOns.length > 0 && (
              <div className="ticket__block">
                <h3>Add anything extra?</h3>
                <div className="lines">
                  {addOns.map((a) => (
                    <label key={a.serviceId} className="line-item">
                      <input type="checkbox" checked={extras.has(a.serviceId)} onChange={(e) => { const next = new Set(extras); if (e.target.checked) next.add(a.serviceId); else next.delete(a.serviceId); setExtras(next); }} />
                      <span><strong>{a.name}</strong></span>
                      <b>+{money(price.get(a.serviceId)!)}</b>
                    </label>
                  ))}
                </div>
              </div>
            )}
          </div>
          <aside className="tally" aria-live="polite">
            {chosen ? (
              <>
                <dl>
                  <div><dt>Subtotal</dt><dd>{money(sub)}</dd></div>
                  <div><dt>Tax</dt><dd>{money(tax)}</dd></div>
                  <div className="grand"><dt>Estimated total</dt><dd>{money(sub + tax)}</dd></div>
                </dl>
                <a className="btn btn--red" href="/booking">Book this<ArrowUpRight size={20} aria-hidden /></a>
                <p>An estimate from our current prices. Your final price is confirmed when you book; condition and size can change it.</p>
              </>
            ) : <p style={{ marginTop: 0, fontSize: 18, fontWeight: 600 }}>Pick a package to see the price.</p>}
          </aside>
        </div>
      </div>
    </section>
  );
}
