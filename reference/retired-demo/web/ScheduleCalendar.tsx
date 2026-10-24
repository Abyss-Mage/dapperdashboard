'use client';
import {useEffect,useRef,useState} from 'react';
import {ChevronLeft,ChevronRight,X} from 'lucide-react';

export type Appointment = {
  id: string;
  customer: string;
  vehicle: string;
  service: string;
  time: string;
  status: string;
  employee: string;
  date?: string;
  reservationId?: string | null;
  durationMinutes?: number;
};

const key = (date: Date) =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;

export default function ScheduleCalendar({appointments}: {appointments: Appointment[]}) {
  const [today] = useState(() => new Date());
  const [month, setMonth] = useState(() => new Date(new Date().getFullYear(), new Date().getMonth(), 1));
  const [selected, setSelected] = useState<string | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    if (selected) dialog.current?.showModal();
    else dialog.current?.close();
  }, [selected]);

  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  const last = new Date(month.getFullYear(), month.getMonth() + 1, 0);
  const count = Math.ceil((first.getDay() + last.getDate()) / 7) * 7;
  const dated = appointments.map((a) => ({...a, date: a.date ?? key(today)}));
  const selectedItems = dated.filter((a) => a.date === selected);

  return (
    <section className="panel month-calendar">
      <div className="section-head">
        <div>
          <h2>{month.toLocaleDateString('en-CA', {month: 'long', year: 'numeric'})}</h2>
          <p>Monthly calendar · select a date to see its agenda and capacity reservations</p>
        </div>
        <div className="month-actions">
          <button
            className="icon-button"
            aria-label="Previous month"
            onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}
          >
            <ChevronLeft size={18} />
          </button>
          <button className="secondary" onClick={() => setMonth(new Date(today.getFullYear(), today.getMonth(), 1))}>
            Today
          </button>
          <button
            className="icon-button"
            aria-label="Next month"
            onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}
          >
            <ChevronRight size={18} />
          </button>
        </div>
      </div>
      <p className="schedule-note">
        Scheduled bookings and work orders bound to capacity reservations appear on their confirmed date. Synthetic
        appointments without explicit dates project onto today’s demo date.
      </p>
      <div className="month-grid">
        {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map((day) => (
          <div className="month-weekday" key={day}>
            {day}
          </div>
        ))}
        {Array.from({length: count}, (_, i) => {
          const day = i - first.getDay() + 1;
          if (day < 1 || day > last.getDate()) return <div className="month-blank" key={i} />;
          const date = key(new Date(month.getFullYear(), month.getMonth(), day));
          const items = dated.filter((a) => a.date === date);
          return (
            <button
              key={i}
              className={'month-day' + (date === key(today) ? ' is-today' : '')}
              aria-label={`${new Date(month.getFullYear(), month.getMonth(), day).toLocaleDateString('en-CA', {
                dateStyle: 'full',
              })}, ${items.length} appointments`}
              onClick={() => setSelected(date)}
            >
              <strong>{day}</strong>
              {items.length > 0 && (
                <>
                  <span className="month-count">
                    {items.length} <span>appointments</span>
                  </span>
                  <span className="month-preview">
                    {items[0].time} · {items[0].service}
                  </span>
                </>
              )}
            </button>
          );
        })}
      </div>
      <dialog
        className="day-dialog"
        ref={dialog}
        onCancel={() => setSelected(null)}
        onClose={() => setSelected(null)}
        onClick={(e) => {
          if (e.target === e.currentTarget) setSelected(null);
        }}
      >
        <div className="section-head">
          <div>
            <p className="eyebrow">DAY AGENDA</p>
            <h2>
              {selected &&
                new Date(selected + 'T12:00:00').toLocaleDateString('en-CA', {dateStyle: 'full'})}
            </h2>
          </div>
          <button
            autoFocus
            className="icon-button"
            aria-label="Close day agenda"
            onClick={() => setSelected(null)}
          >
            <X size={20} />
          </button>
        </div>
        <div className="day-agenda">
          {selectedItems.length ? (
            selectedItems.map((a) => (
              <article className="day-appointment" key={a.id}>
                <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                  <span className="status scheduled">{a.time}</span>
                  {a.reservationId && (
                    <span className="status progress" style={{fontSize: '0.75rem'}}>
                      Capacity Bay Slot
                    </span>
                  )}
                </div>
                <h3>{a.service}</h3>
                <p>
                  {a.customer} · {a.vehicle}
                </p>
                <p>
                  {a.id} · {a.status} · Open team queue
                  {a.reservationId && ` · Res #${a.reservationId.slice(0, 8)}`}
                  {a.durationMinutes ? ` · ${a.durationMinutes} min` : ''}
                </p>
              </article>
            ))
          ) : (
            <p>No appointments for this date.</p>
          )}
        </div>
      </dialog>
    </section>
  );
}
