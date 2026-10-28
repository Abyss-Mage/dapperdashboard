---
name: Dapper Detailing Pros (public website)
description: The 50/50 tape line. Gloss-black paint, masking tape for labels and navigation, Dapper red reserved for booking and the After mark.
colors:
  paint: "#09090b"
  paint-raised: "#121215"
  paint-high: "#1b1b1f"
  ink: "#f3f2ee"
  ink-muted: "#b9b7b0"
  ink-before: "#8c8a84"
  rule: "#ffffff1f"
  tape: "#d9c59c"
  tape-ink: "#2a2318"
  before-wash: "#7a6648"
  dapper-red: "#e3172b"
  dapper-red-hot: "#ff2b40"
typography:
  display:
    fontFamily: "'Barlow Condensed', 'Arial Narrow', sans-serif"
    fontSize: "clamp(52px, 6.8vw, 96px)"
    fontWeight: 700
    lineHeight: 0.9
    letterSpacing: "-0.01em"
  headline:
    fontFamily: "'Barlow Condensed', 'Arial Narrow', sans-serif"
    fontSize: "clamp(46px, 6.4vw, 96px)"
    fontWeight: 700
    lineHeight: 0.9
    letterSpacing: "-0.01em"
  title:
    fontFamily: "'Barlow Condensed', 'Arial Narrow', sans-serif"
    fontSize: "clamp(30px, 2.8vw, 44px)"
    fontWeight: 700
    lineHeight: 0.9
    letterSpacing: "-0.01em"
  numeral:
    fontFamily: "'Barlow Condensed', 'Arial Narrow', sans-serif"
    fontSize: "76px"
    fontWeight: 800
    lineHeight: 1
    fontFeature: "'tnum'"
  body:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "17px"
    fontWeight: 400
    lineHeight: 1.6
  lede:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "clamp(17px, 1.3vw, 20px)"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "'Barlow Condensed', 'Arial Narrow', sans-serif"
    fontSize: "15px"
    fontWeight: 700
    lineHeight: 1
    letterSpacing: "0.08em"
  action:
    fontFamily: "'Barlow Condensed', 'Arial Narrow', sans-serif"
    fontSize: "19px"
    fontWeight: 700
    lineHeight: 1
    letterSpacing: "0.06em"
rounded:
  none: "0px"
  grip: "50%"
spacing:
  gutter: "clamp(16px, 4.4vw, 72px)"
  section-y: "clamp(88px, 12vw, 180px)"
  head-row: "clamp(40px, 6vw, 88px)"
  header: "76px"
  wrap: "1440px"
components:
  button-red:
    backgroundColor: "{colors.dapper-red}"
    textColor: "#ffffff"
    typography: "{typography.action}"
    rounded: "{rounded.none}"
    padding: "0 22px"
    height: "52px"
  button-red-hover:
    backgroundColor: "{colors.dapper-red-hot}"
  button-tape:
    backgroundColor: "{colors.tape}"
    textColor: "{colors.tape-ink}"
    typography: "{typography.action}"
    rounded: "{rounded.none}"
    padding: "0 22px"
    height: "52px"
  button-tape-hover:
    backgroundColor: "#e4d2ab"
  button-line:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.action}"
    rounded: "{rounded.none}"
    padding: "0 22px"
    height: "52px"
  tape-label:
    backgroundColor: "{colors.tape}"
    textColor: "{colors.tape-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.none}"
    padding: "0.55em 0.9em 0.5em"
  tape-label-after:
    backgroundColor: "{colors.dapper-red}"
    textColor: "#ffffff"
    typography: "{typography.label}"
    rounded: "{rounded.none}"
    padding: "0.55em 0.9em 0.5em"
  chip:
    backgroundColor: "{colors.paint-high}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "0 18px"
    height: "46px"
  chip-pressed:
    backgroundColor: "{colors.tape}"
    textColor: "{colors.tape-ink}"
  header-strip:
    backgroundColor: "{colors.tape}"
    textColor: "{colors.tape-ink}"
    rounded: "{rounded.none}"
    height: "48px"
  header-book:
    backgroundColor: "{colors.dapper-red}"
    textColor: "#ffffff"
    rounded: "{rounded.none}"
    padding: "0 20px"
    height: "58px"
  tally:
    backgroundColor: "{colors.tape}"
    textColor: "{colors.tape-ink}"
    rounded: "{rounded.none}"
    padding: "28px"
---

# Design System: Dapper Detailing Pros (public website)

Scope: this file describes the public website only, the Next.js routes under `dapper-platform/apps/web/app/(site)/`, styled by `site.css` and loaded only by the `(site)` root layout. The dashboards under `app/(app)` (employee Workspace, customer account, Master administration) are a separate Operate surface with their own `globals.css` and glass controls; nothing here applies to them and nothing there applies here. The home scroll story (`story.css`, `Story.tsx`) is a deliberately preserved exception with its own staging; see Components.

## Overview

**Creative North Star: "The 50/50 Tape Line"**

The site is a detailer's test panel. A strip of masking tape divides each view into before and after, so the page shows the work instead of claiming it. Everything sits on gloss-black paint; masking tape is the only material laid on top of it, and it carries every label, the navigation, the instant-quote tally and the ways forward at the foot of each page. Dapper red is the paint that matters: booking, and the After mark.

Density is low and the type is loud. Big condensed caps run in two tones, the before half in grey and the after half in white, and the body copy is quiet Manrope at a comfortable reading measure. Depth is physical, not interface depth: tape lifts slightly off the paint and casts a soft shadow; nothing floats as a card.

The before side is always an illustration made from the same photograph (hazed, desaturated, washed in tape-brown) and is labelled as such. It is never presented as a real result.

**Key Characteristics:**
- Gloss-black paint ground with three tonal steps; no other surfaces.
- Masking tape (crepe-paper fibre texture, slight rotation, soft lift shadow) as the single applied material.
- Two-tone condensed caps headlines: before-grey, then after-white.
- Dapper red reserved for booking and the After mark.
- Square corners everywhere; the only circle is the drag grip.
- Motion is tape: laid from the roll end, peeled away, swept across between pages, dragged by the visitor.

## Colors

A near-monochrome paint field, one warm paper material, and one red that is spent only where the visitor commits.

### Primary
- **Dapper Red** (dapper-red): the booking actions (header Book tab, red buttons to `/booking`, the first link on the closing tape strip, "Book this" on the quote tally) and the After label on every split. Also the root `accent-color` and `caret-color`, so native checkboxes and the text caret read as Dapper. Under white type only.
- **Hot Red** (dapper-red-hot): hover state of red actions. Never a resting colour.

### Secondary
- **Masking Tape** (tape): every strip of tape: header, labels, the drag divider, the process and timeline lines, the tally, the closing strip, pressed chips, focus rings, text selection, and the hover colour of directory rows and contact links.
- **Tape Ink** (tape-ink): the dark brown-black written on tape. Type on tape is always this colour, never pure black.
- **Before Wash** (before-wash): the brown multiply layer over the before half of a split and of rack panels, the dust-and-swirl haze that makes "as it arrives" read as illustration.

### Neutral
- **Gloss Paint** (paint): page ground, html and theme colour.
- **Raised Paint** (paint-raised): photo wells behind taped images, empty/error state panels.
- **High Paint** (paint-high): resting chips.
- **Inspection White** (ink): primary text and the after half of every headline.
- **Muted Ink** (ink-muted): ledes, body paragraphs in sections, footer text.
- **Before Grey** (ink-before): the before half of every headline, small meta lines, inactive category links, past timeline years.
- **Rule** (rule): 1px dividers between rows, packages, FAQ items and footer bands.

### Named Rules
**The Paid-In-Red Rule.** Red marks only booking and the After side (plus the native control accent). If an element is red, the visitor is either booking or looking at the finished result.

**The Tape-Carries-Words Rule.** Labels, navigation and calls to move forward are written on tape in tape ink. Floating coloured text labels are not part of this world.

## Typography

**Display Font:** Barlow Condensed (with Arial Narrow, sans-serif), weights 600/700/800 loaded
**Body Font:** Manrope (with system-ui, sans-serif), weights 400/500/600/700 loaded

**Character:** Condensed workshop caps, the kind stencilled on a bay wall or written on tape, over a calm, rounded grotesk for the reading copy.

### Hierarchy
- **Display** (700, clamp 52-96px, line-height 0.9, uppercase, balanced): split-hero page titles, split into a before half (grey) and an after half (white). On phones it stacks at clamp(50px, 15vw, 84px).
- **Headline** (700, clamp 46-96px, 0.9, uppercase): section headings, before-grey with the `after` span in white. The closing panel uses clamp(56px, 7vw, 96px) at 0.88.
- **Title** (700, clamp 30-44px, 0.9, uppercase): process steps, timeline entries, packages (28-38px), rack panel names (26-40px). Directory rows run larger, clamp(34px, 4.6vw, 76px).
- **Numeral** (800, 76px, tabular): process marks in tape colour; timeline years run clamp(56px, 6.4vw, 96px) at 0.85.
- **Body** (400, 17px, 1.6): all reading copy, at 46-68ch measure.
- **Lede** (400, clamp 17-20px): the sentence beside a section headline, max 52ch, muted ink.
- **Label** (700, 15px, 0.08em, uppercase): text on tape labels; nav links are 17px at 0.1em, the vertical divider text 15px at 0.3em.
- **Action** (700, 19px, 0.06em, uppercase): button text.

### Named Rules
**The 96 Ceiling Rule.** No display text exceeds 96px. Scale comes from condensed width and two-tone contrast, not from size.

**The Before-Then-After Rule.** A two-line headline reads grey first and white second. The after half is the point; it gets the light.

## Layout

Content sits in a 1440px wrap with a fluid gutter (16-72px) and generous section padding (88-180px vertical). Section heads are a single row: headline left, lede or action right, bottom-aligned, wrapping on narrow screens, with 40-88px below.

Inner pages open on a full-viewport split (100svh, min 620px). At 760px and below the photo and its tape take the top 64svh (max 560px) and the title, lede and actions move below it onto the paint, clear of the tape. The header collapses links into a full-screen paint menu at 900px. Grids step down: rack 5 / 3 / 2 columns (1100px, 640px), gallery 3 / 2 / 1 with the middle column dropped 0-80px on desktop, category layout 280px rail plus content collapsing to a horizontally scrolling, edge-faded rail at 900px.

Lists are ruled, not boxed: directory rows, packages, FAQ items and contact cells are separated by 1px rules with no containers.

## Elevation & Depth

Depth is tape lifting off paint. Surfaces never shadow; only tape (and red tape) casts a soft, dark, downward shadow, larger for longer and heavier strips, with a 1px inner highlight on the top edge where the paper catches light. A paper-fibre SVG (`/site/assets/tape-fibre.svg`, authored procedural texture) is multiplied over tape at 50% (25% over red). There are no glass panels and no hard offset shadows.

### Shadow Vocabulary
- **Tape lift** (`box-shadow: 0 1px 0 #ffffff40 inset, 0 -1px 0 #00000014 inset, 0 3px 10px #0000005c`): small tape labels.
- **Pressed tape** (`box-shadow: 0 3px 10px #00000066`): buttons and pressed chips.
- **Header strip** (`box-shadow: 0 1px 0 #ffffff40 inset, 0 4px 14px #00000073`): the header tape.
- **Divider tape** (`box-shadow: 0 0 0 1px #0000001f, 0 8px 24px #000000a6`): the draggable split strip.
- **Heavy tape** (`box-shadow: 0 10px 30px #0009`): the quote tally; the closing strip uses `0 1px 0 #ffffff40 inset, 0 8px 24px #000a`.

### Named Rules
**The Only-Tape-Lifts Rule.** If it casts a shadow, it is tape. Paint, photos and text stay flat.

## Shapes

Square-cut rectangles throughout (0px radius). Tape strips sit slightly off-axis: header -0.35deg, labels -1.4deg / +1.2deg, the Book tab +1.6deg, the closing strip -1.2deg, the tally +0.6deg, photo corner tapes -38deg. The single circle is the drag grip on the split divider (44px). Photos are taped to the paint with two strips across opposite corners. Dashed tape-ink rules divide links written on the same strip.

## Components

### Buttons
Buttons are tape pressed onto the paint.
- **Shape:** square (0px), min-height 52px, 22px side padding, condensed caps with an arrow icon that nudges up-right on hover.
- **Red:** booking only. Hover to hot red.
- **Tape:** secondary physical actions (Visit the studio, Try again, Close). Hover lightens the tape.
- **Line:** a 1.5px inset white-35% outline on paint for onward links (The full portfolio); hover brightens the outline to ink.
- **Press:** scale 0.97 at 0.16s on `:active`. Focus is a 2px tape outline offset 3px site-wide.

### Tape labels
- **Style:** tape background, tape-ink condensed caps, fibre texture, tape-lift shadow, optional tilt. The red variant (white text) is the After label.
- **Use:** the Before · illustration and After marks on splits, the Drag the tape cue, tape-labelled rack panels for vehicle types without a photo, and the small tag inside quote block titles.

### Chips
- **Style:** high-paint square chips, 46px, 1px rule inset; hover strengthens the inset to before-grey.
- **State:** pressed chips become tape with tape ink and the pressed-tape shadow.

### Navigation
- **Header:** one tape strip across the top (48px inside a 76px band), logo and DAPPER wordmark, condensed caps links with a tape-ink underline drawn in from the left on hover and held on the current page, the red Book tab hanging off the right end. It hides on scroll down and returns on scroll up (0.5s).
- **Mobile (900px and below):** a Menu control on the strip opens a full-screen paint menu of 44-72px condensed links on rules, with the Book button at the foot.
- **Category rail:** grey condensed links; the current one turns white, steps right 10px and gains a tape square.

### Split hero (signature)
The 50/50 test panel. One photograph renders twice: the after layer with a slight contrast/saturation lift, the before layer clipped to the left of `--x` and treated with grayscale, sepia, low brightness, low contrast and a 1.6px blur under the before wash. A vertical tape strip (46-64px wide) at `--x` carries "Dapper · 50/50" and a round grip; it is a keyboard slider (arrows 5%, Shift 15%, Home/End to 4/96%). On load the tape is laid downward and sweeps from 88% to 50% so the change is seen happening. Photos that do not show the page's vehicle type carry a studio-photo credit. Open items, as built: the before side is a live CSS filter over the after photograph (masked raster before-plates are not yet produced), and only four real studio photographs exist, so most vehicle types use the studio credit or a tape-labelled panel. The direction named knife-cut tape ends; the shipped strips are square-cut, so square-cut is what is recorded here.

### Vehicle rack
Ten portrait panels, each half before and half after with a 16px tape line; hover or focus pulls the line to 0% to reveal the full after. Vehicle types without a photo are inspection-lit paint panels with the name on a rotated tape strip that straightens on hover.

### Directory rows
Big condensed titles on ruled rows; hover tints the title tape and slides it 14px; a taped preview photo follows the pointer on desktop.

### Quote ticket and tally
The instant quote is a work order: chip groups and ruled line items with tabular prices on the left, a sticky tape tally on the right with a 2px tape-ink rule above the 30px total and a full-width red Book this button.

### Closing strip
The last panel of every page: a two-tone headline, then one tape strip laid across the full width carrying Book (red tape, tilted, raised), Call and Talk to the team, divided by dashed rules.

### Motion grammar
Tape is laid from the roll end with a slight press-down overshoot (`back.out(1.6)`, 1.1s); photos are uncovered as a strip peels off them (clip-path, 1.4s `expo.inOut`); headings rise line by line out of a mask (1.15s, 0.09s stagger); everything else settles up 28px in reading order. Page changes sweep a 34vh tape strip across and peel it away to the right (0.9s). Process and timeline tapes grow with the scroll. CSS transitions use `cubic-bezier(.23, 1, .32, 1)`. Reduced motion shows final states with no animation; content is visible without JavaScript.

### Home scroll story (exception)
The home page opens on a pinned frame-sequence / real-time 3D story with its own staging in `story.css`: rounded pills, blurred HUD and control bar, red micro captions, round progress dots, and a separate red (#d10f1f / #fb1a2b). This is a preserved exception, not part of the system. Do not copy its pills, glass, captions, dots or reds onto any other surface; everything after the story uses the tape line.

## Do's and Don'ts

### Do:
- **Do** write labels, navigation and forward actions on tape in tape ink.
- **Do** set section headlines in two tones, before-grey then after-white, capped at 96px.
- **Do** label every before treatment as an illustration ("Before · illustration") and keep real portfolio photographs untreated.
- **Do** credit a photo as a studio photo when it does not show the page's vehicle type; use a tape-labelled panel when a vehicle type has no photo.
- **Do** separate lists with 1px rules instead of boxing them.
- **Do** keep reduced-motion and no-JavaScript states complete: final positions, no movement.

### Don't:
- **Don't** use Dapper red for anything other than booking, the After mark and the native control accent.
- **Don't** round corners; the drag grip is the only circle.
- **Don't** add shadows to paint, photos or text; only tape lifts.
- **Don't** set display text above 96px.
- **Don't** carry the home story's pills, blurred glass panels, micro captions or story reds into the rest of the site.
- **Don't** present a before treatment as a real result or invent prices, results or claims on tape.
