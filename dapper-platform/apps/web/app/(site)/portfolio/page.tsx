import SplitHero from '../_components/SplitHero';
import Gallery from '../_components/Gallery';
import Reveal from '../_components/Reveal';
import { Close } from '../_components/Chrome';
import { ArrowUpRight } from '../_components/Icons';

export const metadata = { title: 'Portfolio', description: 'Vehicles and finishes cared for by Dapper Detailing Pros, published by the team.', alternates: { canonical: '/portfolio' } };

export default function Portfolio() {
  return (
    <>
      <SplitHero image="/site/assets/truck.webp" alt="A pickup truck after detailing at Dapper" before="Seen in the studio." after="Made for the road." lead="The vehicles and finishes cared for by Dapper, published by the team from the studio.">
        <a className="btn btn--red" href="/booking">Book an appointment<ArrowUpRight size={20} aria-hidden /></a>
      </SplitHero>
      <Reveal className="section" aria-labelledby="gallery-title">
        <div className="wrap">
          <div className="head-row">
            <h2 id="gallery-title" className="headline" data-split>Work worth<br /><span className="after">a closer look.</span></h2>
          </div>
          <Gallery />
        </div>
      </Reveal>
      <Close />
    </>
  );
}
