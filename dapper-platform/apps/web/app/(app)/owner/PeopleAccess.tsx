'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './owner.module.css';
import { OwnerIcon } from './OwnerIcons';
import RoleManager from './RoleManager';
import RoleAssignments from './RoleAssignments';
import MemberInvitations from '../workspace/MemberInvitations';
import EmployeeAssignments from './EmployeeAssignments';

type Person = {
  principalId: string;
  email: string;
  active: boolean;
  fullName: string;
  phone: string;
  profileComplete: boolean;
  administrator: boolean;
  mfaEnabled: boolean;
  mfaSetupStarted: boolean;
  locationName: string;
  locationCode: string;
  dealershipName: string;
  roles: string[];
  scope: string;
  documentsTotal: number;
  documentsAcknowledged: number;
  onboardingStatus: 'complete' | 'incomplete' | 'not_assigned' | 'inactive';
  missing: string[];
};

type Section = 'roster' | 'invitations' | 'roles' | 'assign' | 'assignments';
const SECTIONS: { id: Section; label: string }[] = [
  { id: 'roster', label: 'People' },
  { id: 'invitations', label: 'Invitations' },
  { id: 'roles', label: 'Roles' },
  { id: 'assign', label: 'Assign roles' },
  { id: 'assignments', label: 'Franchise' },
];

const STATUS_LABEL: Record<Person['onboardingStatus'], string> = {
  complete: 'Onboarded',
  incomplete: 'Onboarding incomplete',
  not_assigned: 'No franchise yet',
  inactive: 'Inactive',
};
const MISSING_LABEL: Record<string, string> = {
  details: 'name and phone',
  mfa: 'authenticator app',
  documents: 'documents',
  franchise: 'franchise assignment',
};

function Roster({ organizationId, revision }: { organizationId: string; revision: number }) {
  const [people, setPeople] = useState<Person[] | null>(null);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<'all' | 'attention'>('all');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await fetch(`/api/v1/organizations/${organizationId}/access/people`, { cache: 'no-store' });
      if (!response.ok) throw new Error(response.status === 403 ? 'You do not have access to the people list.' : 'The people list could not be loaded. Please retry.');
      setPeople((await response.json()).people ?? []);
    } catch (e) {
      setPeople(null);
      setError(e instanceof Error ? e.message : 'The people list could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [organizationId]);
  useEffect(() => {
    void load();
  }, [load, revision]);

  const shown = useMemo(() => (people ?? []).filter((p) => filter === 'all' || p.onboardingStatus === 'incomplete' || p.onboardingStatus === 'not_assigned'), [people, filter]);
  const attention = (people ?? []).filter((p) => p.onboardingStatus === 'incomplete' || p.onboardingStatus === 'not_assigned').length;

  return (
    <div className={styles.stack}>
      <div className={styles.rowBetween}>
        <div className={styles.segments} role="group" aria-label="Filter people">
          <button type="button" className={styles.segment} aria-pressed={filter === 'all'} onClick={() => setFilter('all')}>
            Everyone{people ? ` · ${people.length}` : ''}
          </button>
          <button type="button" className={styles.segment} aria-pressed={filter === 'attention'} onClick={() => setFilter('attention')}>
            Needs attention{people ? ` · ${attention}` : ''}
          </button>
        </div>
        <button type="button" className="secondary" disabled={loading} onClick={() => void load()}>
          <OwnerIcon name="refresh" size={16} /> {loading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      {error && <p role="alert" className="field-error">{error}</p>}
      {loading && !people && <div className={`${styles.card} ${styles.skeleton}`} role="status" aria-label="Loading people" />}

      {people && !shown.length && (
        <p className={styles.status}>{filter === 'attention' ? 'Nobody needs attention right now.' : 'No members yet. Send an invitation to add the first employee.'}</p>
      )}

      {shown.length > 0 && (
        <ul className={styles.people}>
          {shown.map((p) => (
            <li key={p.principalId} className={styles.person}>
              <div className={styles.personMain}>
                <strong className={styles.personName}>{p.fullName || p.email || 'Account awaiting enrollment'}</strong>
                {p.fullName && p.email && <span className={styles.personSub}>{p.email}</span>}
                {p.phone && <span className={styles.personSub}>{p.phone}</span>}
              </div>
              <div className={styles.personWork}>
                <span>{p.locationName ? `${p.locationCode ? p.locationCode + ' · ' : ''}${p.locationName}` : p.scope || 'No franchise'}</span>
                {p.dealershipName && <span className={styles.personSub}>Dealership focus: {p.dealershipName}</span>}
                <span className={styles.personSub}>{p.roles.length ? p.roles.join(', ') : 'No role assigned'}</span>
              </div>
              <div className={styles.personState}>
                <span className={`${styles.badge} ${styles['badge_' + p.onboardingStatus]}`}>{STATUS_LABEL[p.onboardingStatus]}</span>
                <span className={styles.personSub}>
                  Details {p.profileComplete ? 'done' : 'missing'} · Authenticator {p.mfaEnabled ? 'on' : p.mfaSetupStarted ? 'in setup' : p.administrator ? 'required' : 'off'} · SOP & Compliance {p.documentsAcknowledged}/{p.documentsTotal}
                </span>
                {p.missing.length > 0 && <span className={styles.personSub}>Waiting on: {p.missing.map((m) => MISSING_LABEL[m] ?? m).join(', ')}</span>}
                {p.administrator && <span className={styles.chip}>Administrator</span>}
              </div>
            </li>
          ))}
        </ul>
      )}
      <p className={styles.footnote}>
        Shows state only. Passwords, authenticator secrets and recovery codes are never visible to anyone. An administrator who loses their
        device is reset by the machine owner, not from this screen.
      </p>
    </div>
  );
}

export default function PeopleAccess({ organizationId, revision, onSaved }: { organizationId: string; revision: number; onSaved: () => Promise<void> }) {
  const [section, setSection] = useState<Section>('roster');
  const [bump, setBump] = useState(0);
  return (
    <div className={styles.stack}>
      <div className={styles.segments} role="group" aria-label="People and access sections">
        {SECTIONS.map((s) => (
          <button key={s.id} type="button" aria-pressed={section === s.id} className={styles.segment} onClick={() => setSection(s.id)}>
            {s.label}
          </button>
        ))}
      </div>
      <div className={styles.stack}>
        {section === 'roster' && <Roster organizationId={organizationId} revision={revision + bump} />}
        {section === 'invitations' && (
          <section className="panel detail-card">
            <MemberInvitations key={organizationId} organizationId={organizationId} />
          </section>
        )}
        {section === 'roles' && <RoleManager organizationId={organizationId} onChanged={() => setBump((b) => b + 1)} />}
        {section === 'assign' && (
          <RoleAssignments
            organizationId={organizationId}
            revision={revision + bump}
            onSaved={async () => {
              setBump((b) => b + 1);
              await onSaved();
            }}
          />
        )}
        {section === 'assignments' && <EmployeeAssignments organizationId={organizationId} />}
      </div>
    </div>
  );
}
