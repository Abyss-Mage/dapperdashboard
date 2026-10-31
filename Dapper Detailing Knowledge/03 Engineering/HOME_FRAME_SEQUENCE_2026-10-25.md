# Home story: frame-sequence mode (2026-10-25)

Owner direction: stop hand-building the 3D; produce the home story as AI-generated frames stitched into the site so it looks real.

Built (source + runtime test with placeholder frames; no real frames yet):
- `main website/home-stage.js` picks the player: if `assets/frames/manifest.json` exists it loads `home-frames.js`, otherwise the existing 3D scene (`home-scene.js`) runs.
- `home-frames.js`: canvas player scrubbed by `window.dapperHome.progress`; per-step frame numbers, cross-fade between frames, progressive loading, mobile set, optional tint stills; drag hint hidden in this mode.
- `main website/FRAME_SEQUENCE_SPEC.md`: file format, manifest, step-by-step shot list, sizes, ffmpeg export. `main website/frames-brief/reference/` holds ten 1920x1080 stills exported from the 3D scene as composition guides.
- Placeholder frames used for testing were deleted; the site currently still shows the 3D scene.

Open: generated frames, wording of the "Real-time 3D visualisation" disclaimer, decision on tint stills vs removing shade buttons, vehicle/licence choice. Evidence is runtime test on desktop only.

## 2026-10-28: slicer tool and AI-video brief
- User supplied a 40 s night orbit clip (1280 x 720) as **camera reference only**; the hero will use an AI-generated video instead. The clip shows a real branded car and a store sign and is not stored in the project.
- Added `main website/tools/frame-slicer.html`: a Chrome/Edge page that slices any video into `assets/frames/desktop`, `mobile` (portrait centre crop) and `manifest.json`, with optional step keys by time or frame. No ffmpeg needed.
- Runtime test (this machine): the reference clip became 240 + 240 WebP frames in 84 s (19 MB + 11 MB); the Next.js home story detected the manifest and scrubbed the film with scroll in step with the text. Test frames deleted afterwards.
- The story's small print now switches to "AI-generated illustration of our process, not footage of a customer's vehicle" whenever frames play.
- `FRAME_SEQUENCE_SPEC.md` now holds the per-step AI prompts and production rules (one car from a reference still, ten chained 4–6 s clips, steady orbit, calm text side, no text or logos). No real frames exist yet.
- 2026-10-28 (later): user decided the hero car is the **Dodge Charger Daytona** from their reference clip, generated with **Google Veo** as ten 5 s clips. Prompts: `main website/frames-brief/VEO_PROMPTS.md`; six reference stills of the car (from the clip, background only incidental): `main website/frames-brief/veo-reference/`. Badges unreadable, no plate, no implied manufacturer endorsement.
- 2026-10-28 (later still): the client supplied a second reference (a showroom-style scroll site recorded on a laptop). User decisions: change the **home hero only**; Dapper **does offer colour-change wraps**; the **Dodge** is the car in every chapter. The ten-step process story is retired. The home story now has nine steps (`clear`, `glass`, `tint50`, `tint35`, `ppf`, `satin`, `nardo`, `candy`, `shift`), plays frames only (the 3D scene is no longer loaded on home; the studio photo stands in until frames exist), and is followed by "Try it on" (wrap colours) and "Find your shade" (Clear/50/35) cards that reuse the frames at those keys and stay hidden without a sequence. Headlines are Dapper's own wording, not the reference's. Veo prompts rewritten for nine 5 s clips in `main website/frames-brief/VEO_PROMPTS.md`. Runtime check (dev server, this machine): no-frames state and stand-in-frames state both worked; stand-in frames removed.
