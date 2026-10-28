import Link from 'next/link';
import { categories } from './content';
import Story from './_components/Story';
import Quote from './_components/Quote';
import Gallery from './_components/Gallery';
import Reveal from './_components/Reveal';
import { Close } from './_components/Chrome';
import { ArrowUpRight, Plus } from './_components/Icons';

export const metadata = { title: { absolute: 'Dapper Detailing Pros | Vehicle care in Peterborough' }, alternates: { canonical: '/' } };

const rack = ['sedan', 'suv', 'pickup', 'van', 'hatchback', '2-wheeler', 'boat', 'aircraft', 'ceramic', 'ppf'].map((k) => categories.find((c) => c.key === k)!);

const faqs = [
  { q: 'Are the before views real results?', a: 'No. The before side of each split is an illustration made from the same photograph, and the home story is a visualisation of the process. Actual portfolio photographs appear in the portfolio.' },
  { q: 'Does ceramic coating make a car waterproof?', a: 'The water test shows water beading on a water-repellent surface. It does not mean waterproof paint, scratch-proofing or a particular product warranty. Ask Dapper about coating and maintenance options.' },
  { q: 'How do I arrange an appointment?', a: 'Sign in to your customer account to choose a vehicle, see published offers and request an available time. A request is not a confirmed appointment. Contact the team if you are new to Dapper or need help choosing a service.' },
];

export default function Home() {
  return (
    <>
      <Story />

      <Reveal className="section" aria-labelledby="rack-title">
        <div className="wrap">
          <div className="head-row">
            <h2 id="rack-title" className="headline" data-split>What do<br /><span className="after">you drive?</span></h2>
            <p className="lede" data-rise>Half of every panel is shown as it arrives, half as it leaves. Pick yours to see the care we plan for it.</p>
          </div>
          <div className="rack">
            {rack.map((c) => (
              c.photo ? (
                <Link key={c.slug} href={'/' + c.slug} className="rack__item" data-rise>
                  <img src={c.image} alt="" loading="lazy" />
                  <span className="rack__before" aria-hidden="true"><img src={c.image} alt="" loading="lazy" /></span>
                  <span className="rack__line" aria-hidden="true" />
                  <h3>{c.short}</h3>
                </Link>
              ) : (
                // No photo of this vehicle type yet: the panel is a labelled strip of tape on the paint instead.
                <Link key={c.slug} href={'/' + c.slug} className="rack__item rack__item--type" data-rise>
                  <span className="rack__line" aria-hidden="true" />
                  <h3 className="rack__tag tape">{c.short}</h3>
                </Link>
              )
            ))}
          </div>
        </div>
      </Reveal>

      <Quote />

      <Reveal className="section" aria-labelledby="work-title">
        <div className="wrap">
          <div className="head-row">
            <h2 id="work-title" className="headline" data-split>Real cars.<br /><span className="after">Real Dapper care.</span></h2>
            <Link className="btn btn--line" href="/portfolio" data-rise>The full portfolio<ArrowUpRight size={20} aria-hidden /></Link>
          </div>
          <Gallery limit={6} />
        </div>
      </Reveal>

      <Reveal className="section" aria-labelledby="plan-title">
        <div className="wrap">
          <div className="head-row">
            <h2 id="plan-title" className="headline" data-split>Your vehicle.<br /><span className="after">Your next step.</span></h2>
          </div>
          <div className="process">
            <span className="process__line" data-scrub aria-hidden="true" />
            <article data-rise><span className="mark">1</span><h3>Tell us what you drive.</h3><p>Share your vehicle, its condition and the result you have in mind.</p></article>
            <article data-rise><span className="mark">2</span><h3>Find the right care.</h3><p>Talk through detailing, coating, film or tint with the Dapper team.</p></article>
            <article data-rise><span className="mark">3</span><h3>Make it Dapper.</h3><p>Confirm your package, pricing and appointment with the team.</p></article>
          </div>
        </div>
      </Reveal>

      <Reveal className="section" aria-labelledby="faq-title">
        <div className="wrap faq">
          <h2 id="faq-title" className="headline" data-split style={{ marginBottom: 40 }}>Good<br /><span className="after">to know.</span></h2>
          {faqs.map((f) => (
            <details key={f.q} data-rise>
              <summary>{f.q}<Plus size={28} aria-hidden /></summary>
              <p>{f.a}</p>
            </details>
          ))}
        </div>
      </Reveal>

      <Close before="Let’s make it" after="Dapper." />
    </>
  );
}
