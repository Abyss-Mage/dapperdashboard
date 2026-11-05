/* The words that appear along the hero frame sequence, one per stage of the film. `at` is the frame the stage rests on (the film has
   288 frames; the frames were checked one by one: wash 36-81, paint correction 81-135, film on the rear 144-171, ceramic gloss 180-207,
   window tint 216-252, the finished car 261-287). STORY_SPAN is how many frames either side the words stay up, and the scroll lingers on
   those frames (see frameSequence.ts) so the words can be read. The headline is set like the rest of the site: plain words, then the last
   words in red (`em`). The pictures are AI-generated, so this copy stays visual and makes no price, warranty or availability claim.
   DRAFT COPY: the owner approves it before it ships. */
export type Stage = { at: number; chip: string; swatch: string; tag: string; title: string; em: string; text: string; side: 'left' | 'right' };

export const STORY: Stage[] = [
  { at: 60, chip: 'Foam wash', swatch: '#e8eef3', tag: 'Detailing', title: 'Start with a', em: 'proper wash.', text: 'Foam, rinse and hand detailing, inside and out.', side: 'right' },
  { at: 102, chip: 'Paint correction', swatch: '#d9c9a0', tag: 'Polish', title: 'Swirls out,', em: 'gloss in.', text: 'Machine polishing levels the paint so the light reflects cleanly.', side: 'left' },
  { at: 162, chip: 'Paint protection film', swatch: '#5fd0e6', tag: 'Protection', title: 'Film,', em: 'panel by panel.', text: 'Clear film guards the paint where stones and scuffs land.', side: 'right' },
  { at: 198, chip: 'Ceramic coating', swatch: '#7fa6ff', tag: 'Coating', title: 'Gloss that', em: 'stays put.', text: 'Depth, water repellency and long-lasting protection for the paint.', side: 'left' },
  { at: 224, chip: 'Window tint', swatch: '#6f8590', tag: 'Glass', title: 'Tint, fitted', em: 'in the shop.', text: 'Window tint cut and applied by the Dapper team.', side: 'right' },
  { at: 276, chip: 'Finished', swatch: '#c9a24a', tag: 'The result', title: 'Looking', em: 'Dapper.', text: 'Ready for the road. Book your detail.', side: 'left' },
];
export const STORY_SPAN = 12;
