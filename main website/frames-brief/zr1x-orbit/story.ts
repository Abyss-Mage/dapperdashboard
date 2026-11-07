/* The words that appear along the hero frame sequence, one per service the film shows. `at` is the frame the stage rests on in the
   288-frame orbit film (Corvette ZR1X, 2026-11-05: intro light-up, then detailing, paint correction, PPF, ceramic coating, window tint,
   reveal; the camera orbits the car the whole way). STORY_SPAN is how many frames either side the words stay up, and the scroll lingers on
   those frames (see frameSequence.ts). Headline: plain words, then the last words in red (`em`). The pictures are AI-generated
   illustrations of the services, so the copy stays visual and makes no price, warranty or availability claim.
   DRAFT COPY: the owner approves it before it ships. */
export type Stage = { at: number; chip: string; swatch: string; tag: string; title: string; em: string; text: string; side: 'left' | 'right' };

export const STORY: Stage[] = [
  { at: 80, chip: 'Detailing', swatch: '#e8eef2', tag: 'Detailing', title: 'Every surface,', em: 'reset.', text: 'Foam, rinse and a full detail at 660 The Kingsway.', side: 'left' },
  { at: 121, chip: 'Paint correction', swatch: '#c9d3da', tag: 'Correction', title: 'Swirls out,', em: 'gloss back.', text: 'Machine polishing that takes out swirl marks and haze.', side: 'left' },
  { at: 162, chip: 'Paint protection film', swatch: '#5fd0e6', tag: 'Protection', title: 'Film,', em: 'panel by panel.', text: 'Clear film guards the paint where stones and scuffs land.', side: 'left' },
  { at: 203, chip: 'Ceramic coating', swatch: '#1d3b6e', tag: 'Coating', title: 'Water just', em: 'walks off.', text: 'A hard, slick coating that keeps the gloss and sheds water.', side: 'left' },
  { at: 232, chip: 'Window tint', swatch: '#39434a', tag: 'Window tint', title: 'Cut for', em: 'your glass.', text: 'Film traced to each window, then fitted in the shop.', side: 'left' },
  { at: 246, chip: 'Darker tint', swatch: '#14181b', tag: 'Window tint', title: 'Choose', em: 'how dark.', text: 'More privacy and less glare, in the shade you pick.', side: 'left' },
];
export const STORY_SPAN = 8;
