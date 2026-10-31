import { timeline } from '../content';
import { values } from '../dapper';
import { CallToBook, PageHero, PageMotion, PartnersStrip, SectionHead, WhyGrid } from '../_v3/blocks';

export const metadata = { title: 'About Us', description: 'The people and story behind Dapper Detailing Pros.', alternates: { canonical: '/about-us' } };

export default function About() {
  return (
    <PageMotion>
      <PageHero chip="About us" title={<>Good people. <em>Considered care.</em></>} lead="Built by two childhood friends, Dapper grew from a mobile detailing business into a dedicated Peterborough studio. The care behind every vehicle remains the same." image="/site/assets/portfolio/portfolio-08.jpg" alt="A Dapper detailer washing a GMC Hummer EV" />
      <section className="d-section">
        <SectionHead chip="Our story" title="A journey worth caring about" center />
        <ol className="d-timeline">
          {timeline.map((t) => <li key={t.year} data-d-in><time>{t.year}</time><div><h3>{t.title}</h3><p>{t.text}</p></div></li>)}
        </ol>
      </section>
      <section className="d-panel">
        <SectionHead chip="Our core values" title={values.heading} text={values.text} center dark />
        <div className="d-people">
          <article data-d-in><h3>Ayush Kumar Singh</h3><p>CEO</p></article>
          <article data-d-in><h3>Mohan Preet Singh Virk</h3><p>CTO</p></article>
        </div>
      </section>
      <WhyGrid />
      <PartnersStrip />
      <CallToBook />
    </PageMotion>
  );
}
