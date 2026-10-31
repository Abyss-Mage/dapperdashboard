// Public website copy. Carried over from main website/build-pages.py; no prices, results or records are invented here.

// `photo` is true only when the image actually shows that kind of vehicle or work. Otherwise the image is the studio and the
// site says so (until real photos of each vehicle type exist).
// `from` is the "Starting from" price the current public site shows (only for the categories it prices).
export type Category = { key: string; slug: string; title: string; short: string; summary: string; image: string; photo: boolean; from?: number };

const img = (name: string) => '/site/assets/' + name;

export const categories: Category[] = [
  { key: '2-wheeler', slug: '2-wheeler-detailing', title: '2 Wheeler detailing', short: 'Motorcycles', image: img('portfolio/portfolio-17.jpg'), photo: true, summary: 'Care for the details that make your motorcycle yours. Talk to the team about cleaning, finishing and protection for your two-wheeler.' },
  { key: 'hatchback', slug: 'hatchback-detailing', title: 'Hatchback detailing', short: 'Hatchbacks', image: img('portfolio/portfolio-16.jpg'), photo: false, summary: 'A fresh cabin and a considered exterior finish for the car that takes you through every day.' },
  { key: 'sedan', slug: 'sedan-detailing', title: 'Sedan detailing', short: 'Sedans', image: img('services/sedan.jpg'), photo: true, from: 189, summary: 'Interior cleaning, exterior care and protection options for your daily drive. Find a package that fits your vehicle and its condition.' },
  { key: 'suv', slug: 'suv-detailing', title: 'SUV detailing', short: 'SUVs', image: img('services/suv.webp'), photo: true, from: 189, summary: 'From the school run to the weekend away, give your SUV’s cabin and exterior the attention they deserve.' },
  { key: 'pickup', slug: 'pickup-detailing', title: 'Pickup detailing', short: 'Pickups', image: img('services/pickup.jpg'), photo: true, from: 189, summary: 'Care for a working truck or a weekend companion. Discuss interior cleaning, exterior detailing and protection with the team.' },
  { key: 'van', slug: 'van-detailing', title: 'Van detailing', short: 'Vans', image: img('portfolio/portfolio-14.jpg'), photo: false, summary: 'Detailing for the space you rely on. Talk to us about the size, use and condition of your van to find the right care.' },
  { key: 'ceramic', slug: 'ceramic-coating', title: 'Ceramic coating', short: 'Ceramic', image: img('services/ceramic.jpg'), photo: true, from: 499, summary: 'Explore a coating finish that brings gloss and water repellency. The team will explain preparation, maintenance and suitable coating options.' },
  { key: 'ppf', slug: 'ppf', title: 'Paint protection film', short: 'PPF', image: img('portfolio/portfolio-06.jpg'), photo: true, summary: 'A clear film layer for the painted surfaces you want to protect. Discuss coverage, preparation and finish with Dapper.' },
  { key: 'aircraft', slug: 'aircraft-detailing', title: 'Aircraft detailing', short: 'Aircraft', image: img('services/aircraft.png'), photo: true, from: 249, summary: 'Specialist attention for aircraft interiors and exteriors. Contact the team to discuss your aircraft, its surfaces and the scope of work.' },
  { key: 'boat', slug: 'boat-detailing', title: 'Boat detailing', short: 'Boats', image: img('portfolio/portfolio-08.jpg'), photo: false, summary: 'Interior and exterior care for your time on the water. Tell us about your boat and its condition for a suitable service recommendation.' },
];

export const timeline = [
  { year: '2022', title: 'The beginning', text: 'Ayush began detailing alongside his studies in Canada, building the business through mobile work and referrals.' },
  { year: '2023', title: 'Building the foundation', text: 'Mohan Preet Singh Virk joined, bringing technology and digital systems into the business.' },
  { year: '2024', title: 'Reaching more drivers', text: 'The business invested in its digital presence and reached more vehicle owners around Peterborough.' },
  { year: '2025', title: 'Growing commercial relationships', text: 'Dapper expanded its work with dealerships and commercial clients.' },
  { year: '2026', title: 'A home for Dapper', text: 'The first shop opened at 660 The Kingsway in Peterborough, alongside mobile detailing services.' },
];

export const studio = {
  phone: '(705) 977-6217',
  tel: '+17059776217',
  email: 'info@dapperdetailing.ca',
  street: '660 The Kingsway',
  city: 'Peterborough, ON K9J 7C8',
  hours: '8:00 AM–9:00 PM, seven days a week',
  maps: 'https://www.google.com/maps/search/?api=1&query=660+The+Kingsway+Peterborough+ON+K9J+7C8',
};

export const nav = [
  { href: '/services', label: 'Services' },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/about-us', label: 'About' },
  { href: '/contact-us', label: 'Contact' },
];
