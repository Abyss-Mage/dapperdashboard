/* Instant quote: pick the vehicle, the package and any extras, and see the price with tax. It reads the same public booking
   catalogue the booking page uses, so the numbers always match. If online pricing is not available (no published site, no network)
   the whole section stays hidden rather than showing a dead tool. */
(function () {
  const root = document.getElementById('quote');
  if (!root) return;
  const api = '/api/v1/public/booking/main/options';
  const VEHICLE = /^(hatchback|sedan|suv|pickup|van|2 wheeler|aircraft (?:small|medium|large)|boat (?:small|medium|large))\s*-\s*(.+)$/i;
  const money = (c) => new Intl.NumberFormat('en-CA', { style: 'currency', currency: 'CAD' }).format(c / 100);
  const span = (m) => { if (m < 60) return m + ' min'; if (m < 1440) { const h = Math.floor(m / 60), r = m % 60; return h + (h === 1 ? ' hr' : ' hrs') + (r ? ' ' + r + ' min' : ''); } const d = Math.floor(m / 1440), h = Math.floor((m % 1440) / 60); return d + (d === 1 ? ' day' : ' days') + (h ? ' ' + h + (h === 1 ? ' hr' : ' hrs') : ''); };
  const label = (v) => (v.toLowerCase() === 'suv' ? 'SUV' : v.replace(/\b\w/g, (c) => c.toUpperCase()));
  const el = (tag, attrs, ...kids) => { const n = document.createElement(tag); Object.entries(attrs || {}).forEach(([k, v]) => (k === 'class' ? (n.className = v) : k.startsWith('on') ? n.addEventListener(k.slice(2), v) : n.setAttribute(k, v))); kids.flat().forEach((k) => n.append(k)); return n; };

  fetch(api, { cache: 'no-store' }).then((r) => (r.ok ? r.json() : Promise.reject())).then((o) => {
    const price = new Map(o.services.map((s) => [s.id, s.priceCents]));
    const packs = o.rules.map((r) => { const m = VEHICLE.exec(r.serviceName.trim()); return { rule: r, vehicle: m ? label(m[1]) : '', name: m ? m[2].trim() : r.serviceName.trim() }; }).filter((p) => price.has(p.rule.serviceId));
    if (!packs.length) return;
    const vehicles = [...new Set(packs.map((p) => p.vehicle).filter(Boolean))].sort();
    const state = { vehicle: vehicles[0] || '', pack: null, extras: new Set() };
    const out = root.querySelector('[data-quote-body]');
    const taxOf = (sub) => o.taxes.rates.filter((t) => t.active).reduce((s, t) => { const n = sub * t.rateBasisPoints; let q = Math.floor(n / 10000); const r = n % 10000; if (o.taxes.settings.rounding === 'down') q += 0; else if (o.taxes.settings.rounding === 'half_even') q += r > 5000 || (r === 5000 && q % 2) ? 1 : 0; else q += r >= 5000 ? 1 : 0; return s + q; }, 0);

    function draw() {
      const shown = packs.filter((p) => !p.vehicle || p.vehicle === state.vehicle);
      if (state.pack && !shown.includes(state.pack)) { state.pack = null; state.extras.clear(); }
      const extras = state.pack ? state.pack.rule.availableAddons.filter((a) => price.has(a.serviceId)) : [];
      const sub = (state.pack ? price.get(state.pack.rule.serviceId) : 0) + [...state.extras].reduce((s, id) => s + (price.get(id) || 0), 0);
      const tax = taxOf(sub);
      const params = new URLSearchParams();
      out.replaceChildren(
        vehicles.length ? el('div', { class: 'q-block' }, el('h3', {}, '1. What are we caring for?'), el('div', { class: 'q-tiles', role: 'group', 'aria-label': 'Vehicle type' }, vehicles.map((v) => el('button', { type: 'button', class: 'q-tile', 'aria-pressed': String(v === state.vehicle), onclick: () => { state.vehicle = v; draw(); } }, v)))) : '',
        el('div', { class: 'q-block' }, el('h3', {}, vehicles.length ? '2. Choose a package' : 'Choose a package'), el('div', { class: 'q-list' }, shown.map((p) => el('button', { type: 'button', class: 'q-row', 'aria-pressed': String(p === state.pack), onclick: () => { state.pack = p; state.extras.clear(); draw(); } }, el('span', {}, el('strong', {}, p.name), el('small', {}, 'About ' + span(p.rule.durationMinutes))), el('b', {}, money(price.get(p.rule.serviceId))))))),
        extras.length ? el('div', { class: 'q-block' }, el('h3', {}, '3. Add anything extra?'), el('div', { class: 'q-list' }, extras.map((a) => el('label', { class: 'q-row q-check' }, el('input', { type: 'checkbox', ...(state.extras.has(a.serviceId) ? { checked: '' } : {}), onchange: (e) => { e.target.checked ? state.extras.add(a.serviceId) : state.extras.delete(a.serviceId); draw(); } }), el('span', {}, el('strong', {}, a.name)), el('b', {}, '+' + money(price.get(a.serviceId))))))) : '',
        el('div', { class: 'q-total', 'aria-live': 'polite' },
          state.pack ? [el('div', {}, el('span', {}, 'Subtotal'), el('span', {}, money(sub))), el('div', {}, el('span', {}, 'Tax'), el('span', {}, money(tax))), el('div', { class: 'q-grand' }, el('span', {}, 'Estimated total'), el('span', {}, money(sub + tax))),
            el('a', { class: 'pill dark', href: '/booking' }, 'Book this ', el('span', { 'aria-hidden': 'true' }, '↗')),
            el('p', {}, 'An estimate from our current prices. Your final price is confirmed when you book; condition and size can change it.')]
            : el('p', {}, 'Pick a package to see the price.')));
      void params;
    }
    root.hidden = false;
    const link = document.getElementById('quote-link');
    if (link) link.hidden = false;
    draw();
  }).catch(() => { /* leave the section hidden */ });
})();
