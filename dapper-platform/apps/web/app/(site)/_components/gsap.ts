'use client';
// One place registers the GSAP plugins the public site uses, so every component imports gsap from here.
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';
import { SplitText } from 'gsap/SplitText';
import { useGSAP } from '@gsap/react';

gsap.registerPlugin(ScrollTrigger, SplitText, useGSAP);
ScrollTrigger.config({ ignoreMobileResize: true });
gsap.defaults({ ease: 'expo.out', duration: 1 });

// The site's two eases: tape being laid (a slight overshoot as it is pressed down) and everything else (fast out, long settle).
export const LAY = 'back.out(1.6)';
export const OUT = 'expo.out';

export { gsap, ScrollTrigger, SplitText, useGSAP };
