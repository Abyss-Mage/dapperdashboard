'use client';
import {useCallback,useEffect,useState} from 'react';
import Select from '../GlassSelect';

type Customer = {id: string; name: string};
type Service = {id: string; name: string; kind: string; archived: boolean; parentId: string | null};
type Reservation = {id: string; start: string; durationMinutes: number; cancelledAt: string | null};

export default function WorkOrderForm({base, onSaved, customerRevision = 0}: {base: string; onSaved: () => Promise<void>; customerRevision?: number}) {
  const [addonIds, setAddonIds] = useState<string[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [reservations, setReservations] = useState<Reservation[]>([]);
  const [customerId, setCustomerId] = useState('');
  const [serviceId, setServiceId] = useState('');
  const [reservationId, setReservationId] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [notice, setNotice] = useState('');

  const loadChoices = useCallback(async () => {
    setLoaded(false);
    try {
      const now = new Date();
      const past = new Date(now.getTime() - 24 * 3600 * 1000).toISOString();
      const future = new Date(now.getTime() + 30 * 24 * 3600 * 1000).toISOString();

      const [customerResponse, serviceResponse, reservationResponse] = await Promise.all([
        fetch(base + '/customers'),
        fetch(base + '/services'),
        fetch(`${base}/scheduling/reservations?from=${encodeURIComponent(past)}&to=${encodeURIComponent(future)}`).catch(() => null),
      ]);

      if (!customerResponse.ok || !serviceResponse.ok) throw new Error();
      const [c, s] = await Promise.all([customerResponse.json(), serviceResponse.json()]);
      setCustomers(c.items);
      setServices(s.items.filter((service: Service) => !service.archived));

      if (reservationResponse && reservationResponse.ok) {
        const resData = await reservationResponse.json();
        setReservations(resData.items ?? []);
      } else {
        setReservations([]);
      }

      setLoaded(true);
      setError('');
    } catch {
      setError('Could not load permitted customers and services. Retry before creating a work order.');
    }
  }, [base]);

  useEffect(() => {
    loadChoices();
  }, [loadChoices, customerRevision]);

  async function save(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    setError('');
    setNotice('');
    setFields({});
    const selectedService = services.find((service) => service.id === serviceId);
    if (!selectedService || !customers.some((c) => c.id === customerId)) {
      setError('Choose a customer and main service from this location.');
      return;
    }
    setBusy(true);
    try {
      const response = await fetch(base + '/jobs', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          customerId,
          serviceId: selectedService.id,
          description,
          addonIds,
          reservationId: reservationId ? reservationId : null,
        }),
      });
      const body = await response.json();
      if (!response.ok) {
        setFields(body.errors ?? {});
        setError(body.error ?? 'Check the highlighted work-order fields.');
        return;
      }
      setDescription('');
      setAddonIds([]);
      setCustomerId('');
      setServiceId('');
      setReservationId('');
      setNotice(
        reservationId
          ? 'Internal work order created and bound to scheduled capacity.'
          : 'Internal work order created in the available-work pool.'
      );
      await onSaved();
    } catch {
      setError('Could not reach the workspace. Input is preserved; refresh the pool before retrying.');
    } finally {
      setBusy(false);
      queueMicrotask(() => form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus());
    }
  }

  return (
    <section className="panel detail-card">
      <h2>Create an internal work order</h2>
      <p>
        This creates available work for the authorized team. Optionally link an active capacity reservation to bind the
        work order to scheduled calendar time. The selected service name is saved as a work-order snapshot.
      </p>
      {error && <p className="field-error" role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      {!loaded ? (
        <button className="secondary" onClick={loadChoices}>
          Reload customers and services
        </button>
      ) : (
        <form onSubmit={save}>
          <button type="button" className="secondary" disabled={busy} onClick={loadChoices}>
            Refresh customers, services and reservations
          </button>
          <div className="management-form">
            <label className="form-field">
              Customer
              <Select
                required
                disabled={busy}
                value={customerId}
                onChange={(e) => setCustomerId(e.target.value)}
                aria-invalid={!!fields.customerId}
                aria-describedby={fields.customerId ? 'work-customer-error' : undefined}
              >
                <option value="">Choose customer</option>
                {customers.map((customer) => (
                  <option key={customer.id} value={customer.id}>
                    {customer.name}
                  </option>
                ))}
              </Select>
              {fields.customerId && (
                <small className="field-error" id="work-customer-error">
                  {fields.customerId}
                </small>
              )}
            </label>
            <label className="form-field">
              Main service
              <Select
                required
                disabled={busy}
                value={serviceId}
                onChange={(e) => {
                  setServiceId(e.target.value);
                  setAddonIds([]);
                }}
                aria-invalid={!!fields.serviceId}
                aria-describedby={fields.serviceId ? 'work-service-error' : undefined}
              >
                <option value="">Choose main service</option>
                {services
                  .filter((service) => service.kind === 'main')
                  .map((service) => (
                    <option key={service.id} value={service.id}>
                      {service.name}
                    </option>
                  ))}
              </Select>
              {fields.serviceId && (
                <small className="field-error" id="work-service-error">
                  {fields.serviceId}
                </small>
              )}
            </label>
          </div>
          {reservations.length > 0 && (
            <label className="form-field">
              Link capacity reservation (optional)
              <Select
                disabled={busy}
                value={reservationId}
                onChange={(e) => setReservationId(e.target.value)}
                aria-invalid={!!fields.reservationId}
                aria-describedby={fields.reservationId ? 'work-reservation-error' : undefined}
              >
                <option value="">None (unassigned to calendar slot)</option>
                {reservations
                  .filter((r) => !r.cancelledAt)
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {new Date(r.start).toLocaleString('en-CA')} ({r.durationMinutes} min) · {r.id.slice(0, 8)}
                    </option>
                  ))}
              </Select>
              {fields.reservationId && (
                <small className="field-error" id="work-reservation-error">
                  {fields.reservationId}
                </small>
              )}
            </label>
          )}
          <fieldset disabled={busy}>
            <legend>Add-on services (optional)</legend>
            <p>Each selected add-on is included once in this work order. Prices and booking capacity are not calculated.</p>
            {services
              .filter((service) => service.kind === 'addon' && service.parentId === serviceId)
              .map((service) => (
                <label className="form-field" key={service.id}>
                  <span>
                    <input
                      type="checkbox"
                      checked={addonIds.includes(service.id)}
                      onChange={(event) =>
                        setAddonIds((current) =>
                          event.target.checked ? [...current, service.id] : current.filter((id) => id !== service.id)
                        )
                      }
                    />
                    {service.name}
                  </span>
                </label>
              ))}
            {!serviceId && <p>Choose a main service to see its add-ons.</p>}
            {serviceId &&
              !services.some((service) => service.kind === 'addon' && service.parentId === serviceId) && (
                <p>No active add-ons linked to this main service.</p>
              )}
            {fields.addonIds && (
              <p role="alert" className="field-error">
                {fields.addonIds}
              </p>
            )}
          </fieldset>
          <label className="form-field">
            Work notes (optional)
            <textarea
              disabled={busy}
              maxLength={2000}
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              aria-invalid={!!fields.description}
              aria-describedby={fields.description ? 'work-description-error' : undefined}
            />
            {fields.description && (
              <small className="field-error" id="work-description-error">
                {fields.description}
              </small>
            )}
          </label>
          <button className="primary" disabled={busy || !customers.length || !services.length}>
            {busy ? 'Creating…' : 'Create work order'}
          </button>
          {(!customers.length || !services.length) && (
            <p>Add a permitted customer and an active main service before creating work.</p>
          )}
        </form>
      )}
    </section>
  );
}
