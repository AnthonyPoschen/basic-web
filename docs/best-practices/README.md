# Website best practices

These notes are for apps that use Basic Web, and for agents that change those
apps. Framework *why* lives in [performance.md](../performance.md). This tree
is the *how*: load-path habits that keep first paint, LCP, and routing correct.

When a Lighthouse run, a console error, or a production incident teaches a
repeatable loading lesson, add or update a file here and link it from this
index. Do not leave the lesson only in chat.

## How to add a practice

1. Create `docs/best-practices/<short-name>.md`.
2. Use the same shape as the files already here: problem, practice, anti-pattern,
   how we learned it.
3. Link it from the list below in one sentence that names the failure it prevents.
4. Point consuming-app `AGENTS.md` files at this README so agents look here first.

Keep one concern per file. If two lessons share a cause, keep them together.

## Practices

- [Defer only external classic scripts](defer-external-scripts.md) — `defer` on
  an inline `<script>` is ignored, so it runs before deferred framework files.
- [Register routes in a deferred file](router-registration.md) — `Router.register`
  must run after `/framework/basic-web.js`, in document order.
- [Keep CSS split in source](split-css-source.md) — do not merge stylesheets by
  hand; the framework combines local `index.html` links at serve time.
- [Static imports preload on every page that uses the element](static-vs-dynamic-imports.md) —
  a public header must not `import` dashboard modules.
- [Load third-party scripts when the page needs them](third-party-scripts.md) —
  Clerk (and similar) in `index.html` costs every anonymous visit.
- [Cache versioned framework URLs as immutable](versioned-cache.md) — `?v=`
  already busts the file; a one-day max-age still revalidates it.
- [Revalidate HTML that pins versioned assets](html-revalidate-on-deploy.md) —
  a one-hour HTML `max-age` keeps the previous deploy until a hard refresh.
- [Size and preload the LCP image](lcp-images.md) — a 300KB hero behind six CSS
  files is a 5s LCP on mobile HTTP/1.1.
- [Offer HTTP/2 at the TLS terminator](http2-at-the-edge.md) — browsers speak
  HTTP/1.1 until Gateway ALPN advertises `h2`; the pod can stay HTTP/1.1.
- [Minified HTML may omit `</html>`](minified-optional-html-tags.md) — tdewolff
  drops optional end tags; source can still close `body`/`html`.
- [Unique first-response HTML](seo-first-response.md) — crawlers index the
  first GET, not the hydrated shell; each public URL needs its own title,
  canonical, visible copy, crawlable `href`s, sitemap loc, and public cache.
- [Preserve nested SSR host attributes](nested-ssr-host-attributes.md) —
  rewriting nested custom elements into declarative shadow DOM must keep
  `id`/`class`/`data-*`, or first load throws and stays on the skeleton.
