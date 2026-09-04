# Keep CSS split in source

## Problem

Each `<link rel="stylesheet">` is render-blocking. On HTTP/1.1 an origin has
about six connections, so six CSS files plus classic scripts queue the LCP
image. Empty or comment-only files still cost a round trip. A mobile
Lighthouse run on Genos estimated **1.2s** of render-blocking delay from
separate CSS and classic JS.

Authors still need separate files (`theme.css`, `base.css`, fonts, a classless
vendor sheet). Merging those in git to "fix Lighthouse" fights the way the
repo is organised.

## Practice

Keep global CSS as separate source files and list them from `index.html`.
Basic Web concatenates two or more *local* stylesheet links into one
`/framework/styles.css` (Genos also serves `/styles/bundle.css` until it
bumps the framework). Relative `url(...)` paths are rewritten against the
original file.

Do not concatenate CSS in the application repository as the performance fix.

A nearly empty `layout.css` or `responsive.css` can stay as a placeholder.
The bundle makes the extra file free on the wire.

## Anti-pattern

Do not require apps to ship one hand-maintained `app.css`.
Do not serve each `index.html` stylesheet as its own request in production.
Do not inline the whole bundle into `index.html` by default: the HTML shell is
`no-cache`, the bundled CSS can be versioned and cached.

## How we learned it

Genos `index.html` had seven stylesheets, two of them ~200 bytes. Combining
them at serve time, plus HTTP/2, dropped render-blocking savings from 1,230ms
to 300ms (the remaining one bundle). Unused rules inside the classless sheet
are a separate trade: do not split that sheet unless the product drops
classless styling.

Framework detail: [performance.md](../performance.md).
