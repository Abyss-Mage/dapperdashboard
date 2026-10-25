# Home story: frame-sequence mode (2026-10-25)

Owner direction: stop hand-building the 3D; produce the home story as AI-generated frames stitched into the site so it looks real.

Built (source + runtime test with placeholder frames; no real frames yet):
- `main website/home-stage.js` picks the player: if `assets/frames/manifest.json` exists it loads `home-frames.js`, otherwise the existing 3D scene (`home-scene.js`) runs.
- `home-frames.js`: canvas player scrubbed by `window.dapperHome.progress`; per-step frame numbers, cross-fade between frames, progressive loading, mobile set, optional tint stills; drag hint hidden in this mode.
- `main website/FRAME_SEQUENCE_SPEC.md`: file format, manifest, step-by-step shot list, sizes, ffmpeg export. `main website/frames-brief/reference/` holds ten 1920x1080 stills exported from the 3D scene as composition guides.
- Placeholder frames used for testing were deleted; the site currently still shows the 3D scene.

Open: generated frames, wording of the "Real-time 3D visualisation" disclaimer, decision on tint stills vs removing shade buttons, vehicle/licence choice. Evidence is runtime test on desktop only.
