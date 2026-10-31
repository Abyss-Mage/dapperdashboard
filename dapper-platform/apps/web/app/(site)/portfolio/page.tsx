import { transformations } from '../dapper';
import { CallToBook, PageHero, PageMotion, Reels, SectionHead } from '../_v3/blocks';
import Gallery3 from '../_v3/Gallery3';

export const metadata = { title: 'Portfolio', description: 'Real vehicles cared for by Dapper Detailing Pros in Peterborough.', alternates: { canonical: '/portfolio' } };

export default function Portfolio() {
  return (
    <PageMotion>
      <PageHero chip="Portfolio" title={<>Real results. <em>Real vehicles.</em></>} lead={transformations.text} image="/site/assets/portfolio/portfolio-01.jpg" alt="A black Range Rover at sunset after detailing" />
      <section className="d-section">
        <SectionHead chip="Our work" title="Work worth a closer look" text="Cars, trucks, classics and motorcycles cared for by the Dapper team. Select a photo to see it full size." />
        <Gallery3 />
      </section>
      <Reels button={false} />
      <CallToBook />
    </PageMotion>
  );
}
