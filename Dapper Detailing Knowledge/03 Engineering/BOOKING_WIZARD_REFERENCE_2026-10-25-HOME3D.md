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

## Garage rebuilt to match the real shop (2026-10-25)

Source: the owner's 16 photos of the Dapper garage. Procedural only (no photo textures, no copied assets): dark block walls, ribbed grey floor tile with three red bays, hexagon LED ceiling clusters and a wall hex panel, warm vertical tubes, white fluorescents, shelving with jugs/sprays, workbench with bins, shop vacs, generator, loft, chairs, cream sectional roller door, Dapper logo sign.
- The car now drives in forward through the open roller door on arrival; the door closes behind it and re-opens on the "Ready" step.
- Not modelled: motorcycle, GMC truck, hanging props. Car model is still the Mercedes (licence unconfirmed; `3D/` stays untracked).
- Evidence: runtime screenshots (puppeteer, desktop 1440x900) of arrival, garage, coating and ready steps. Phone view and real-device GPU not re-checked after this change.
- Deferred: Pacific Coast Highway scene (jet + yacht), owner's car videos.

## Realism pass (2026-10-25, later)

Owner feedback: the road scene looked broken, the car was not dirty enough (tyres included) and the props, tools and water looked like a game.
- Road: the smeared ground-projected HDR is gone. A procedural asphalt road (aggregate grain, cracks, worn lane paint, tyre-wear paths), gravel shoulders, a galvanised crash barrier and distance fog sit in front of the CC0 coastal HDR sky.
- Dirt: paint gets mud low down, a dust film, thin rain streaks and splatter flecks; tyres and rims get their own dust/caked-mud/brake-dust shader (per wheel, washed by the same rinse front); the glass carries a road film that rinses off too.
- Props: chemical jugs and trigger sprays are lathe-turned bottles with printed (abstract, non-brand) labels, translucent shells and liquid; towel stacks, wash buckets with grit guards, shop vac colour toned down; floor reflects more.
- Tools and water: pressure-washer gun with steel wand and hose; foam lance with a hanging foam bottle; water as streaked fans plus soft mist/droplet sprites; foam as bubble-cluster sprites; ceramic applicator is a suede-wrapped foam block; water beads are flat lenses with highlights and runs; the cyan "game" glow lines are gone.
- Outside the garage door: sky gradient and a blurred tree line instead of a white panel.
- Evidence: runtime screenshots (desktop 1440x900 and 390x844 emulated phone) of all ten steps. Not tested on a real device or GPU other than this machine's.

## Second realism pass (2026-10-25, owner screenshots)
- Windows: the glass no longer lifts out (it read as broken sheets). The tint steps now read "Glass prepped" then "Film applied"; the glass stays in the car and darkens to the chosen shade.
- Wheels: only the tyre and rim spin; every tyre triangle is assigned to its nearest wheel, which removes the stray shards under the tyres and the torn edges at the arches.
- Cubes: props, crates, fixtures and the gun are rounded/beveled or cylindrical; LED tubes are round; door windows reflect the sky.
- Foam: the foam coverage now follows the lance (uniform `uFoamZ`) from the rear to the nose, then the lance keeps working.
- Ceramic: applicator is a suede foam block in a nitrile-gloved hand with a sleeve; the glow lines are gone. Water beads are round mirror-bright domes with highlights; the pour is smooth streams with splash sprites.
- Outside the garage door: the same coastal HDR sky as the road, a street, kerbs, hazy buildings and textured trees.
- Known gap: small jagged teeth remain at the front-left bonnet/fender edge. They come from the Mercedes model's own panel geometry (not from our materials); fixing them needs a cleaner car model.
- Follow-up: the large dark polygon patches on the body were a black inner mesh of the Mercedes model showing through the paint where the two overlap; the paint is now pushed out 7 mm along its normals so it always wins. A horizon haze band hides the seam between the flat ground and the HDR mountains from every drag angle.
- Seam teeth: the zig-zag edges on the bonnet, roof and door handle were the Mercedes file's quantised normals, not missing geometry. Paint, tyre and rim normals are rebuilt with `toCreasedNormals` at load (supersedes the earlier "model geometry" note); the paint is also offset 3 mm and given a depth offset so inner trim never shows through, and the camera near plane is 0.3 m for depth precision. Tyres are darker rubber with lighter dust.
