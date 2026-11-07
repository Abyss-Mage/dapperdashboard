# ZR1X orbit film (2026-11-05)

Film: https://d2ol7oe51mr4n9.cloudfront.net/user_3GeJ72oSDVkWTuqZa1xeo0TXdsm/c7cce5fd-a7a6-430d-9e78-c667701bcdc9.mp4 (36 s, 1920x1080, 24 fps)
Frames: https://d2ol7oe51mr4n9.cloudfront.net/user_3GeJ72oSDVkWTuqZa1xeo0TXdsm/e75984bb-d98c-4dfb-abe0-8c5716ed2c53.zip
  288 desktop frames 1600x900 + 288 mobile frames 960x540 (16:9; the player fits phones by width) + manifest.json

Chapters (camera orbits 360 degrees the whole way): intro light-up 0-41, detailing 41-82, paint correction 82-123,
PPF 123-164, ceramic 164-205, window tint 205-246, reveal 246-287.

Install (apply together, or the site breaks):
1. Replace the contents of the site's frames folder (/site/assets/frames: desktop/, mobile/, manifest.json) with the zip.
2. dapper-platform/apps/web/app/(site)/home/frameSequence.ts: change `const COUNT = 241;` to `const COUNT = 288;`
3. Replace dapper-platform/apps/web/app/(site)/home/story.ts with story.ts in this folder (draft copy, owner approves).
4. Build and restart; log START/FINISH in PARALLEL_AGENT_COORDINATION.md.
