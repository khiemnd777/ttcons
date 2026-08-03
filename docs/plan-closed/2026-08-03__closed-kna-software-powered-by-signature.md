# [Closed] KNA Software Powered-By Signature

Status: closed

Closed at: 2026-08-03

Verification limitation:

- Browser/UAT was not requested. The implementation was technically verified but remained `NOT UX ACCEPTED` when the plan closed.

Done:

- Completed the mandatory mapping bootstrap, local source review, Review Package and Mockup as Text; the user approved the global scope, footer/sidebar placement, exact label and new-tab link behavior.
- Refined the approved presentation to the exact uppercase `POWERED BY KNASOFTWARE` wording, one non-wrapping 9px line and no external-link icon.
- Added the shared `PoweredBySignature` component with a visible keyboard focus state and a new-tab `https://www.knasoftware.com` link protected by `noopener noreferrer`.
- Rendered the credit in the login card, standard-shell desktop sidebar/mobile footer, project-workspace desktop sidebar/mobile footer and Pipeline Builder template-sidebar footer without changing routes, permissions, API contracts or backend behavior.
- Updated the central module map and frontend UI/utility map for the branding owner and placement contract.
- Current-source verification passed: frontend tests 31/31, TypeScript plus Vite production build, bundle budget (entry 120.3 KiB gzip; 101 JavaScript chunks) and diff whitespace checks.
