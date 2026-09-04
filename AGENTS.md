# AGENTS.md

Basic Web is a Go-served custom-element runtime used by more than one website.
Read `README.md` for setup and conventions. Read `docs/performance.md` before
changing how HTML, CSS, JavaScript, the element manifest, or cache headers are
served or loaded.

Those behaviors exist to keep first paint and LCP fast when apps keep source
files split by concern. Do not merge CSS in source, split `/framework/basic-web.js`
back into separately fetched classic scripts, or drop versioned immutable caching
to make the handler look simpler. The justification for each feature is in
`docs/performance.md`.
