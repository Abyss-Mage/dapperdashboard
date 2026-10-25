# Home page: real-time 3D scroll story (2026-10-25)

Client direction: the landing page must look and move like the Blackline reference video (sent twice as the "ultimate reference"), but with a **3D animation look, not flat images**.

## What is built (source + build + headless Chrome with GPU; not yet seen on a real phone)
- `main website/home-scene.js`: one three.js scene (our licensed Porsche Carrera GT model) in a dark studio with a polished checker floor and red runway, LED bar ceiling and light blades, RoomEnvironment reflections. The car changes state in real time: road dirt, clean, ceramic gloss with falling rain and beads, body panels lifting apart with cyan cut lines (PPF), window tint by VLT, and paint finishes (satin black, Nardo grey, candy red, colour shift, gloss white). The camera glides between shots and drifts slowly; on wide screens the car is framed to the right of the words, on phones above them.
- `main website/story.js`: GSAP ScrollTrigger pins the stage and scrubs through 11 steps (snap to each); Lenis (`home-motion.js`) makes the scroll fluid. It publishes `window.dapperHome.progress`; the words, status chip (swatch + label + value) and dots follow the active step. The last step, "Make it yours", lets the visitor pick a finish and a tint (70/50/35/20/5% VLT) and the car changes as they click. A bottom bar keeps Instant quote and Book an appointment one tap away.
- Reduced motion: no pin; the 3D stays still and the words scroll over it. If WebGL fails or is lost, a poster image shows.
- The previous 3D page is kept at `/studio/`.
- Checked: real mouse-wheel scrolling moves through all 11 steps with words, chip and car in sync; desktop 1440 and phone 390 render with no sideways scroll.

## Open points
- The colour finishes and tint are visualisations; the active catalogue has no colour wraps and window tint services are archived. Owner to confirm which finishes to show or drop.
- No heat-rejection figures are shown (the reference shows them); only VLT, which is the film level itself.
- The model file is about 5.9 MB; a poster shows while it loads. A lighter model or Draco/meshopt re-compression would help phones.

## Update (2026-10-25, later): coastal mountain road, driving car, alternating words, drag to look around
- Client feedback: the car looked like a toy; wanted realism, interactivity, a better background, the car driving left to right on a mountain road while the camera zooms in on the work, and words alternating left/right each slide. Reference scenery: Big Sur / Sea-to-Sky style coastal mountain roads.
- Background: Poly Haven CC0 HDR "Red Hill Curve" (Cape Town coastal mountain road, ocean, town and peak), downloaded with the owner's approval, projected onto the ground with three.js GroundedSkybox and used for lighting and paint reflections (`assets/env/`, sources in `assets/env/SOURCE.md`; three.js HDRLoader and GroundedSkybox vendored from three@0.186.1). The lane was located from a top-down projection of the HDR; the car drives along it towards the ocean view (reads left to right) and rolls forward as the story advances, wheels turning, after a drive-in entrance.
- Words alternate left/right each step and slide in from their side; the car is framed on the opposite side. Visitors can drag sideways to swing the camera around the car (vertical drags still scroll).
- Open: the car model. The owner asked to use a real-brand car and "not think about licences". I explained that brand collaborations cover the car design, not the 3D artist's copyright in a model file, so a model still needs a licence that allows use. Good free licensed options: Karol Miklas "Porsche 911 (930) Turbo" (CC BY, .glb, Gumroad, free with email) and Lionsharp "Porsche 911 Carrera 4S" (CC BY-SA, Sketchfab login). Both need the owner to download (account/email), then drop the file in; I will wire it in and add the credit.
- Limits: the HDR is a ground-level 360 photo, so close-to-camera ground is soft and the view is from road level, not the aerial vista in the reference images.

## Update (2026-10-25, evening): Mercedes GLS garage sequence (phase 1 of the cinematic)
- Client direction: road-worn car → enters a modification garage → detailing (foam, then pressure rinse) → window tint (glass comes out, visitor picks a shade, glass goes back in with that shade) → ceramic (applicator, then water droplet test) → PPF (film laid on). Phase 2, not started: the car leaves the garage onto the Pacific Coast Highway with a private jet over the ocean and a yacht (owner said to finish the garage first).
- Model: the owner's Mercedes-Benz GLS 580 FBX in the root `3D/` folder (CGTrader upload `uploads_files_2787791`), converted in headless Chrome (three.js FBXLoader → GLTFExporter) and compressed with gltfpack to `assets/three/mercedes-gls.glb`. **Licence of this file not yet confirmed by the owner.**
- 10 steps: road, garage, foam, rinse, tintOut (shade picker 70/50/35/20/5%), tintIn, coat, beads, film, done. A black curtain hides the cut from the road set to the garage set.
- All effects are real-time: road grime + dust layer in the paint shader; snow foam mask; rinse front that sweeps foam and grime off front to back; lifted side/rear panes that read as clear glass and return in the chosen shade; ceramic front with a cyan levelling line and an applicator pad riding on a height map of the bonnet; raycast water beads and a pour; PPF film revealed from the roof down with a cyan squeegee line.
- The rinse, coat and film steps rest part-way through so the visitor sees both halves and the tool on the car; the following step completes it.
- Checked (headless Chrome with GPU, real wheel scrolling, desktop 1440 and phone 390): all 10 steps render in sync with the words, no sideways scroll, no script errors. Not yet seen on a real phone. The two 404s on the home page are `/api/v1/public/sites/dapper` and the booking options: no public site with slug `dapper` is published yet.
- Known rough edges: foam spray is small instanced spheres (reads as droplets up close); hood reflections are the hexagon light grid.
