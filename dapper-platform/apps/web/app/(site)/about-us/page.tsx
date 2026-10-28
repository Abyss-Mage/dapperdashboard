import { timeline } from '../content';
import SplitHero from '../_components/SplitHero';
import Reveal from '../_components/Reveal';
import { Close } from '../_components/Chrome';
import { ArrowUpRight } from '../_components/Icons';

export const metadata = { title: 'About Us', description: 'The people and story behind Dapper Detailing Pros.', alternates: { canonical: '/about-us' } };

export default function About() {
  return (
    <>
      <SplitHero image="/site/assets/hero.webp" alt="The Dapper detailing studio" before="Good people." after="Considered care." lead="Built by two childhood friends, Dapper grew from a mobile detailing business into a dedicated Peterborough studio. The care behind every vehicle remains the same.">
        <a className="btn btn--tape" href="/contact-us">Visit the studio<ArrowUpRight size={20} aria-hidden /></a>
      </SplitHero>
      <Reveal className="section" aria-labelledby="journey-title">
        <div className="wrap">
          <div className="head-row">
            <h2 id="journey-title" className="headline" data-split>A journey<br /><span className="after">worth caring about.</span></h2>
          </div>
          <div className="timeline">
            <span className="timeline__tape" data-scrub="y" aria-hidden="true" />
            <ol>
              {timeline.map((t) => (
                <li key={t.year} data-rise>
                  <time className="num">{t.year}</time>
                  <div><h3>{t.title}</h3><p>{t.text}</p></div>
                </li>
              ))}
            </ol>
          </div>
        </div>
      </Reveal>
      <Reveal className="section" aria-labelledby="people-title">
        <div className="wrap">
          <h2 id="people-title" className="headline" data-split>The people<br /><span className="after">behind Dapper.</span></h2>
          <div className="people">
            <div data-rise><h3>Ayush Kumar Singh</h3><p>CEO</p></div>
            <div data-rise><h3>Mohan Preet Singh Virk</h3><p>CTO</p></div>
          </div>
        </div>
      </Reveal>
      <Close />
    </>
  );
}
