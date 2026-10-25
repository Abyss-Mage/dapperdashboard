'use client';
import { useCallback, useEffect, useState } from 'react';
import ConfirmDialog from './ConfirmDialog';
import Select from '../GlassSelect';

type Service = {
  id: string;
  name: string;
  kind: string;
  parentId: string | null;
  version: number;
  archived: boolean;
  centralServiceId?: string | null;
};

export default function ServiceEditor({
  base,
  canManage,
}: {
  base: string;
  canManage: boolean;
}) {
  const [items, setItems] = useState<Service[]>([]);
  const [editing, setEditing] = useState<Service | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<{
    action: 'delete' | 'archive';
    item: Service;
  } | null>(null);
  const [name, setName] = useState('');
  const [kind, setKind] = useState('main');
  const [parentId, setParentId] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [central, setCentral] = useState<Service[]>([]);
  const [centralId, setCentralId] = useState('');
  const [centralError, setCentralError] = useState('');

  useEffect(() => {
    const controller = new AbortController();
    setCentral([]);
    setCentralId('');
    setCentralError('');
    if (!canManage) return;
    const organizationBase = base.split('/locations/')[0];
    fetch(organizationBase + '/central-services', {
      cache: 'no-store',
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok)
          throw Error(
            'Organization catalogue.read is needed to import central services.'
          );
        return response.json();
      })
      .then((body) => {
        if (!controller.signal.aborted)
          setCentral(body.items.filter((item: Service) => !item.archived));
      })
      .catch((e) => {
        if (!controller.signal.aborted) setCentralError(e.message);
      });
    return () => controller.abort();
  }, [base, canManage]);

  async function importCentral(e: React.FormEvent) {
    e.preventDefault();
    const selected = central.find((item) => item.id === centralId);
    if (!selected) return;
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const response = await fetch(base + '/services/from-central', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          centralServiceId: selected.id,
          version: selected.version,
        }),
      });
      const body = await response.json();
      if (!response.ok)
        throw Error(
          body.error ??
            'The central service could not be enabled. Import its main service first if it is an add-on.'
        );
      setCentralId('');
      setNotice(`Central service "${selected.name}" imported.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not enable service');
    } finally {
      setBusy(false);
    }
  }

  const load = useCallback(async () => {
    const r = await fetch(base + '/services');
    if (!r.ok) throw Error('Could not load this location’s services');
    setItems((await r.json()).items);
  }, [base]);

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, [load]);

  function clear() {
    setEditing(null);
    setName('');
    setKind('main');
    setParentId('');
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(base + '/services' + (editing ? '/' + editing.id : ''), {
        method: editing ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          kind,
          parentId: kind === 'addon' ? parentId : null,
          ...(editing ? { version: editing.version } : {}),
        }),
      });
      if (!r.ok) {
        const body = await r.json();
        setError(
          body.error ??
            Object.values(body.errors ?? {}).join('. ') ??
            'Could not save service'
        );
        return;
      }
      setNotice(`Service "${name}" saved.`);
      clear();
      await load();
    } catch {
      setError('Could not reach the workspace. Input is preserved.');
    } finally {
      setBusy(false);
    }
  }

  async function archive(item: Service) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(base + '/services/' + item.id + '/archive', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version: item.version }),
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not archive service');
        return;
      }
      setNotice(`Service "${item.name}" archived.`);
      if (editing?.id === item.id) clear();
      await load();
    } catch {
      setError('Could not archive service. Refresh before retrying.');
    } finally {
      setBusy(false);
    }
  }

  async function deleteService(item: Service) {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await fetch(base + '/services/' + item.id + '?version=' + item.version, {
        method: 'DELETE',
      });
      if (!r.ok) {
        const body = await r.json();
        setError(body.error ?? 'Could not delete service');
        return;
      }
      const data = await r.json();
      setNotice(data.message ?? `Service "${item.name}" removed.`);
      if (editing?.id === item.id) clear();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not delete service.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel detail-card">
      <h2>Services and add-ons</h2>
      <p>
        <a href="/content">Manage service publication &amp; portfolio images ↗</a>
      </p>
      <p>
        Managed services for this exact location. Changes preserve revisions; archive retains history. Deleting unused services removes them permanently; deleting referenced services removes them from the active catalogue while retaining historical records.
      </p>
      {error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}
      {canManage && (
        <>
          <p>
            Linked central services receive central name/archive revisions. Their historical work orders and invoices retain their recorded snapshots.
          </p>
          {centralError && <p>{centralError}</p>}
          {central.length > 0 && (
            <form className="management-form" onSubmit={importCentral}>
              <label className="form-field">
                Use a central service at this location
                <Select
                  required
                  value={centralId}
                  onChange={(e) => setCentralId(e.target.value)}
                >
                  <option value="">Choose a central service</option>
                  {central.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} · {item.kind}
                    </option>
                  ))}
                </Select>
              </label>
              <button className="primary" disabled={busy}>
                Enable central service
              </button>
            </form>
          )}
        </>
      )}
      {canManage && (
        <form className="management-form" onSubmit={save}>
          <label className="form-field">
            Service name
            <input
              required
              minLength={2}
              maxLength={120}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className="form-field">
            Type
            <Select
              value={kind}
              onChange={(e) => {
                setKind(e.target.value);
                setParentId('');
              }}
            >
              <option value="main">Main service</option>
              <option value="addon">Add-on</option>
            </Select>
          </label>
          {kind === 'addon' && (
            <label className="form-field">
              Main service
              <Select
                required
                value={parentId}
                onChange={(e) => setParentId(e.target.value)}
              >
                <option value="">Choose main service</option>
                {items
                  .filter(
                    (s) =>
                      !s.archived && s.kind === 'main' && s.id !== editing?.id
                  )
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
              </Select>
            </label>
          )}
          <button className="primary" disabled={busy}>
            {editing ? 'Save revision' : 'Add service'}
          </button>
          {editing && (
            <button type="button" className="secondary" onClick={clear}>
              Cancel edit
            </button>
          )}
        </form>
      )}
      <div className="management-list">
        {items.map((item) => (
          <article key={item.id}>
            <div>
              <strong>{item.name}</strong>
              <small>
                {item.kind === 'addon'
                  ? `Add-on to ${
                      items.find((s) => s.id === item.parentId)?.name ??
                      'main service'
                    }`
                  : 'Main service'}{' '}
                · revision {item.version}
                {item.centralServiceId ? ' · Linked central service' : ''}
                {item.archived ? ' · Archived' : ''}
              </small>
            </div>
            {canManage && !item.archived && (
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                {!item.centralServiceId && (
                  <button
                    className="secondary"
                    disabled={busy}
                    onClick={() => {
                      setEditing(item);
                      setName(item.name);
                      setKind(item.kind);
                      setParentId(item.parentId ?? '');
                    }}
                  >
                    Edit
                  </button>
                )}
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setConfirmTarget({ action: 'archive', item })}
                >
                  Archive
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setConfirmTarget({ action: 'delete', item })}
                >
                  Delete
                </button>
              </div>
            )}
          </article>
        ))}
      </div>

      <ConfirmDialog
        isOpen={confirmTarget !== null}
        title={confirmTarget?.action === 'delete' ? 'Delete Service' : 'Archive Service'}
        entityName={confirmTarget?.item.name ?? ''}
        entityType="Service"
        actionType={confirmTarget?.action ?? 'delete'}
        explanation={
          confirmTarget?.action === 'delete'
            ? 'Deleting an unused service removes it permanently. If the service has existing bookings, invoices, or work orders, it will be removed from the active catalogue while permanently preserving all historical records.'
            : 'Archiving removes this service from active customer booking and scheduling while preserving all completed jobs, past bookings, and audit records.'
        }
        impactNotice={
          confirmTarget?.item.kind === 'main'
            ? 'Add-on services attached to this service will also be affected.'
            : undefined
        }
        isBusy={busy}
        onConfirm={async () => {
          if (!confirmTarget) return;
          const target = confirmTarget;
          setConfirmTarget(null);
          if (target.action === 'delete') {
            await deleteService(target.item);
          } else {
            await archive(target.item);
          }
        }}
        onCancel={() => setConfirmTarget(null)}
      />
    </section>
  );
}
