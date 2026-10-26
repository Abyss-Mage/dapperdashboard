'use client';

import { useCallback, useEffect, useState } from 'react';
import styles from './owner.module.css';
import HoursResources, { summarize } from './HoursResources';
import FranchiseManager from './FranchiseManager';
import ServicesManager from './ServicesManager';
import TaxSettings from './TaxSettings';

type Org = { id: string; name: string; actions: string[] };
type Scope = { organizationId: string; locationId: string; locationName: string; actions: string[] };
type Section = 'setup' | 'franchises' | 'services' | 'taxes' | 'hours';
const SECTIONS: { id: Section; label: string }[] = [
  { id: 'setup', label: 'Setup checklist' },
  { id: 'franchises', label: 'Franchises' },
  { id: 'services', label: 'Services' },
  { id: 'taxes', label: 'Taxes' },
  { id: 'hours', label: 'Hours & resources' },
];

type Step = { id: string; title: string; done: boolean | null; detail: string; go: Section; action: string };

export default function LocationsSetup({ organization, scope, base, onSaved }: { organization: Org; scope: Scope | undefined; base: string; onSaved: () => Promise<void> }) {
  const [section, setSection] = useState<Section>('setup');
  const [steps, setSteps] = useState<Step[] | null>(null);
  const [revision, setRevision] = useState(0);
  const canResources = !!scope?.actions.includes('scheduling.manage');

  const load = useCallback(async () => {
    if (!scope) return setSteps([]);
    const json = async (url: string) => {
      const r = await fetch(url, { cache: 'no-store' });
      return r.ok ? r.json() : null;
    };
    const [structure, services, resources] = await Promise.all([
      json(`/api/v1/organizations/${organization.id}/structure`),
      scope.actions.includes('catalogue.read') ? json(`${base}/services`) : Promise.resolve(null),
      canResources ? json(`${base}/scheduling/resources`) : Promise.resolve(null),
    ]);
    const site = structure?.franchises?.find((f: { id: string }) => f.id === scope.locationId);
    const activeServices = (services?.items ?? []).filter((s: { archived: boolean }) => !s.archived).length;
    const list = (resources?.items ?? []) as Parameters<typeof summarize>[0][];
    const withHours = list.filter((r) => (r.weeklyHours?.length ?? 0) > 0);
    const through = withHours.map((r) => summarize(r).through).filter(Boolean).sort().pop();
    setSteps([
      { id: 'franchise', title: 'Franchise code and details', done: site ? !!site.code : null, detail: site ? (site.code ? `Code ${site.code}${site.address?.city ? ' · ' + site.address.city : ' · no address added (optional)'}` : 'This franchise has no code yet.') : 'Franchise details are not visible to your role.', go: 'franchises', action: 'Edit franchise' },
      { id: 'services', title: 'Services', done: services ? activeServices > 0 : null, detail: services ? (activeServices ? `${activeServices} active service${activeServices === 1 ? '' : 's'} in this franchise's catalogue.` : 'No services yet. Import them from the central catalogue or add your own.') : 'Catalogue is not visible to your role.', go: 'services', action: 'Manage services' },
      { id: 'resources', title: 'Resources', done: resources ? list.length > 0 : null, detail: resources ? (list.length ? `${list.length} resource${list.length === 1 ? '' : 's'}: ${list.map((r) => r.name).join(', ')}.` : 'No resources yet. A resource is a bay, room or crew that can take bookings.') : 'Scheduling is not visible to your role.', go: 'hours', action: 'Add a resource' },
      { id: 'hours', title: 'Weekly hours', done: resources ? list.length > 0 && withHours.length === list.length : null, detail: resources ? (withHours.length ? `${withHours.length} of ${list.length} resources have weekly hours${through ? ', open through ' + new Date(through).toLocaleDateString('en-CA', { month: 'short', day: 'numeric', year: 'numeric' }) : ''}.` : 'No weekly hours set. Customers cannot book until a resource has opening times.') : 'Scheduling is not visible to your role.', go: 'hours', action: 'Set hours' },
    ]);
  }, [organization.id, scope, base, canResources]);
  useEffect(() => {
    void load();
  }, [load, revision]);

  const bump = () => setRevision((n) => n + 1);
  const doneCount = (steps ?? []).filter((s) => s.done).length;

  return (
    <div className={styles.stack}>
      <div className={styles.segments} role="group" aria-label="Locations and setup sections">
        {SECTIONS.map((s) => (
          <button key={s.id} type="button" className={styles.segment} aria-pressed={section === s.id} onClick={() => setSection(s.id)}>{s.label}</button>
        ))}
      </div>

      {section === 'setup' && (
        <div className={styles.stack}>
          <div>
            <p className={styles.eyebrow}>{scope ? scope.locationName : 'No franchise selected'}</p>
            <h2 className={styles.setupTitle}>{steps ? `${doneCount} of ${steps.length} steps done` : 'Checking setup…'}</h2>
            {steps && steps.length > 0 && (
              <div className={styles.meter} role="progressbar" aria-valuemin={0} aria-valuemax={steps.length} aria-valuenow={doneCount} aria-label="Setup progress">
                <span style={{ width: `${(doneCount / steps.length) * 100}%` }} />
              </div>
            )}
          </div>
          {!steps && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading setup" />}
          {steps?.length === 0 && <p className={styles.status}>Choose a franchise in the sidebar to see its setup.</p>}
          <ol className={styles.steps}>
            {steps?.map((s, i) => (
              <li key={s.id} className={styles.step}>
                <span className={`${styles.stepNo} ${s.done ? styles.stepDone : ''}`} aria-hidden="true">{s.done ? '✓' : i + 1}</span>
                <div className={styles.stepBody}>
                  <strong>{s.title}</strong>
                  <span className={styles.personSub}>{s.detail}</span>
                </div>
                <span className={`${styles.badge} ${s.done ? styles.badge_complete : s.done === null ? '' : styles.badge_incomplete}`}>{s.done ? 'Done' : s.done === null ? 'No access' : 'To do'}</span>
                <button type="button" className="secondary" onClick={() => setSection(s.go)}>{s.action}</button>
              </li>
            ))}
          </ol>
          <p className={styles.footnote}>Services, prices, hours and capacity are your decisions; this checklist only shows what is configured. Nothing is filled in for you.</p>
        </div>
      )}

      {section === 'franchises' && <FranchiseManager organization={organization} selectedId={scope?.locationId} onSaved={async () => { await onSaved(); bump(); }} />}

      {section === 'services' && (
        <div className={styles.stack}>
          {scope?.actions.includes('catalogue.read') && (
            <ServicesManager key={base} mode="franchise" listUrl={`${base}/services`} organizationId={organization.id} canManage={scope.actions.includes('catalogue.manage')} title={`Services at ${scope.locationName}`} intro="What this franchise offers. Switch on central services or add its own." />
          )}
          {organization.actions.includes('catalogue.read') && (
            <ServicesManager key={organization.id} mode="central" listUrl={`/api/v1/organizations/${organization.id}/central-services`} organizationId={organization.id} canManage={organization.actions.includes('catalogue.manage')} title="Central catalogue" intro="Organization-wide services every franchise can switch on." />
          )}
        </div>
      )}

      {section === 'taxes' && (scope ? <TaxSettings key={base} base={base} franchiseName={scope.locationName} canManage={organization.actions.includes('invoices.configure')} /> : <p className={styles.status}>Choose a franchise in the sidebar first.</p>)}

      {section === 'hours' && (scope ? <HoursResources key={base} base={base} canManage={canResources} onChanged={bump} /> : <p className={styles.status}>Choose a franchise in the sidebar first.</p>)}
    </div>
  );
}
