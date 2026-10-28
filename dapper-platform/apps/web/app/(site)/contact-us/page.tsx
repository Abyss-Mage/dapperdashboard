import { studio } from '../content';
import SplitHero from '../_components/SplitHero';
import Reveal from '../_components/Reveal';
import { Close } from '../_components/Chrome';
import { ArrowUpRight } from '../_components/Icons';

export const metadata = { title: 'Contact Us', description: 'Contact Dapper Detailing Pros at 660 The Kingsway, Peterborough.', alternates: { canonical: '/contact-us' } };

export default function Contact() {
  return (
    <>
      <SplitHero image="/site/assets/classic.webp" alt="A classic car inside the Dapper studio" before="Let’s talk" after="about your vehicle." lead="Ready to book, exploring a package, or looking for a custom quote? The Dapper team can help you choose your next step.">
        <a className="btn btn--tape num" href={'tel:' + studio.tel}>Call {studio.phone}</a>
      </SplitHero>
      <Reveal className="section" aria-labelledby="reach-title">
        <div className="wrap">
          <h2 id="reach-title" className="sr">Ways to reach the studio</h2>
          <div className="contact-grid">
            <div data-rise><h2>Call the studio</h2><a className="big num" href={'tel:' + studio.tel}>{studio.phone}</a></div>
            <div data-rise><h2>Email the team</h2><a className="big" href={'mailto:' + studio.email} style={{ textTransform: 'none' }}>{studio.email}</a></div>
            <div data-rise>
              <h2>Visit us</h2>
              <p className="big">{studio.street}</p>
              <p>{studio.city}, Canada</p>
              <p style={{ marginTop: 20 }}><a className="btn btn--line" href={studio.maps} target="_blank" rel="noopener noreferrer">Get directions<ArrowUpRight size={20} aria-hidden /></a></p>
            </div>
            <div data-rise>
              <h2>Opening hours</h2>
              <p className="big num">8 AM–9 PM</p>
              <p>Seven days a week. Contact us to confirm holiday hours and appointment availability.</p>
            </div>
          </div>
        </div>
      </Reveal>
      <Close before="We look forward" after="to welcoming you." />
    </>
  );
}
