# Home story as a photographic frame sequence

The home page can play a pre-made image sequence (AI-generated frames, stitched) instead of the real-time 3D scene. Drop the frames and a `manifest.json` into `assets/frames/` and the site switches over by itself; delete `manifest.json` and the 3D scene comes back. Nothing in the words, dots, buttons or scroll behaviour changes.

Status: **player built and tested with placeholder frames (source/runtime). No real frames exist yet.** Do not describe AI-generated frames as footage of real work; the page already carries "Real-time 3D visualisation… illustrative" wording that must be reviewed and reworded when the frames go live.

## How it plays
Scrolling moves a playhead through the sequence. Each of the ten story steps has a frame number (`keys`); the playhead rests exactly on that frame at the step and travels between the numbers while the visitor scrolls. The next frame is cross-faded in, so even 150 to 240 frames feel smooth. Frames load progressively (step frames first), and the nearest loaded frame shows while the rest arrive.

## The ten steps (frame each one must show)
| # | key | what the frame shows | text side |
|---|-----|----------------------|-----------|
| 1 | `road` | the dirty SUV on the coastal road | text left, car right |
| 2 | `garage` | the car arriving in the Dapper garage | text right, car left |
| 3 | `foam` | snow foam being applied | text left |
| 4 | `rinse` | pressure rinse, half the car clean | text right |
| 5 | `tintOut` | window prep, shade picker on screen | text left |
| 6 | `tintIn` | film applied, glass at the default 50% shade | text right |
| 7 | `coat` | ceramic coating being applied | text left |
| 8 | `beads` | water beading on the coated paint | text right |
| 9 | `film` | paint protection film on the panels | text left |
| 10 | `done` | clean car, door open, "ready for the road" | text right |

Leave roughly the left or right 40% calm (no bright detail) on the side where the words sit, as in the reference stills in `frames-brief/reference/` (exported from the current 3D scene so the composition and camera angles can be matched).

## Files
```
assets/frames/manifest.json
assets/frames/desktop/0000.webp ... (landscape, 16:9)
assets/frames/mobile/0000.webp  ... (optional, portrait 4:5, same frame count and same key numbers)
assets/frames/tint/70.webp 50.webp 35.webp 20.webp 5.webp   (optional, see below)
```
Targets: desktop 1600 x 900 WebP quality ~78 (about 60 to 110 KB each; 240 frames is roughly 20 MB); mobile 900 x 1125 (about 40 to 70 KB each). Zero-padded sequential numbers, no gaps. Keep the vehicle, garage, lighting and camera path consistent across all frames: generate the key frames first, then interpolate between neighbours (first/last-frame video generation or frame interpolation), then export with `ffmpeg`:

```
ffmpeg -i story.mp4 -vf "scale=1600:900" -c:v libwebp -quality 78 desktop/%04d.webp
```

## manifest.json
```json
{
  "version": 1,
  "pad": 4,
  "desktop": { "dir": "desktop/", "ext": "webp", "count": 240 },
  "mobile":  { "dir": "mobile/",  "ext": "webp", "count": 240 },
  "keys": { "road": 0, "garage": 28, "foam": 55, "rinse": 80, "tintOut": 104, "tintIn": 126,
            "coat": 150, "beads": 178, "film": 205, "done": 239 },
  "tint": { "key": "tintIn", "dir": "tint/", "ext": "webp" }
}
```
- `mobile` is optional (phones fall back to `desktop`). `keys` may be left out for any step; it is then spread evenly.
- `tint` is optional: one full-frame still per shade (file names `70`, `50`, `35`, `20`, `5`), the same picture as the `tintIn` frame with that window shade. They cross-fade in when the visitor picks a shade. Without them the shade buttons simply do nothing visible, so supply them or remove the shade buttons.

## Things to decide before generating
1. The vehicle: the Mercedes model's licence is unconfirmed; an AI-generated SUV should not copy a real customer's car or a trademarked design beyond what is already on the road scene.
2. No fake customer, plate, logo or invented credentials in the frames; the Dapper sign and shop must match the real shop photos.
3. Wording near the player ("Real-time 3D visualisation…") must be changed to match what the frames are.
