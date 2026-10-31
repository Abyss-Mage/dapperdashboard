'use client';
// The Contact page's extra GSAP plugins, kept out of the shared bundle: button text that diffs between states, a scrambled
// city tag, and the small burst when an enquiry is sent. Everything else comes from ./gsap.
import { TextPlugin } from 'gsap/TextPlugin';
import { ScrambleTextPlugin } from 'gsap/ScrambleTextPlugin';
import { Physics2DPlugin } from 'gsap/Physics2DPlugin';
import { gsap, ScrollTrigger, SplitText, useGSAP } from './gsap';

gsap.registerPlugin(TextPlugin, ScrambleTextPlugin, Physics2DPlugin);

export { gsap, ScrollTrigger, SplitText, useGSAP };
