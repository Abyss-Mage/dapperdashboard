// Dapper's public content, taken word for word from the current site (dapperdetailing.ca, read 2026-10-30) and the studio's
// own media. Prices are the "Starting from" figures the current site publishes; the user approved showing them.

export const hero = {
  title: 'Detailing that speaks for itself.',
  lead: 'Detailing, ceramic coating, paint protection film and window tint at 660 The Kingsway, Peterborough.',
};

export const values = {
  heading: 'Driven by quality, powered by passion',
  text: 'Keeping automobiles detailed isn’t just about aesthetics; it’s about value and enhancing longevity. Regular detailing maintains the interior’s condition and also the external aesthetics ultimately prolong the life of your vehicle.',
};

export const servicesIntro = {
  heading: 'Detailing services for every vehicle',
  text: 'No two vehicles are the same. That’s why we offer specialized detailing and ceramic coating services for sedans, SUVs, pickup trucks, aircraft, and more, ensuring the right level of care for every vehicle.',
};

export type Service = { slug: string; key: string; name: string; from?: number; image: string; text: string };
export const services: Service[] = [
  { slug: 'sedan-detailing', key: 'sedan', name: 'Sedan detailing', from: 189, image: '/site/assets/services/sedan.jpg', text: 'Perfect for daily drivers and commuter vehicles, our sedan detailing services are designed to restore cleanliness, enhance appearance, and protect your vehicle inside and out.' },
  { slug: 'suv-detailing', key: 'suv', name: 'SUV detailing', from: 189, image: '/site/assets/services/suv.webp', text: 'From family SUVs to luxury crossovers, our detailing services help remove built-up dirt, refresh interior surfaces, and keep your vehicle looking its best year-round.' },
  { slug: 'pickup-detailing', key: 'pickup', name: 'Pickup detailing', from: 189, image: '/site/assets/services/pickup.jpg', text: 'Built for work and adventure, pickup trucks require specialized care. Our detailing packages help clean, protect, and maintain both the interior and exterior of your truck.' },
  { slug: 'ceramic-coating', key: 'ceramic', name: 'Ceramic coating', from: 499, image: '/site/assets/services/ceramic.jpg', text: 'Enhance gloss, improve water repellency, and add long-lasting protection with our professional ceramic coating solutions designed to preserve your vehicle’s finish.' },
  { slug: 'aircraft-detailing', key: 'aircraft', name: 'Aircraft detailing', from: 249, image: '/site/assets/services/aircraft.png', text: 'Our aircraft detailing services help maintain a clean, polished appearance while protecting exterior surfaces and enhancing the overall presentation of your aircraft.' },
];

export const partners = [
  { src: '/site/assets/partners/partner-1.png', name: 'Angevaare Mazda' },
  { src: '/site/assets/partners/partner-2.png', name: 'Jack McGee Peterborough' },
  { src: '/site/assets/partners/partner-3.png', name: 'Subaru' },
  { src: '/site/assets/partners/partner-4.png', name: 'Hyundai' },
];

export const transformations = {
  heading: 'Real Results. Real Vehicles. Real Protection.',
  text: 'Every vehicle tells a story and every transformation showcases our commitment to excellence. From paint correction and ceramic coatings to complete interior and exterior detailing, our work is designed to restore, protect, and elevate your vehicle’s appearance. Watch these real transformations and discover why vehicle owners across Peterborough trust Dapper Detailing Pros to keep their investments looking their absolute best.',
  clips: ['166', '500', '623', '890', '676'].map((n) => '/site/assets/transformations/transformation-' + n + '.webm'),
};

export const why = [
  { title: 'Trained professionals', text: 'Our experienced team is trained in professional detailing methods and continuously follows industry best practices. Every vehicle is handled with care, precision, and attention to detail to ensure consistently high-quality results.' },
  { title: 'Premium products', text: 'We use carefully selected professional-grade products trusted throughout the detailing industry. From cleaning solutions to protective coatings, every product is chosen to deliver superior performance, durability, and vehicle-safe results.' },
  { title: 'Pick-up & drop-off', text: 'We understand that finding time for vehicle maintenance can be challenging. Our convenient pick-up, drop-off, and shuttle options help make the detailing process as seamless and hassle-free as possible for our customers.' },
  { title: 'Standard procedures', text: 'Every service follows a proven process developed to maintain consistency and quality. By adhering to structured procedures, we ensure that every vehicle receives the same level of care and attention regardless of the service selected.' },
  { title: 'Specialized equipment', text: 'Achieving exceptional results requires more than just skill. Our team utilizes professional detailing equipment, advanced tools, and specialized machinery designed to safely and effectively clean, restore, and protect your vehicle.' },
  { title: 'Flexible scheduling', text: 'Whether you need service during the week or on the weekend, we offer flexible scheduling options to accommodate your lifestyle. Our goal is to make professional vehicle care convenient and accessible for every customer.' },
];

export const processSteps = [
  { title: 'Schedule your booking', text: 'An easy online booking system on our website; choose mobile service or at-shop detailing, pick your date and time in seconds.', image: '/site/assets/portfolio/portfolio-06.jpg' },
  { title: 'Confirmed & covered', text: 'You get instant confirmation, and our pros show up on time at your location (or welcome you at the shop), fully equipped and ready to go.', image: '/site/assets/portfolio/portfolio-08.jpg' },
  { title: 'The Dapper treatment', text: 'We treat your vehicle like it’s our own, using premium products and proven techniques to deliver a flawless, head-turning finish.', image: '/site/assets/portfolio/portfolio-07.jpg' },
  { title: 'Looking Dapper', text: 'We walk you through the results, make sure you’re thrilled, and send you off riding clean — rebook anytime to keep that fresh look.', image: '/site/assets/portfolio/portfolio-01.jpg' },
];

export const testimonials = [
  { quote: 'Dropped off a disaster, got back a new car. Impressed is an understatement. Thank you guys', name: 'Josh R' },
  { quote: 'My car looks brand new, even better than when I saw it off the showroom floor ! Absolutely stellar service, quality, and communication. Would recommend, 5/5', name: 'Thomas Vaccaro' },
  { quote: 'Absolutely AMAZING work was done on my 2 vehicles! Highly recommend them! Price was fantastic as well, very reasonable. Super friendly and passionate about what they do!! Thanks guys!!!', name: 'Bryan Morgan' },
  { quote: 'They did a great job getting every single bug off the car and all the dog hair out of our Chevy equinox. You wouldn’t even know there was a dog in it. Super job guys', name: 'Christina Murdock' },
  { quote: 'Very pleased with the outcome. Customer service was great can tell they take pride in making sure the customer is happy and satisfied', name: 'Angela White' },
  { quote: 'Had both vehicles detailed today. The team did a great job and the customer service was excellent. Will go back for sure.', name: 'kevin stacey' },
  { quote: 'First time customer, definitely using them again. I had the privilege of chatting with the owner today, you can tell he’s all about customer service and grateful. Plus they gave me a free pizza. 10/10 recommend.', name: 'Jenn' },
  { quote: 'They took considerably longer than they predicted, however in the end that translated into a very thorough job.', name: 'William Loukides' },
];

export const cta = {
  heading: 'Ready to Give Your Car the Care It Deserves?',
  text: 'Choose the detailing package that fits your vehicle’s needs and enjoy a cleaner, better-protected Car with professional care from the Dapper Detailing Pros team.',
};

export const contact = {
  phone: '(705) 977-6217', tel: '+17059776217', email: 'info@dapperdetailing.ca',
  street: '660 The Kingsway', city: 'Peterborough, ON K9J 7C8', country: 'Canada',
  directions: 'https://share.google/qmWu43MCkVzDXTZ34',
  instagram: 'https://www.instagram.com/dapperdetailingpros', youtube: 'https://www.youtube.com/@DapperDetailingPros',
};

export const proof = { customers: 'Trusted by 1000+ customers', rating: '5☆ Google Ratings' };

// The portfolio: photographs from Dapper's current public portfolio page (main website/portfolio-import.json, retrieved
// 2026-10-24). Captions describe only what each photo shows.
export const portfolio = [
  ['01', 'Black Range Rover at sunset'], ['02', 'RAM pickup beside a light aircraft'], ['03', 'Black Ford F-150'], ['04', 'BMW cabin with tan leather'],
  ['05', 'White Dodge RAM pickup'], ['06', 'White Porsche 911 at the studio'], ['07', 'Classic Mercedes, engine bay open'], ['08', 'GMC Hummer EV during a wash'],
  ['09', 'Black classic Chevrolet Nova'], ['10', 'Porsche cabin at night'], ['11', 'Jeep Wrangler'], ['12', 'Grey RAM 1500'],
  ['13', 'Black Cadillac Escalade'], ['14', 'Grey Mazda SUV'], ['15', 'Black Ford F-150, front view'], ['16', 'Grey Genesis SUV'],
  ['17', 'Harley-Davidson motorcycle'], ['18', 'Classic blue Dodge pickup'],
].map(([n, caption]) => ({ src: '/site/assets/portfolio/portfolio-' + n + '.jpg', caption }));
