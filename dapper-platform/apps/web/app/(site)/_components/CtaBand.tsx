import { ArrowRight } from 'lucide-react';
import { contact, cta } from '../dapper';
import './cta.css';

/* The closing call on every page: a pure white band, the question in two lines (the second in red), the booking button and the number, all centred. */
export function CtaBand({ id, lead = 'Ready to give your car', tail = 'the care it deserves?' }: { id: string; lead?: string; tail?: string }) {
  return (
    <section className="cta" aria-labelledby={id}>
      <h2 id={id} className="cta__h">{lead} <em>{tail}</em></h2>
      <p className="cta__lede">{cta.text}</p>
      <div className="cta__act">
        <a className="cta__btn" href="/booking"><span>Book now</span><ArrowRight size={20} aria-hidden="true" /></a>
        <a className="cta__call" href={'tel:' + contact.tel}>or call <span>+1 {contact.phone}</span></a>
      </div>
    </section>
  );
}
