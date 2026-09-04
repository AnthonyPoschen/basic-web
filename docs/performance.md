# Why Basic Web ships these performance features

This is an explanation for maintainers and coding agents. Consuming sites keep
source files split by concern. Basic Web is responsible for not turning that
authoring shape into extra round trips, render-blocking queues, or stale caches.

Do not remove, bypass, or "simplify" these behaviors because a single page looks
fine on a fast desktop connection. Measure first-load on a throttled mobile
trace, including HTTP/1.1. Several production frontends still reach browsers as
HTTP/1.1 even when the Go process can speak HTTP/2.

The load model is longer than a static HTML page:

```text
HTML
  → render-blocking CSS and classic JS
  → /framework/basic-web.js
  → /framework/element-manifest.json
  → element HTML files
  → element modules and their static imports
  → LCP image / fonts
```

Anything that adds a request to that chain, or that waits for the previous step
before starting the next, shows up as First Contentful Paint and Largest
Contentful Paint delay. Custom-element trees make this worse unless the
framework starts known work in parallel.

## Combine local stylesheets at serve time

**What:** `WithBundledStylesheets` rewrites `index.html` so two or more local
`<link rel="stylesheet">` tags become one `/framework/styles.css` response.
Source CSS files stay on disk at their original paths. Relative `url(...)`
values are rewritten against the original file. The bundle is rebuilt on each
request so `DEV=1` edits still apply.

**Why:** Every stylesheet link is render-blocking. Browsers will not paint until
those files arrive. On HTTP/1.1 an origin is limited to about six connections, so
six CSS files plus classic scripts queue the LCP image behind them. Tiny or
nearly empty files such as a comment-only `layout.css` still cost a round trip.
A 2026 mobile Lighthouse run on a consuming site estimated **1.2s** of
render-blocking delay from separate CSS and classic JS files, while LCP waited
on an image that could not use a connection until those files finished.

Apps should keep CSS split by concern (`theme.css`, `base.css`, fonts, vendor
classless sheets). Merging those files in source is the wrong fix. The framework
combines them for the browser.

**Do not:**

- Require consuming apps to concatenate CSS by hand.
- Serve each `index.html` stylesheet as its own render-blocking request again.
- Inline the whole bundle into `index.html` as the default. The HTML shell is
  `no-cache`; the bundled CSS can be versioned and cached.
- Cache the bundle only at process start in development. Editors expect a CSS
  save to show up after reload.

## Ship one framework runtime file

**What:** `/framework/basic-web.js` is `utils.js`, `loader.js`, and `router.js`
joined in that order.

**Why:** Those three files used to be separate render-blocking scripts. That was
three more critical-path requests and a brittle include order in every
`index.html`. The join is a deliberate payload vs round-trip trade: a few extra
kilobytes once is cheaper than three connections on a mobile HTTP/1.1 path.

**Do not** split the runtime back into separately fetched classic scripts without
a measured replacement (for example a single deferred module).

## Cache versioned framework URLs as immutable

**What:** When `SetupHttpMuxOptions.WebVersion` is a real release value and the
request has a matching `?v=` query, these responses use
`Cache-Control: public, max-age=31536000, immutable`:

- `/framework/element-manifest.json`
- `/framework/basic-web.js`
- `/framework/styles.css`

Unversioned requests, and `dev`, use `no-cache`.

**Why:** The URL already names the bytes. A one-day default cache on a versioned
`basic-web.js` was flagged as wasted repeat-visit work: the file is cache-busted
by `?v=` but the browser still revalidates it the next day. Immutable caching is
safe only because a new deploy changes `WebVersion` and therefore the query.

Unversioned URLs stay revalidatable so an old HTML shell cannot pin a new
immutable file, and so a prior release can still fetch a current manifest if it
omits `?v=`.

**Do not:**

- Apply the default one-day middleware cache to versioned framework URLs.
- Make unversioned `/framework/basic-web.js` immutable. That traps browsers on a
  stale runtime after a deploy.
- Fingerprint by rewriting filenames unless you also update every HTML and
  preload to match. The `?v=` query is the contract consuming apps already use.

## Preload and fetch the element manifest immediately

**What:** The HTML response can send a `Link` preload for
`/framework/element-manifest.json?v=...`. The loader starts that fetch as soon
as `/framework/basic-web.js` runs, without waiting for `DOMContentLoaded`.

**Why:** Route HTML is not in the first document. The next request after the
runtime is the manifest. Starting it from the document and from the runtime
overlaps that fetch with remaining CSS/JS instead of adding it to the end of
the chain.

The preload URL must match the loader request exactly, including `?v=`. A
mismatched preload is a wasted request and can still leave the loader blocked on
a second fetch.

## Fetch the known element tree concurrently

**What:** At startup Basic Web scans `elements/**/*.html` and records:

- element name → source file
- static child custom elements in templates and literal `document.createElement`
- literal local ES module imports (`/`, `./`, `../`)

On hydration it fetches that whole static tree together, then installs
templates and runs scripts. `route-view` keeps `aria-busy` and hides the route
until that tree is defined, then reveals on the next animation frame.

**Why:** Without a manifest, each custom element is a waterfall: fetch the page,
parse it, discover `<site-header>`, fetch that, discover `<catalog-game-card>`,
fetch that. The dependency list exists so those requests start together.

Revealing nodes as they arrive causes layout shift on a fast network (a header
appears, then the hero, then a footer jump). Hiding until the static tree is
ready is a CLS fix, not decoration.

**Do not:**

- Ask consuming apps to maintain a second hand-written manifest or preload list.
- Treat dynamic `import()`, user-specific URLs, or runtime `createElement` names
  as statically known. Guessing those is how you fetch the wrong files.
- Remove the busy/hidden hold to "show something sooner" without measuring CLS
  and the half-rendered route.

## Preload literal module imports with the element

**What:** When the loader is about to fetch an element file, it emits
`modulepreload` for that file's recorded local static imports.

**Why:** A homepage header that `import`s a 50KB dashboard module will otherwise
start that download only after the header HTML has been fetched and its module
has started evaluating. Preload moves the JS request to the same window as the
element HTML. Shared imports are deduplicated so ten elements importing one
helper produce one preload.

Consuming apps should keep rarely needed code behind dynamic `import()` so the
manifest does not preload it on public pages. That is an application split, not
a reason to drop `modulepreload`.

## Start route hydration when `route-view` connects

**What:** `route-view` hydrates from `connectedCallback` via `Router.start()`,
not only from `DOMContentLoaded`.

**Why:** Deferred third-party scripts (auth widgets, analytics) also run after
parse. Waiting for `DOMContentLoaded` puts element fetches behind those scripts.
Starting on connect overlaps the element tree with them.

## Minify and compress in production, not in source

**What:** `memfs.CreateMinifiedFS` minifies HTML, CSS, and JS into memory for
release builds. `CompressHandler` / `CompressFunc` negotiate gzip/brotli.

**Why:** Authors keep readable source. The bytes on the wire should still be
small. Do not require consuming apps to check in minified CSS/JS as the source
of truth.

SSE endpoints must not be compressed. Event streams break if they go through
the generic compressor.

## Default cache is not a substitute for versioned cache

**What:** `Middleware` sets `max-age=86400` only when the handler left
`Cache-Control` empty, and `no-cache` in development.

**Why:** That default is a safety net for forgotten static files. Framework
resources with a release `WebVersion` must set their own headers, as above.
Leaving versioned `/framework/basic-web.js` on the one-day default is the cache
bug this policy exists to avoid.

## What belongs in the consuming app

Basic Web does not choose your images, fonts, or third-party scripts. A
consuming site still needs to:

- Avoid loading dashboard or admin modules from public layout elements.
- Defer or conditionally load third-party JS that is unused on marketing pages.
- Compress and size LCP images; preload only the variant the viewport needs.
- Serve HTTP/2 or HTTP/3 at the edge when the cluster allows it.

Those are not reasons to undo the framework behaviors above. They stack: one
stylesheet, one runtime, a parallel element tree, and then a well-sized LCP
image on a multiplexed connection.
