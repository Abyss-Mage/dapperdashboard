import SplitHero from '../_components/SplitHero';
import ServiceRows from '../_components/ServiceRows';
import Reveal from '../_components/Reveal';
import { Close } from '../_components/Chrome';
import { ArrowUpRight } from '../_components/Icons';

export const metadata = { title: 'Services', description: 'Vehicle detailing, ceramic coating and paint protection film in Peterborough.', alternates: { canonical: '/services' } };

export default function Services() {
  return (
    <>
      <SplitHero image="/site/assets/suv.webp" alt="An SUV in the Dapper studio" before="Care for every" after="kind of journey." lead="Find your vehicle. Explore the care. Choose your next step with Dapper.">
        <a className="btn btn--red" href="/booking">Book an appointment<ArrowUpRight size={20} aria-hidden /></a>
      </SplitHero>
      <Reveal className="section" aria-labelledby="dir-title">
        <div className="wrap">
          <div className="head-row">
            <h2 id="dir-title" className="headline" data-split>Ten kinds<br /><span className="after">of vehicle.</span></h2>
            <p className="lede" data-rise>Cars, trucks, bikes, boats and aircraft, plus ceramic coating and paint protection film for any of them.</p>
          </div>
          <ServiceRows />
        </div>
      </Reveal>
      <Close />
    </>
  );
}
