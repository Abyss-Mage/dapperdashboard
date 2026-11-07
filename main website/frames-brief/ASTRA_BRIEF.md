# Brief to paste into GPT-6 Astra (hero film planning)

GPT-6 Astra writes text: it plans shots and writes prompts but does not render video itself, so the footage still comes from a video model (Veo here). Paste everything inside the block below into Astra, then attach the six stills from `veo-reference/` and the client's showroom reference video if Astra accepts it.

```text
You are directing a 45-second AI-generated car film for the home page of Dapper Detailing Pros, a vehicle detailing studio in Peterborough, Ontario (window tint, paint protection film, ceramic coating, colour-change wraps). The film is not watched as a video: it is cut into 240 still frames and scrubbed by the visitor's scroll, so it must move slowly, evenly and continuously, with no cuts except one planned room change.

Car: the Dodge Charger Daytona four-door in the attached stills, matte olive-green, gloss-black hood and roof accents, black five-spoke wheels, full-width white LED front light bar, full-width red rear light bar. Same car in every shot. No licence plate, badges not readable (Dapper must not appear endorsed by Dodge). No people, hands only if unavoidable, no on-screen text, logos or watermarks.

Style reference: a showroom-style website where one car under studio lights changes as you scroll (clear glass, then glowing cyan outlines on each window, then tinted glass; then exploded protection-film panels; then wrap colours sweeping over the body).

Structure: nine 5-second clips, rendered in Google Veo, 16:9, 1080p or higher, 24 fps.
Studio A (clips 1-4): bright detailing studio, white walls, black-framed glass roll-up doors, linear LED light bars on a black ceiling, polished light-gray epoxy floor with reflections.
Studio B (clips 5-9): dark detailing studio, matte black walls, linear LED ceiling bars, glossy dark floor, a rack of wrap vinyl rolls in many colours at the left edge.
1 clear: original olive paint, clear untinted windows, slow orbit side three-quarter to front three-quarter. Keep the left third calm.
2 glass: cyan outlines trace each window, film shapes cut to each window lift and settle back; glass stays clear. Keep the right third calm.
3 tint50: windows darken evenly to about 50 percent. Keep the left third calm.
4 tint35: windows darken to about 35 percent; orbit to rear three-quarter; lights dim toward near-black in the last second. Keep the right third calm.
5 ppf: starts from near-black in Studio B; cyan outlines around every body panel, clear film panels lift off as an exploded view and seal back. Keep the left third calm.
6 satin: a wrap sweeps front to back turning the paint satin black. Keep the right third calm.
7 nardo: satin black to solid Nardo gray, same sweep. Keep the left third calm.
8 candy: gray to deep glossy candy red. Keep the right third calm.
9 shift: red to a purple-to-teal colour-shift wrap; finish the orbit at the front three-quarter and hold. Keep the left third calm.
The calm third is where the website's headline sits; keep the car near the middle overall so a portrait phone crop still shows it.

Please give me:
1. A final Veo prompt for each of the nine clips (one paragraph each, including the car, studio and camera move), plus one shared negative prompt.
2. For each clip, its first frame: which clip's last frame to start from (clips 2-4 and 6-9 chain from the previous clip; clips 1 and 5 start from the reference stills) and what that frame must show.
3. A continuity checklist I can use to reject bad takes (car shape, wheel count, light bars, paint, reflections, camera speed, anything readable).
4. If a clip fails twice, a simpler fallback prompt for it.
Do not invent prices, awards, customers or claims about results.
```

## What comes back to the project
- Use Astra's prompts in Veo, chain the clips as it lists, then join the nine clips in order.
- Slice with `../tools/frame-slicer.html` using the step positions at the bottom of `VEO_PROMPTS.md` (`clear = 0s` … `shift = end`).
- If Astra's prompts differ from `VEO_PROMPTS.md` in a way that changes a step (for example a different colour), tell Claude so the page words and chip match.
