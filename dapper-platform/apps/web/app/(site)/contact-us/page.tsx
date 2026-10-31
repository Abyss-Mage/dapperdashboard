import { ArrowRight } from 'lucide-react';
import { studio } from '../content';
import { contact } from '../dapper';
import { CallToBook, PageHero, PageMotion, SectionHead } from '../_v3/blocks';

export const metadata = { title: 'Contact Us', description: 'Contact Dapper Detailing Pros at 660 The Kingsway, Peterborough.', alternates: { canonical: '/contact-us' } };

export default function Contact() {
  return (
    <PageMotion>
      <PageHero chip="Contact us" title={<>Let’s talk about <em>your vehicle.</em></>} lead="Ready to book, exploring a package, or looking for a custom quote? The Dapper team can help you choose your next step." image="/site/assets/portfolio/portfolio-06.jpg" alt="A white Porsche 911 at the Dapper studio" />
      <section className="d-section">
        <SectionHead chip="Reach the studio" title="We’ll come to you — no waiting rooms" />
        <div className="d-contact">
          <article data-d-in><h2>Call the studio</h2><a className="big" href={'tel:' + contact.tel}>{contact.phone}</a><p>Talk to the team about your vehicle.</p></article>
          <article data-d-in><h2>Email the team</h2><a className="big" href={'mailto:' + contact.email}>{contact.email}</a><p>We’ll reply with your options.</p></article>
          <article data-d-in><h2>Visit us</h2><p className="big">{contact.street}</p><p>{contact.city}, {contact.country}</p><a className="d-link" href={contact.directions} target="_blank" rel="noopener noreferrer">Get directions<ArrowRight size={16} /></a></article>
          <article data-d-in><h2>Opening hours</h2><p className="big">{studio.hours}</p><p>Contact us to confirm holiday hours and appointment availability.</p></article>
        </div>
      </section>
      <CallToBook title="Ready for precise detailing?" />
    </PageMotion>
  );
}
