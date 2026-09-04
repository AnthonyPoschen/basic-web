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

## Website best practices

Consuming apps follow `docs/best-practices/README.md`. Each file is one
load-path lesson (deferred scripts, route registration, CSS split, imports,
third-party JS, cache, LCP, HTTP/2). When a Lighthouse finding, console error,
or production incident teaches a repeatable rule, add or update a file in that
tree and link it from the README. Do not leave the lesson only in chat.
