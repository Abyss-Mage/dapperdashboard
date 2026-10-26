# Demo verification — Transformation Studio

Checked 21 October 2026 in local Chromium with Playwright. **47 acceptance checks passed.** Full results: verification/studio-checks.json. No uncaught application errors or failed asset responses were observed during the main test journey.

## Observed behaviour

- Porsche Carrera GT geometry renders; arrival begins with procedural dirt. Detailing Before restores dirt and After removes it.
- Ceramic opens on the side panels and offers a hood close-up and displays 300 hood/side water beads and 180 falling rain droplets. The water-test toggle removes them. Current beads use a physical water-like index of refraction and transmission; visual quality was inspected in screenshots.
- PPF displays film and coverage highlights on the body surfaces. Service copy describes full-body coverage. The highlight toggle and before/after controls work.
- The 20–95% VLT control changes glass opacity; lower VLT is darker. Before returns the untreated glass appearance.
- Middle-button dragging orbits the normal arrival scene, service scene and interior scene. Left-button dragging pans the camera. Reset camera and composed camera buttons remain available.
- Unlock opens a custom segmented driver-door surface on a demo hinge; close returns it. This is not an authored or factory-accurate articulation. Driver and seat viewpoints were visually inspected.
- Keyboard service navigation updates selection and focus. Native range controls support keyboard adjustment. The portfolio dialog supports navigation, Escape and focus restoration. FAQ disclosures expand.
- Booking buttons point to Dapper's existing booking route. No live booking or payment was submitted.
- Manual pause stops the continuous render loop; controls still draw static changes. Reduced-motion mode starts paused and retains service controls.
- Graphics-context loss displays photo fallback. A deliberately blocked GLB also preserves content and booking. No-JavaScript content remains available.
- No document overflow was observed at 320, 390, 768, 1024 or 1440 CSS pixels. Separate mobile 390×844 and tall-panel 398×1724 journeys passed, including tint controls.

## Visual review

Inspected arrival, clean finish, ceramic hood, PPF, tint, door, driver view, seats, and mobile/tall-panel screenshots. Corrected model quantization, camera clipping, cabin text readability and mobile service positioning during development. Current screenshots are supplied alongside the source package.

## Limits

This is browser-level prototype acceptance, not a production certification. Physical iOS/Android devices, Safari/Firefox, assistive-technology audits, measured Core Web Vitals, sustained GPU/FPS profiling, tint installation suitability, product warranties and model ownership verification were not tested. Context-restoration handling exists but successful recovery is not part of these acceptance results. The service visuals are illustrative rather than scientifically measured or actual customer before/after evidence.

## Production-vehicle replacement

The concept vehicle was rejected and replaced with the attributed 2004 Porsche Carrera GT by Ddiaz Design. The replacement has actual exterior, dashboard, seats, lighting and wheel geometry. Body/tyre dirt, water, tint and film controls are adapted to it. All texture-coordinate channels are retained during geometry decoding; numerous small wheel meshes are batched. Paint normals are smoothed for the finish preview. Door surface segmentation is approximate and remains a demo limitation.

The procedural garage awaits the user's blank studio images. Physical-device testing, sustained GPU performance and successful context restoration remain outside these checks.
