# Veo prompts for the home hero film (9 clips × 5 s, showroom style)

Decided 2026-10-28 by the user: the hero follows the client's showroom reference (a car under studio lights that changes as you scroll, then "Try it on" and "Find your shade" cards). The car is the **Dodge Charger Daytona** from the user's earlier clip, in every chapter, generated with **Google Veo**. Dapper offers colour-change wraps (confirmed by the user). Neither reference video is used on the site.

The home story (`dapper-platform/apps/web/app/(site)/_components/Story.tsx`) has nine steps; each clip below ends on one of them.

| Clip | Step key | What the page says | Words sit |
|---|---|---|---|
| 1 | `clear` | See your tint before it's on. | left |
| 2 | `glass` | Cut for your exact glass. | right |
| 3 | `tint50` | 50%. Darker, still clear. | left |
| 4 | `tint35` | 35%. More privacy. | right |
| 5 | `ppf` | Every panel, cut to fit. | left |
| 6 | `satin` | Satin black. | right |
| 7 | `nardo` | Nardo gray. | left |
| 8 | `candy` | Candy red. | right |
| 9 | `shift` | Colour-shift. | left |

## Before you start
- **Reference images:** add two or three stills from `veo-reference/` to every clip so the car stays the same.
- **Chaining:** clips 2–4 start from the last frame of the clip before; clips 6–9 likewise. Clip 5 opens the second studio, so start it fresh from the reference images (clip 4 ends on dimming lights and clip 5 starts from near-black, which hides the change of room).
- **Length and format:** 5 s each (use 6 s if 5 is not offered), 16:9, 1080p or higher, 24 fps. Sound is ignored.
- **Words side:** keep the third of the frame where the words sit (see the table) dark and simple; keep the car near the middle overall so the phone crop still shows it.
- **Badges and plates:** badges unreadable, no plate, so nothing suggests Dodge endorses Dapper.

## Shared lines (paste at the end of every prompt)
Car:
```text
Same Dodge Charger Daytona four-door as the reference images: gloss-black hood and roof accents, black five-spoke wheels, full-width white LED light bar across the front and full-width red light bar across the rear, no licence plate, badges not readable. Photorealistic automotive commercial, full-frame cinema camera, 35 mm lens, smooth slow gimbal movement, no cuts, real reflections on the paint. No people, no on-screen text, no captions, no watermarks, no logos.
```
Studio A (clips 1–4):
```text
Bright modern detailing studio: white walls, black-framed glass roll-up doors, long linear LED light bars across a black ceiling, polished light-gray epoxy floor with clear reflections, black wall cabinets.
```
Studio B (clips 5–9):
```text
Dark detailing studio: matte black walls, long linear LED light bars across the ceiling, glossy dark epoxy floor with reflections, a tall rack of wrap vinyl rolls in many colours at the left edge.
```

## Negative prompt (if there is a field for it)
```text
text, captions, subtitles, watermark, logo, licence plate, readable badge, people, hands, extra wheels, warped body panels, melting reflections, flicker, camera shake, jump cut, zoom jump
```

## The nine clips
1. **clear** (+ car line + Studio A)
```text
The car is parked in the studio in its original matte olive-green paint, side windows completely clear and untinted so the tan interior is easy to see. The camera orbits slowly from the side three-quarter toward the front three-quarter at door height. Keep the car in the right two-thirds of the frame and the left third calm.
```
2. **glass** (+ car line + Studio A)
```text
Thin glowing cyan outlines trace the edges of each side window one by one, and translucent cyan film shapes cut to each window lift a few centimetres off the glass, hover, then settle back onto it. The glass stays clear. The camera continues its slow orbit to the front three-quarter. Keep the car in the left two-thirds and the right third calm.
```
3. **tint50** (+ car line + Studio A)
```text
The side and rear windows darken smoothly and evenly from clear to a medium smoked tint, about 50 percent, the interior still visible through it. The camera moves slowly to the side profile. Keep the car in the right two-thirds and the left third calm.
```
4. **tint35** (+ car line + Studio A)
```text
The windows darken further to a deeper smoked tint, about 35 percent, the interior only faintly visible. The camera orbits slowly to the rear three-quarter, then the studio lights dim gently toward near-black in the last second. Keep the car in the left two-thirds and the right third calm.
```
5. **ppf** (+ car line + Studio B; start from the reference images, not the previous clip)
```text
Starting from near-black, the ceiling light bars come up on the car in its olive paint with dark tinted windows. Glowing cyan holographic outlines appear around every body panel (doors, fenders, hood, bumpers, roof) and the clear protection-film panels lift off the car as a floating exploded view, then slide back and seal onto the paint. The camera orbits slowly at the front three-quarter. Keep the car in the right two-thirds and the left third calm.
```
6. **satin** (+ car line + Studio B)
```text
A colour-change wrap sweeps over the car from front to back like a wave, turning the olive paint into smooth satin black with a soft low sheen. The camera tracks slowly along the side. Keep the car in the left two-thirds and the right third calm.
```
7. **nardo** (+ car line + Studio B)
```text
The satin black wrap is replaced front to back by a solid flat Nardo gray wrap with a light gloss, the change moving across the panels like a wave. The camera orbits slowly toward the rear three-quarter. Keep the car in the right two-thirds and the left third calm.
```
8. **candy** (+ car line + Studio B)
```text
The gray wrap is replaced front to back by a deep glossy candy-apple red wrap, highlights from the ceiling bars running over the curves. Low camera at the front three-quarter, slow push-in. Keep the car in the left two-thirds and the right third calm.
```
9. **shift** (+ car line + Studio B)
```text
The red wrap is replaced front to back by a colour-shift chameleon wrap that moves from deep purple to teal as the camera orbits; the camera completes a slow orbit to the front three-quarter and holds still for the last second. Keep the car in the right two-thirds and the left third calm.
```

## After Veo
1. Join the nine clips in order into one video (no transitions).
2. Open `../tools/frame-slicer.html`, choose the video, 240 frames, and paste these step positions (end of each clip):
```text
clear = 0s
glass = 10s
tint50 = 15s
tint35 = 20s
ppf = 25s
satin = 30s
nardo = 35s
candy = 40s
shift = end
```
For 6 s clips: `clear = 0s`, `glass = 12s`, `tint50 = 18s`, `tint35 = 24s`, `ppf = 30s`, `satin = 36s`, `nardo = 42s`, `candy = 48s`, `shift = end`.
3. Export into `main website/assets/frames`, run `python build-pages.py`, rebuild and restart the web app. The film, the chip, and the "Try it on" / "Find your shade" cards all come from these frames; no other pictures are needed.
