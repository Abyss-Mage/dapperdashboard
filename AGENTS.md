# Dapper engineering entry point

Read `Dapper Detailing Knowledge/00 Home.md`, the base-platform ADR, requirements matrix, open decisions and current implementation status before changing application behavior. The vault is canonical; keep code-specific upstream instructions as technical guidance. Direct user instructions take precedence.

Latest user direction (2026-10-22): build the production-grade application and real workflows/APIs for the future Dapper website. ADR-005 supersedes the demo-product direction. Remove demo routes/branding/JSON sessions from the active application; keep isolated test fixtures and historical evidence separate. Never relabel synthetic records as real operations. Do not use WSL. ERPNext/HRMS imports remain reference material. Use apps/web and apps/api, preserve upstream imports/licenses, and do not claim imported Python modules run inside Go. Do not reuse the rejected generic SaaS starter or claim production acceptance without evidence.

Do not turn unapproved policies, legacy settings or upstream defaults into approved Dapper behavior. No credentials/customer exports in source or Obsidian. Mark source presence, static checks, runtime tests and live provider evidence separately. Do not modify or retire legacy systems. Do not delegate unless the user requests it.
