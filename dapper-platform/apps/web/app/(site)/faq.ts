/* Questions and answers for each service page, in the studio's own words from dapperdetailing.ca. Hatchbacks, pickups and vans use the
   same four questions as sedans and SUVs. Window tint uses only what the catalogue states (the time of each package). */
export type Faq = { q: string; a: string };

const car = (noun: string, article = 'a'): Faq[] => [
  { q: `How often should I get my ${noun} professionally detailed?`, a: `We recommend detailing your ${noun} every 3–6 months to maintain its appearance, protect interior and exterior surfaces, and help preserve its long-term value.` },
  { q: `What’s included in ${article} ${noun} detailing service?`, a: 'Depending on the package selected, services may include interior deep cleaning, exterior washing and waxing, wheel and tire care, glass cleaning, steam cleaning, fabric protection, and ceramic coating.' },
  { q: `How long does ${noun} detailing take?`, a: 'The time depends on the package selected. Exterior wash services typically take around 1 hour, while complete interior and full-body detailing packages can take 3–6 hours to ensure exceptional results.' },
  { q: `Is ceramic coating included with every ${noun} detailing package?`, a: 'No. Ceramic coating is included only in select premium packages such as our Ultimate Protection Package. Other packages may include wax protection or can be upgraded with ceramic coating for enhanced paint protection.' },
];

export const faqs: Record<string, Faq[]> = {
  hatchback: car('hatchback'),
  sedan: car('sedan'),
  suv: car('SUV', 'an'),
  pickup: car('pickup'),
  van: car('van'),
  '2-wheeler': [
    { q: 'How often should I have my motorcycle professionally detailed?', a: 'We recommend professional motorcycle detailing every 2–4 months, depending on riding frequency, weather exposure, and storage conditions. Regular detailing helps maintain appearance and protect painted and chrome surfaces.' },
    { q: 'What’s included in a 2 wheeler detailing service?', a: 'Our 2 wheeler detailing service may include foam washing, waxing, wheel and tire cleaning, chrome polishing, screen cleaning, engine cleaning, and surface protection depending on the package selected.' },
    { q: 'How long does motorcycle detailing take?', a: 'Most motorcycle detailing services take between 1–3 hours, depending on the bike’s size, condition, and the level of detailing required.' },
    { q: 'Can detailing help protect my motorcycle from weather and road grime?', a: 'Yes. Professional detailing helps remove dirt, bugs, road grime, and contaminants while providing protection against UV rays, moisture, and environmental exposure that can affect your motorcycle’s finish over time.' },
  ],
  ceramic: [
    { q: 'How long does ceramic coating last?', a: 'Our ceramic coating packages can provide protection for several months to multiple years, depending on the coating selected, vehicle usage, and maintenance routine. Proper care helps maximize the coating’s lifespan and performance.' },
    { q: 'What are the benefits of ceramic coating?', a: 'Ceramic coating creates a protective layer that helps repel water, dirt, road contaminants, and UV rays. It also enhances gloss, makes cleaning easier, and helps preserve your vehicle’s paintwork.' },
    { q: 'How long does ceramic coating application take?', a: 'Application time varies depending on the vehicle’s size, condition, and the level of paint preparation required. Most ceramic coating services take between 4–12 hours, while advanced coatings may require additional curing time.' },
    { q: 'Does ceramic coating eliminate the need to wash my vehicle?', a: 'No. Ceramic coating makes washing easier and helps reduce dirt buildup, but regular maintenance is still recommended to keep your vehicle looking its best and to maximize the coating’s longevity.' },
  ],
  ppf: [
    { q: 'How long does PPF last?', a: 'High-quality PPF can provide several years of protection when properly installed and maintained. Its lifespan depends on the film selected, vehicle usage, environmental exposure, and aftercare.' },
    { q: 'What does PPF protect my vehicle from?', a: 'PPF helps protect your original paint from stone chips, road debris, light scratches, bug splatter, stains, and everyday road hazards. It acts as a sacrificial layer between your paint and the outside world.' },
    { q: 'Does PPF damage or change the look of my paint?', a: 'No. Premium PPF is designed to maintain the appearance of your original paint while adding a clear protective layer. Proper installation also helps create a clean, seamless finish.' },
    { q: 'How long does PPF installation take?', a: 'Installation time depends on the vehicle size, coverage selected, and condition of the paint. Full PPF generally requires more time than Front PPF because significantly more panels need to be prepared and installed.' },
  ],
  aircraft: [
    { q: 'How often should I have my aircraft professionally detailed?', a: 'We recommend professional aircraft detailing every 3–6 months, depending on flight frequency, storage conditions, and environmental exposure. Regular detailing helps maintain appearance and protect exterior surfaces.' },
    { q: 'What’s included in an aircraft detailing service?', a: 'Our aircraft detailing packages may include exterior washing and polishing, cabin cleaning, window cleaning, surface protection, stain removal, and specialized detailing services depending on the package selected.' },
    { q: 'How long does aircraft detailing take?', a: 'The time required varies based on the aircraft’s size and the package selected. Most detailing services can take anywhere from 3–8 hours to ensure a thorough and professional finish.' },
    { q: 'Can detailing help protect my aircraft’s exterior?', a: 'Yes. Professional detailing helps remove contaminants, preserve painted surfaces, and protect against UV exposure, dirt buildup, and environmental elements that can affect your aircraft’s appearance over time.' },
  ],
  boat: [
    { q: 'How often should I have my boat professionally detailed?', a: 'We recommend professional boat detailing every 3–6 months, depending on how frequently your boat is used and its exposure to sun, saltwater, and environmental conditions.' },
    { q: 'What’s included in a boat detailing service?', a: 'Our boat detailing packages may include exterior washing, hull cleaning, polishing, interior cleaning, glass treatment, surface protection, stain removal, and other marine-specific detailing services depending on the package selected.' },
    { q: 'How long does boat detailing take?', a: 'The time required depends on the size and condition of the vessel. Most boat detailing services take between 3–8 hours, while larger boats may require additional time for a complete finish.' },
    { q: 'Can detailing help protect my boat from sun and water damage?', a: 'Yes. Professional detailing helps protect surfaces from UV exposure, water spotting, oxidation, salt buildup, and other environmental contaminants that can affect your boat’s appearance over time.' },
  ],
  'window-tint': [
    { q: 'How long does window tinting take?', a: 'A single window takes about 30 minutes, the standard 4-window package about 60 minutes, and the full 5-window premium package about 90 minutes.' },
    { q: 'Which window tint package should I choose?', a: 'Choose a single window to tint one window, the standard 4-window package for four windows, or the full 5-window premium package for five. Not sure? Call the team and we will point you to the right one.' },
  ],
};
