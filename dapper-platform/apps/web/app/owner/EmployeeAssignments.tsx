'use client';
import { useCallback, useEffect, useState } from 'react';

type Assignment = { principalId: string; locationId: string; locationName: string; dealershipId: string; dealershipName: string; timezone: string; version: number };
type Franchise = { locationId: string; locationName: string; code: string };
type Dealership = { locationId: string; dealershipId: string; dealershipName: string };
type Submission = { principalId: string; locationName: string; dealershipName: string; weekStart: string; timezone: string; intervals: { day: number; start: string; end: string }[]; notes: string; version: number; submittedAt: string };
type Data = { assignments: Assignment[]; locations: Franchise[]; dealerships: Dealership[]; submissions: Submission[] };
type Member = { principalId: string; email: string; active: boolean };
const days = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
async function read<T>(r: Response): Promise<T> { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error ?? 'Employee assignment settings are unavailable.'); return b as T; }

/* An employee is assigned to a franchise. A partner dealership is an optional focus and is chosen separately. */
export default function EmployeeAssignments({ organizationId }: { organizationId: string }) {
  const [data, setData] = useState<Data | null>(null), [members, setMembers] = useState<Member[]>([]), [principal, setPrincipal] = useState(''), [site, setSite] = useState(''), [dealer, setDealer] = useState(''), [zone, setZone] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const base = `/api/v1/organizations/${organizationId}`;
  const load = useCallback(async () => { const [next, access] = await Promise.all([fetch(`${base}/employee-worksites`, { cache: 'no-store' }).then(read<Data>), fetch(`${base}/access`, { cache: 'no-store' }).then(read<{ members: Member[] }>)]); setData(next); setMembers(access.members); }, [base]);
  useEffect(() => { let live = true; setData(null); setMembers([]); setPrincipal(''); setSite(''); setDealer(''); setZone(''); setError(''); load().catch(e => { if (live) setError(e.message); }); return () => { live = false; }; }, [load]);
  const existing = data?.assignments.find(a => a.principalId === principal);
  const siteDealerships = data?.dealerships.filter(d => d.locationId === site) ?? [];
  const label = (f: Franchise) => (f.code ? `${f.code} · ${f.locationName}` : f.locationName);
  async function save(e: React.FormEvent) {
    e.preventDefault(); if (!site || !principal) return; setBusy(true); setError(''); setNotice('');
    try { await read(await fetch(`${base}/employee-worksites/${principal}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ locationId: site, dealershipId: dealer, timezone: zone, version: existing?.version ?? 0 }) })); await load(); setNotice('Franchise assignment saved. No operational permissions were added.'); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  return <section className="panel detail-card"><h2>Employee franchise &amp; dealership</h2><p>Employees work at the franchise you assign. A partner dealership is optional and is only a focus: it never replaces the franchise. Assigning a franchise does not grant customer, finance or attendance permissions.</p>
    {error && <p role="alert" className="field-error">{error}</p>}{notice && <p role="status">{notice}</p>}
    {!data && !error && <p role="status">Loading employee assignments…</p>}
    {data && <><form className="management-form" onSubmit={save}>
      <label className="form-field">Employee<select required disabled={busy} value={principal} onChange={e => { const id = e.target.value; setPrincipal(id); const a = data.assignments.find(a => a.principalId === id); setSite(a?.locationId ?? ''); setDealer(a?.dealershipId ?? ''); setZone(a?.timezone ?? ''); }}><option value="">Choose employee</option>{members.filter(m => m.active).map(m => <option value={m.principalId} key={m.principalId}>{m.email || 'Enrolled member'}</option>)}</select></label>
      <label className="form-field">Franchise<select required disabled={busy} value={site} onChange={e => { setSite(e.target.value); setDealer(''); }}><option value="">Choose franchise</option>{data.locations.map(f => <option key={f.locationId} value={f.locationId}>{label(f)}</option>)}</select></label>
      <label className="form-field">Partner dealership (optional)<select disabled={busy || !site} value={dealer} onChange={e => setDealer(e.target.value)}><option value="">No dealership</option>{siteDealerships.map(d => <option key={d.dealershipId} value={d.dealershipId}>{d.dealershipName}</option>)}</select></label>
      <label className="form-field">Timezone<input required disabled={busy} maxLength={100} value={zone} onChange={e => setZone(e.target.value)} placeholder="IANA timezone, e.g. America/Toronto" /></label>
      <button className="primary" disabled={busy || !principal || !site}>{busy ? 'Saving…' : existing ? 'Update assignment' : 'Assign employee'}</button>
    </form>{!data.locations.length && <p>Create a franchise first (Administration → Franchises).</p>}
    <h3>Current assignments</h3>{data.assignments.length ? <ul>{data.assignments.map(a => <li key={a.principalId}>{members.find(m => m.principalId === a.principalId)?.email || 'Organization member'} — {a.locationName}{a.dealershipName ? ` · dealership focus: ${a.dealershipName}` : ''} · {a.timezone}</li>)}</ul> : <p>No employees assigned to a franchise.</p>}
    <p>Staff availability is approved or rejected on its own page: Staff availability.</p></>}
  </section>;
}
