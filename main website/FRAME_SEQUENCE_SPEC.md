# Home story as a photographic frame sequence

The home page can play a pre-made image sequence (AI-generated frames, stitched) instead of the real-time 3D scene. Drop the frames and a `manifest.json` into `assets/frames/` and the site switches over by itself; delete `manifest.json` and the 3D scene comes back. Nothing in the words, dots, buttons or scroll behaviour changes.

Status: **player built and tested; slicer tool added and tested with a reference clip (2026-10-28). No real frames exist yet.** Do not describe AI-generated frames as footage of real work; the story now swaps its small print to "AI-generated illustration of our process, not footage of a customer’s vehicle" whenever frames are playing.

## How it plays
Scrolling moves a playhead through the sequence. Each of the ten story steps has a frame number (`keys`); the playhead rests exactly on that frame at the step and travels between the numbers while the visitor scrolls. The next frame is cross-faded in, so even 150 to 240 frames feel smooth. Frames load progressively (step frames first), and the nearest loaded frame shows while the rest arrive.

## The nine steps (showroom style, 2026-10-28)
The client's reference replaced the ten-step process story: one car under studio lights changes as the visitor scrolls. Step keys, page words and the clip for each are in `frames-brief/VEO_PROMPTS.md`:
`clear`, `glass` (window tint, bright studio), `tint50`, `tint35`, `ppf` (exploded film panels, dark studio), `satin`, `nardo`, `candy`, `shift` (colour-change wraps). After the film, "Try it on" (wrap colours) and "Find your shade" (Clear / 50% / 35%) cards reuse the frames at those keys. Without a published sequence the studio photo stands in and the cards are hidden; the real-time 3D scene is no longer used on the home page.

## Files
```
assets/frames/manifest.json
assets/frames/desktop/0000.webp ... (landscape, 16:9)
assets/frames/mobile/0000.webp  ... (optional, portrait 4:5, same frame count and same key numbers)
```
Targets: desktop 1600 x 900 WebP quality ~78 (about 60 to 110 KB each; 240 frames is roughly 20 MB); mobile 900 x 1125 (about 40 to 70 KB each). Zero-padded sequential numbers, no gaps.

## Slicing the finished video (no ffmpeg needed)
Open `tools/frame-slicer.html` in Chrome or Edge, choose the finished video, set the number of frames (240 is a good default), optionally type where each story step rests (`foam = 8s`, a frame number, or `end`), then press **Choose frames folder and export** and pick `main website/assets/frames`. It writes `desktop/`, `mobile/` and `manifest.json`. Then run `python build-pages.py`, rebuild and restart the web app; the home story switches from 3D to the film by itself, and its small print changes to the AI-film wording.

Tested 2026-10-28 (runtime, this machine): a 40 s 1280 x 720 H.264 reference clip sliced into 240 desktop + 240 phone frames in 84 s (19 MB + 11 MB), and the home story played them in step with the scroll. The test frames were deleted; that clip is reference only and must not ship (it shows a real branded car and a store sign).

## Making the AI video
Prompts, clip order, studio descriptions and slicer step positions: `frames-brief/VEO_PROMPTS.md` (Google Veo, the Dodge Charger Daytona from the user's reference clip, nine 5 s clips). The earlier ten-step process prompts were retired when the client chose the showroom style.

## manifest.json
```json
{
  "version": 1,
  "pad": 4,
  "desktop": { "dir": "desktop/", "ext": "webp", "count": 240 },
  "mobile":  { "dir": "mobile/",  "ext": "webp", "count": 240 },
  "keys": { "clear": 0, "glass": 53, "tint50": 80, "tint35": 106, "ppf": 133, "satin": 159, "nardo": 186, "candy": 212, "shift": 239 }
}
```
- `mobile` is optional (phones fall back to `desktop`). `keys` may be left out for any step; it is then spread evenly.

## Things to decide before generating
1. The vehicle: the user chose the Dodge Charger Daytona (2026-10-28). Keep its badges unreadable and show no plate; the footer already states that manufacturer affiliation is not implied.
2. No fake customer, plate, logo or invented credentials in the frames; the Dapper sign and shop must match the real shop photos.
3. Wording near the player switches automatically to the AI-film note; check it still matches what the frames show.
