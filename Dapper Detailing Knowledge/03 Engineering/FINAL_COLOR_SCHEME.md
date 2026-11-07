# Final approved Dapper colour scheme

CONFIRMED direct user instruction, 2026-10-24. This specification supersedes previous colour choices. Implementation is pending; recording approval does not mean the live UI has changed. Retain existing layout, repaired contact-field dimensions, authentication and workflows.

Brand main: #FB1A2B.

## Primary ramp
| Step | Hex |
| --- | --- |
| 50 | #FFF1F2 |
| 100 | #FFD9DC |
| 300 | #FF7B86 |
| 500 | #FB1A2B |
| 600 | #D10F1F |
| 700 | #A30B18 |
| 900 | #4A0509 |

## Tokens
| Token | Light | Dark |
| --- | --- | --- |
| Background / --bg | #FFFAFA | #0E0E11 |
| Surface / --surface | #FFFFFF | #17171B |
| Surface alt / --surface-alt | #FFF1F2 | #1F1F25 |
| Border / --border | #F0DCDE | #2E2E36 |
| Text / --text | #1A0B0D | #F5F5F7 |
| Muted / --text-muted | #6B5558 | #A1A1AB |
| Primary / --primary | #D10F1F | #E5222F |
| Primary hover / --primary-hover | #A30B18 | #C71B28 (decided 2026-10-24; was #FF3B4A, 3.51:1 with white) |
| Primary text / --on-primary | #FFFFFF | #FFFFFF |
| Brand / --brand | #FB1A2B | #FB1A2B |
| Link / --link | #D10F1F | #FF5A67 |
| Dark/light button / --dark-button | #2B2023 | #F5F5F7 |
| Text on dark/light button / --on-dark-button | #FFFFFF | #0E0E11 |
| Outline border and text | #D10F1F | #FF5A67 |
| Gold / --gold | #E8A200 | #FFC247 |
| Gold tint / --gold-tint | #FFF1D0 | #3D2F0A |
| Gold tint text / --on-gold-tint | #6B4400 | #FFD98A |
| Red tint / --red-tint | #FFE3E5 | #3A1519 |
| Red tint text / --on-red-tint | #A30B18 | #FF8F99 |
| Neutral chip background | #ECE6E7 | Not specified by user |
| Neutral chip text | #2B2023 | Not specified by user |
| Sale banner / --banner | #FB1A2B | #C2121F |
| Sale banner text / --on-banner | #FFFFFF, large/bold only | #FFFFFF |
| Success feedback / --success | #15804F (decided 2026-10-24; was #1A8F5A, 3.97:1) | #34D399 |
| Error feedback / --error | #A30B18 | #FF6B77 |
| Warning feedback | Use Gold | Use Gold |

## Required usage

Use brand red for logo/graphics/highlights, not ordinary white-on-brand small button text. Buttons use the specified primary/on-primary pairs. User notes white on raw brand red is approximately4:1 and unsuitable for small text; independently calculate actual pairs before launch. Check muted text, chips, normal/hover button states, outline controls, focus rings, alerts and both themes against their actual backgrounds. Do not claim accessibility certification from supplied estimates. If an exact supplied pair fails, document the measured result and seek a decision rather than silently replacing the approved colour.

Keep red/charcoal/gold palette. Success/error/warning colours are for alerts and form feedback, not decoration. Replace the existing green live shift dot with a neutral or approved semantic treatment; shift state also has explicit text. Do not add decorative gold merely to use every token. Preserve the current theme toggle and persisted preference.

## Implementation handoff

1. Read coordination before edits. Scope final shared tokens across Dapper application; coordinate public/main-website lane before touching their styles.
2. Start with Workspace CSS module, map canvas/ink/muted/line/accent aliases to approved semantic tokens, and distinguish primary button fill from link/icon red in dark mode. Replace old hard-coded hover/feedback/chip/focus values; prevent globals.css dark selectors from overriding scoped approved surfaces/text.
3. Keep contact inputs48px and aligned; no layout redesign or workflow changes.
4. Measure specified contrast pairs, then build/typecheck. Verify actual computed colours and hover/focus in light/dark desktop/mobile; check calendar, walk-in, shift, alerts and invoice surfaces without creating operational records. Reserve guarded build/restart and preserve user draft tabs and any active shift.
5. Update DESIGN.md, acceptance evidence, checkpoint and coordination finish. Keep Master/website work tracked separately if not modified/verified.

Current status: SOURCE APPLIED 2026-10-24 (Claude); NOT YET BUILT, RESTARTED OR VISUALLY VERIFIED. Palette tokens applied to apps/web/app/globals.css (new :root/[data-theme=dark] semantic tokens + legacy aliases), owner/owner.module.css, inline colours in app/**/*.tsx, public/site pages.css/studio.css/studio-scene.js (mirrored to `main website/`); workspace/employee.module.css already carried the tokens. `tsc --noEmit` passes. Pending: guarded build/restart, light/dark desktop/mobile check, decisions below.
Open decisions (palette values as supplied, not substituted): (1) success #1a8f5a on light bg is 3.97:1, below 4.5:1 for small text; (2) dark primary-hover #ff3b4a behind white text is 3.51:1; (3) white on #fb1a2b is 3.97:1, so it is limited to large/bold text; (4) dark neutral chip and the success/notice tints (#eaf6ee/#176238, #17352b) were not specified, so they were derived; (5) public site stays light-paper + dark studio (no theme toggle) and the 3D scene material colours are unchanged except the background/fog, now #0e0e11.

## Contrast decisions — CONFIRMED by user 2026-10-24
Measured WCAG ratios (calculated, not certification):
- Success light: #15804F on #FFFAFA = 4.80 (supplied #1A8F5A = 3.97). #1A8F5A may remain for icons/large graphics only. Dark #34D399 unchanged.
- Dark primary hover: #C71B28 with white = 5.81 (supplied #FF3B4A = 3.51). Dark default #E5222F with white = 4.56 passes. #FF3B4A remains valid only as red text/border on #0E0E11 (5.49).
- White on raw brand #FB1A2B = 3.97: brand red restricted to logo/graphics/large bold (>=18.66px); buttons use --primary. Dark #0E0E11 on brand = 4.85 if small text is ever needed.
- Derived (unspecified) dark neutral chip, success and notice tints retained; per-pair checks still open.
Status: source applied; build/restart see LATEST_CHECKPOINT.
