import { categories } from './content';

/* What each package includes, in the studio's own words from dapperdetailing.ca. Used when the package's published summary is still the
   generic booking line; once the team writes a real summary in the dashboard (one item per line) that is shown instead. */
export const GENERIC = /book online, or call the team/i;
export const INCLUDES: [RegExp, string[]][] = [
  [/front ppf/i, ['Full hood', 'Full front bumper', 'Both front fenders', 'Professional installation and edge finishing']],
  [/full ppf/i, ['Full exterior paint coverage', 'Hood, bumpers, fenders and doors', 'Quarter panels, roof and trunk', 'Other painted exterior panels', 'Professional installation and edge finishing']],
  [/express interior/i, ['Full interior vacuum: seats, carpets and floor mats', 'Trunk or cargo area vacuumed', 'Dash, console and door panels wiped down', 'Cupholders, vents and crevices dusted out', 'Interior glass cleaned, streak-free', 'Floor mats pulled, shaken and cleaned', 'Light deodorize for a fresh finish']],
  [/armour up/i, ['Full 3-stage paint correction', 'Swirl marks, scratches, oxidation and paint imperfections removed', 'Premium ceramic coating applied', '5-Year Manufacturer Warranty', 'Before-and-after photos of the transformation']],
  [/^3 stage paint correction/i, ['Removes heavy swirl marks, scratches, oxidation and paint defects', 'Restores gloss, clarity and depth', 'Mirror-like reflections']],
  [/fortress/i, ['Premium-grade ceramic coating', 'Complete paint decontamination and surface preparation', 'Enhanced hydrophobic protection', 'Superior resistance to minor scratches', 'Protection against UV rays and chemical contaminants', 'Annual maintenance reminders', '5-Year Manufacturer Warranty']],
  [/shield package/i, ['Professional ceramic coating application', 'Complete paint decontamination and surface preparation', 'Hydrophobic water-repelling layer', 'Protection against harmful UV rays', '6-hour professional installation', 'Post-cure care guidance', '3-Year Manufacturer Warranty']],
  [/5 year ceramic/i, ['Professional ceramic coating', 'High-gloss hydrophobic finish', 'Protection against UV rays and contaminants', '5-Year Ceramic Coating Warranty']],
  [/single window/i, ['Replace or upgrade the tint on a single window', 'Ideal for small fixes and privacy', 'Heat and UV protection on a specific window']],
  // Owner-approved wording, 2026-11-14 (the old booking system had none for this package).
  [/5[- ]window/i, ['Complete tinting for all four side windows and the rear windshield', 'Enhances privacy', 'Reduces interior heat and blocks harmful UV rays', 'A sleek, uniform look']],
  [/4[- ]window/i, ['Complete tinting for all four side windows', 'Enhances privacy', 'Reduces interior heat and blocks harmful UV rays', 'A sleek, uniform look']],
  [/^(boat|aircraft).*signature/i, ['Full interior detail', 'Full exterior detail', 'Surfaces cleaned, decontaminated and dried', 'Protective wax finish']],
  [/^(boat|aircraft).*paint correction/i, ['Surface washed and decontaminated', 'Machine polishing to reduce swirls and light scratches', 'Gloss restored across the paint']],
  [/^(boat|aircraft).*ceramic/i, ['Surface washed and decontaminated', 'Ceramic coating applied to the exterior', 'A glossy, water-repellent finish']],
  [/^(boat|aircraft).*interior/i, ['Thorough vacuum of every surface', 'Seats, carpet and upholstery cleaned', 'Panels, trim and vinyl wiped and conditioned', 'Glass and windows cleaned inside']],
  [/ultimate/i, ['Includes the Luxury Full Body Detail', '12–18 month ceramic coating', '12-month fabric protection', 'Windshield ceramic coating', 'Glass cleaned inside and out']],
  [/luxury/i, ['Full interior detail', 'Exterior detail and wax', 'Rims and wheels detailed', 'Exterior surfaces enhanced', 'Tire dressing finish']],
  [/vip/i, ['Interior revived and shined', 'Professional steam cleaning', 'Deep seat cleaning', 'Glass and windows cleaned', 'Air freshener treatment']],
  [/exterior/i, ['Thorough hand wash', 'Wheel and tire cleaning', 'Bug and tar removal', 'Door jamb cleaning', '30-day wax protection']],
];
const BLURBS = new Set(categories.map((c) => c.summary.trim()));
export const generic = (summary: string) => GENERIC.test(summary) || BLURBS.has(summary.trim());
export const includes = (name: string, summary: string): { list: string[]; text: string } => {
  if (summary && !generic(summary)) {
    const lines = summary.split(/\r?\n|•|;/).map((x) => x.trim()).filter(Boolean);
    return lines.length > 1 ? { list: lines, text: '' } : { list: [], text: summary };
  }
  const hit = INCLUDES.find(([re]) => re.test(name));
  return { list: hit ? hit[1] : [], text: hit || generic(summary) ? '' : summary };
};

// The built-in list for a package name, for the dashboard editor to start from (error404 #132).
export const builtInIncludes = (name: string) => INCLUDES.find(([re]) => re.test(name))?.[1] ?? [];
