# Cache versioned framework URLs as immutable

## Problem

Release HTML requests `/framework/basic-web.js?v=<git sha>`. The query already
names the bytes. If the response still uses `Cache-Control: max-age=86400`
(the middleware default), Lighthouse reports inefficient cache lifetimes and
repeat visits revalidate a file that cannot change without a new `v`.

Unversioned URLs must stay revalidatable. An old HTML shell that omits `?v=`
must not pin a new immutable runtime.

## Practice

When `WebVersion` is a real release value and `?v=` matches it, these
responses should be `public, max-age=31536000, immutable`:

- `/framework/element-manifest.json`
- `/framework/basic-web.js`
- `/framework/styles.css` (and an app bundle such as `/styles/bundle.css`)

Unversioned requests and `dev` use `no-cache`. Application `/scripts/` and
`/styles/` files should use the same rule when they are requested with `?v=`.

Images and fonts that are content-addressed by filename can be immutable
without a query. Changing bytes at the same URL then requires a rename.

## Anti-pattern

Do not leave versioned `/framework/basic-web.js` on the one-day default.
Do not make unversioned framework URLs immutable.

## How we learned it

The first Genos Lighthouse run flagged `basic-web.js?v=<sha>` at a 1-day
lifetime (2KiB wasted). Basic Web now sets immutable for a matching `v`.
Apps still on an older module will keep seeing that finding until they bump.
