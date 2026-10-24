# Historical transformation studio prototype

Updated 21 October 2026. This replaces the client-rejected Ferrari presentation with a Porsche Carrera GT garage demo, informed by the user-supplied joinsparq.mp4 reference. Official primary colour: #FB1A2B.

## Run and review

Open http://127.0.0.1:4173/ while the local preview server is running. To start another preview, serve this directory with a static HTTP server; do not open index.html directly from the filesystem because the 3D scene uses JavaScript modules.

No build step, account or backend is required. Runtime assets are local. Booking links intentionally open the existing Dapper website in a new tab.

## Client walkthrough

1. Arrival: a dirty Porsche Carrera GT in a dark detailing garage. Scroll into the transformation studio.
2. Detailing: drag the comparison slider or select Before / After to remove or restore procedural road dirt.
3. Ceramic: the camera opens close to the side panels in falling rain. Select Hood close-up to inspect the bonnet. Transparent refractive beads and runoff illustrate water repellency; switch the water test on/off and compare the concept states.
4. PPF: explore a glossy full-body finish. Toggle coverage highlights across the painted body and carbon panels. The coloured lines explain coverage; real film is clear. This is not a panel-accurate installation template.
5. Window tint: adjust 20–95% visible light transmission. Lower VLT looks darker. Compare the original glass against the selected shade.
6. Inside out: open the custom demo door, then choose Open door, Dashboard or Seats & trim. Close the cabin to reset it.
7. Continue to Dapper's real portfolio, open a photograph, explore the FAQ and follow an existing booking link.

## Camera controls

- Middle mouse button held + drag: orbit horizontally and vertically.
- Left mouse button held + drag: pan the camera.
- Works in the normal arrival scene, all service modes and the interior scene.
- Reset camera restores the current shot. Front, Side and Rear buttons choose composed exterior angles. Rotate enables a continuous exterior orbit.
- Touch: sideways swipe rotates; vertical scrolling remains available.
- Pause motion stops continuous rendering and smooth scrolling. Controls draw static results while paused. The operating system's reduced-motion setting starts the experience paused.

## Implemented technology

Three.js 0.186.1 loads a local Porsche Carrera GT GLB with Meshopt decoding. The model represents a real production vehicle. Its original geometry and texture coordinates are decoded into floating-point attributes before transforms are applied. Body normals are smoothed for the studio finish. Numerous small wheel meshes are consolidated by material and wheel position. The source does not supply an articulated door rig: the demo separates a driver-door surface region and adds a custom hinge, so this is a visual prototype rather than a factory-accurate rig.

The demo includes procedural body/tyre dirt, finish and glass changes, hood/side water beads, falling rain, full-body film highlights and cabin cameras. GSAP handles DOM reveals; Lenis provides desktop wheel smoothing. Accessible HTML controls drive the scene. These effects illustrate services and do not measure product performance. No unavailable taste-skills package or separate Three UI framework is claimed.

The garage is a procedural interpretation of the customer photo: black block walls, strip lights, shelves, bottles, a workbench and red/grey checker flooring. Blank studio images from all sides remain pending.

## Boundaries and decisions

This is a client-approval landing-page prototype. No booking submission, payment, CRM, customer data or analytics integration is included. No price, warranty, measured protection level or waterproof claim is made. Tint is included at the user's request; final service availability, packages and copy require client approval. VLT appearance is illustrative and is not installation/legal guidance.

The 2004 Porsche Carrera GT asset by Ddiaz Design is attributed under CC BY 4.0 in the footer and asset register. Manufacturer affiliation is not implied. Model size is approximately 5.94 MB. No mobile FPS, Core Web Vitals or production-device performance target is claimed. Desktop shadows are disabled on narrow screens and pixel ratio is capped. The scene pauses when offscreen, hidden, behind the portfolio dialog or manually paused.

Photo fallback handles a missing model or unavailable/lost graphics context. Service information and booking links remain usable without JavaScript. Context restoration has a handler but is not included in the current successful test coverage.

The original referenced 16-page SOW was not present in this workspace. This implementation does not claim SOW reconciliation. Earlier DOCX/PDF approval documents describe an older concept and have not been updated by this demo iteration.

## Evidence

See QA_RESULTS.md and verification/studio-checks.json for current checks. ASSET_SOURCES.md records sources and modifications. Earlier implementation files and obsolete acceptance results have been removed from the active package; an internal backup is retained under work/ outside this deliverable.

## Latest concept refinements

- Closer arrival, exterior and ceramic cameras; separate Side panels and Hood close-up ceramic controls.
- Dirt is applied to all four tyre materials and wheel trim with the same before/after amount as the body. Clean mode restores dark rubber.
- Rain and runoff use a motion clock that stops while paused or in reduced-motion mode. This is an illustrative particle effect, not a fluid-dynamics or coating-performance test.
- Window glass is separated from lamp lenses for tint. The opening-door surface segmentation remains approximate.
- Blank customer-studio photographs from all sides are expected from the user. Integrating those images remains pending; the current procedural garage is provisional.
