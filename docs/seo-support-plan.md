# SEO support research and design plan

Status: design draft for future discussion. None of the APIs or behavior proposed in this document is implemented.

Last reviewed: 3 September 2026

## Recommendation

Basic Web should return useful, route-specific HTML on the first HTTP response and then progressively enhance that HTML with the existing custom-element runtime. The server and browser should use one generated route table. Public static routes should be declared next to their page templates, while data-backed routes should use explicit Go renderers.

The response for an indexable URL should contain the page's meaningful public content, not only a shell or component definitions. That includes headings, text, links, images and alt text, and page-specific metadata. Interactive behavior may still load later.

Do not serve a special HTML version only to bots. Google describes bot-specific dynamic rendering as a workaround and recommends server-side rendering, static rendering, or hydration instead ([Google: dynamic rendering](https://developers.google.com/search/docs/crawling-indexing/javascript/dynamic-rendering)). The same useful response should go to people and crawlers.

Declarative Shadow DOM is the leading option for preserving Basic Web's shadow-DOM component model in the server response. It is not yet the final choice. The implementation should first test its browser, crawler, CSS, payload-size, and hydration behavior against a light-DOM alternative.

## Desired outcome

For every public route, a direct request with JavaScript disabled should return:

- The correct HTTP status.
- A unique, descriptive `<title>`.
- A page-specific meta description where one is useful.
- An absolute canonical URL derived from trusted configuration.
- Meaningful semantic content, including the main heading and ordinary `<a href>` navigation.
- Optional `robots` directives and JSON-LD structured data.

After JavaScript loads, the custom elements should add behavior without blanking, duplicating, or visibly replacing the server content. Client-side navigation should remain available and should update the URL, visible page, title, description, canonical URL, and route state together.

This work improves crawlability and indexing inputs. It cannot guarantee indexing, ranking, a particular search-result title, or a rich result.

## Why the current response is weak

Basic Web currently uses an app-shell model:

- [`SetupHttpMuxWithOptions`](../pkg/util/http.go) serves `index.html` for `/` and for non-file paths that do not exist in the web filesystem.
- [`web/index.html`](../web/index.html) registers routes only through calls such as `window.Router.register("/items", "page-items")`.
- [`RouteView.render`](../pkg/util/js/router.js) clears the outlet, creates the selected page element, and waits for the element loader.
- [`ShadowHTMLElement`](../pkg/util/js/utils.js) creates a new open shadow root, adopts a JavaScript-created shared stylesheet, and clones an ordinary `<template>` into that root.
- The example's `:not(:defined) { display: none; }` rule hides undefined custom-element hosts.

A direct request to `/items` therefore returns the same shell as `/`. Its route outlet is empty:

```html
<nav>
  <a href="/">Home</a>
  <a href="/items">Items</a>
</nav>

<route-view not-found="page-home"></route-view>
```

The browser must fetch the element manifest, fetch the element files, install their templates and scripts, define the custom elements, and create their shadow roots before the page content exists. If any of those steps fails, the response can remain blank or nearly blank.

Google handles JavaScript pages in separate crawling, rendering, and indexing phases. App-shell pages need the rendering phase before Google can see their generated content, and a page can wait in the render queue. Google recommends server or pre-rendering because it is faster for people and crawlers and because not every bot runs JavaScript ([Google: JavaScript SEO basics](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics)).

There is also a route-status problem. An unknown non-file path currently receives `index.html` with `200 OK`, and `route-view` falls back to `page-home` in the example. Google defines an error-like, empty, or missing page returned with `200` as a possible soft 404 and recommends `404` or `410` for unavailable content ([Google: soft 404 errors](https://developers.google.com/search/docs/crawling-indexing/troubleshoot-crawling-errors#soft-404-errors)).

## Research findings

### Return content before JavaScript

Google can execute JavaScript, but its first crawl parses the HTTP response and later rendering executes the application. A `200` response is normally queued for rendering, while non-`200` responses may skip rendering. Server-rendered or pre-rendered content avoids making essential content depend on that later work ([Google: JavaScript SEO basics](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics)).

This supports a simple framework contract: public content must be present in the initial response. JavaScript may enhance it, but JavaScript should not be the only way to create it.

### Keep real links and real URLs

Google generally discovers links from `<a>` elements with resolvable `href` attributes. Script-only pseudo-links are unreliable. Descriptive anchor text also helps people and search engines understand the target ([Google: link best practices](https://developers.google.com/search/docs/crawling-indexing/links-crawlable)).

Basic Web already uses the History API and intercepts same-origin anchor clicks. That is compatible with this requirement as long as rendered pages keep ordinary links and every linked path also works as a direct HTTP request.

### Web components can be indexed, but only after they are visible

Google supports web components. During rendering it flattens visible light- and shadow-DOM content. Content absent from the rendered HTML cannot be indexed ([Google: web components](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics#web-components)).

An ordinary `<template>` does not solve the initial-content problem. A template holds a fragment for later cloning and insertion; its contents are not rendered as part of the document ([WHATWG: the `template` element](https://html.spec.whatwg.org/multipage/scripting.html#the-template-element)). Returning all component source files or copying their ordinary templates into `index.html` would still leave the visible route empty.

Declarative Shadow DOM changes that behavior. When the HTML parser encounters a `<template shadowrootmode="open">` under a valid host, it attaches a shadow root and parses the template content into it ([WHATWG: `shadowrootmode`](https://html.spec.whatwg.org/multipage/scripting.html#attr-template-shadowrootmode)). This lets a server return rendered, encapsulated component content before the component class is defined.

### Hydration must preserve the declarative root

Calling `attachShadow()` on a host that already has a declarative shadow root with the same mode returns the existing root, but first clears it. Calling it with an incompatible root throws ([MDN: `attachShadow()` on an existing shadow host](https://developer.mozilla.org/en-US/docs/Web/API/Element/attachShadow#calling_this_method_on_an_element_that_is_already_a_shadow_host)).

Basic Web therefore cannot run its current constructor unchanged over a server-rendered host. `ShadowHTMLElement` must detect and reuse an existing root without calling `attachShadow()` or cloning the template again. An open root is available through `this.shadowRoot`; a closed-root design would need `ElementInternals.shadowRoot` or component-owned access. Basic Web already uses open roots, so open Declarative Shadow DOM is the smaller change.

The words `open` and `closed` control JavaScript access to the root. They do not remove shadow-DOM CSS scoping. Page rules normally do not select nodes inside either kind of shadow tree ([MDN: shadow DOM and CSS encapsulation](https://developer.mozilla.org/en-US/docs/Web/API/Web_components/Using_shadow_DOM#elementshadowroot_and_the_mode_option)).

### `<noscript>` is not a second page format

The HTML standard says that `<noscript>` represents nothing when scripting is enabled and represents its children when scripting is disabled ([WHATWG: `noscript`](https://html.spec.whatwg.org/dev/scripting.html#the-noscript-element)). It is useful for a real no-JavaScript warning or fallback, such as an alternate way to submit an interactive form.

It should not contain a manually maintained duplicate of every page. That would create two content trees that can drift, would not fix the blank state for JavaScript-enabled users, and would treat SEO as a crawler-only variant. The ordinary response should be useful whether or not scripts run.

### Status and metadata are route data

Unknown resources should return a real `404`; removed resources may return `410`; a permanent replacement should use `301` ([Google: crawl errors and redirects](https://developers.google.com/search/docs/crawling-indexing/troubleshoot-crawling-errors#soft-404-errors)). A client-only not-found component cannot repair an incorrect HTTP status already sent by the server.

Google uses the `<title>`, the visible main title, headings, and other signals when it creates a search-result title. Titles should be descriptive and distinct, but Google can choose a different result title ([Google: title links](https://developers.google.com/search/docs/appearance/title-link)). Google usually builds a snippet from page content and may use the meta description when that description better summarizes the page ([Google: snippets](https://developers.google.com/search/docs/appearance/snippet)).

Canonical signals should agree. Google recommends putting an absolute `rel="canonical"` URL in source HTML, using self-referential canonicals on canonical pages, linking internally to canonical URLs, and listing canonical URLs in the sitemap ([Google: canonical URLs](https://developers.google.com/search/docs/crawling-indexing/consolidate-duplicate-urls)). Canonicals should come from an application-configured site origin, not an untrusted request `Host` or forwarding header.

### `robots.txt`, `noindex`, and sitemaps have separate jobs

`robots.txt` controls crawling. It is not a reliable way to remove a URL from search results. Use authentication for private content or a `noindex` meta/header directive for a public page that must not be indexed. The crawler must be allowed to fetch the page to see `noindex` ([Google: `robots.txt`](https://developers.google.com/search/docs/crawling-indexing/robots/intro), [Google: robots meta directives](https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag)). Do not block CSS or JavaScript that a crawler needs to understand a page.

A sitemap should contain fully qualified canonical URLs that the site wants in search. Sitemap submission is a hint, not a guarantee of crawling or indexing. A root-level sitemap may be advertised with a `Sitemap:` line in `robots.txt` ([Google: build a sitemap](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap)).

Basic Web already returns a permissive default `robots.txt` and advertises a static `sitemap.xml` when that file exists. The generated URL currently uses request headers through `requestOrigin`. SEO support should use the configured canonical site origin for both canonicals and generated sitemap references.

### Structured data is optional enhancement

Google recommends JSON-LD when it fits the site. Structured data must describe visible page content and follow the rules for its specific result type. Valid markup makes a page eligible for a rich result; it does not guarantee one ([Google: structured-data introduction](https://developers.google.com/search/docs/appearance/structured-data/intro-structured-data), [Google: structured-data guidelines](https://developers.google.com/search/docs/appearance/structured-data/sd-policies)).

The first SEO release can provide a safe JSON-LD injection point without trying to generate schemas automatically. Schema types and fields belong to the application because only the application knows what the page represents.

## Proposed author experience

The shortest static-page workflow should keep the route beside the page that owns it. The following attribute names are a draft:

```html
<!-- web/elements/pages/page-home.html -->
<template
  id="page-home"
  data-route="/"
  data-title="Acme Gardening"
  data-description="Simple gardening advice for Australian backyards."
>
  <main>
    <h1>Acme Gardening</h1>
    <p>Simple gardening advice for Australian backyards.</p>
    <a href="/about">About our gardeners</a>
  </main>
</template>

<script>
  customElements.define(
    "page-home",
    class extends ShadowHTMLElement {
      constructor() {
        super("page-home");
      }
    },
  );
</script>
```

```html
<!-- web/elements/pages/page-about.html -->
<template
  id="page-about"
  data-route="/about"
  data-title="About us | Acme Gardening"
  data-description="Meet the Sydney gardeners behind Acme Gardening."
>
  <main>
    <h1>About Acme Gardening</h1>
    <p>We have helped Sydney gardeners since 2012.</p>
    <a href="/">Read our gardening advice</a>
  </main>
</template>

<script>
  customElements.define(
    "page-about",
    class extends ShadowHTMLElement {
      constructor() {
        super("page-about");
      }
    },
  );
</script>
```

The author would no longer repeat these public routes in `index.html`:

```html
<script>
  Router.register("/", "page-home");
  Router.register("/about", "page-about");
</script>
```

Those registrations remain necessary as framework behavior. Basic Web generates them from the page definitions. Explicit `Router.register()` should remain as a backward-compatible escape hatch for client-only routes, but a browser-only registration cannot be server-rendered, receive a reliable direct-request status, or be generated into a sitemap.

## Treat `index.html` as the shared document shell

`index.html` remains the source for markup shared by all routes. It gains explicit injection points for route data. These marker names are also drafts:

```html
<!doctype html>
<html lang="en-AU">
  <head>
    <title data-basic-web-title>Acme Gardening</title>
    <meta name="description" content="" data-basic-web-description>
    <meta name="robots" content="index,follow" data-basic-web-robots>
    <link rel="canonical" href="" data-basic-web-canonical>

    <link rel="stylesheet" href="/styles.css">
    <script src="/framework/basic-web.js"></script>
  </head>
  <body>
    <nav>
      <a href="/">Home</a>
      <a href="/about">About</a>
    </nav>

    <route-view data-basic-web-outlet></route-view>
  </body>
</html>
```

The file is a layout, not the unchanged final response. On `GET /about`, the Go handler matches `/about`, fills the marked head elements, recursively renders `page-about`, inserts it into the outlet, and writes the route's status.

The handler must parse and modify HTML structurally. Regex replacement is not safe for nested markup, escaping, duplicate elements, or script/style contents.

## Generate one route table for Go and the browser

At startup, Basic Web should extend its current element scan to build one validated site model. The route portion could serialize in the existing element manifest like this:

```json
{
  "elements": {
    "page-home": "pages/page-home.html",
    "page-about": "pages/page-about.html"
  },
  "dependencies": {},
  "moduleImports": {},
  "routes": [
    {
      "pattern": "/",
      "element": "page-home",
      "meta": {
        "title": "Acme Gardening",
        "description": "Simple gardening advice for Australian backyards."
      }
    },
    {
      "pattern": "/about",
      "element": "page-about",
      "meta": {
        "title": "About us | Acme Gardening",
        "description": "Meet the Sydney gardeners behind Acme Gardening."
      }
    }
  ]
}
```

The in-memory form powers server matching and rendering. The serialized form powers browser navigation and effectively makes the same internal `Router.register()` calls. Neither side should parse route declarations from arbitrary JavaScript.

Startup validation should reject:

- Duplicate or ambiguous route patterns.
- A `data-route` template whose `id` is not a valid custom-element name.
- Route metadata that cannot be represented safely.
- Missing required shell injection points when SEO rendering is enabled.
- Cycles that make recursive static rendering impossible.
- A static and Go renderer claiming the same pattern without explicit precedence.

Static routes can be enumerated into a sitemap. Parameterized patterns such as `/items/:uuid` cannot: the framework needs the application's real list of canonical item URLs.

## Recursively render the selected element tree

Rendering only the outer page template is insufficient when meaningful content belongs to a nested custom element:

```html
<template id="page-home" data-route="/" data-title="Acme Gardening">
  <main>
    <h1>Acme Gardening</h1>
    <feature-list></feature-list>
  </main>
</template>
```

```html
<template id="feature-list">
  <section>
    <h2>What we offer</h2>
    <ul>
      <li>Garden design</li>
      <li>Lawn maintenance</li>
      <li>Native plant advice</li>
    </ul>
  </section>
</template>
```

For the Declarative Shadow DOM candidate, the response would contain an instance of every nested element and an instantiated shadow root for each instance:

```html
<route-view data-basic-web-server-rendered data-route-path="/">
  <page-home data-basic-web-server-rendered>
    <template shadowrootmode="open">
      <main>
        <h1>Acme Gardening</h1>
        <feature-list data-basic-web-server-rendered>
          <template shadowrootmode="open">
            <section>
              <h2>What we offer</h2>
              <ul>
                <li>Garden design</li>
                <li>Lawn maintenance</li>
                <li>Native plant advice</li>
              </ul>
            </section>
          </template>
        </feature-list>
      </main>
    </template>
  </page-home>
</route-view>
```

The recursive renderer needs an explicit policy for attributes, slots, repeated elements, missing element definitions, recursion depth, and components whose initial content comes from JavaScript. It must never execute arbitrary browser JavaScript on the Go server.

The renderer should render only public, deterministic initial content. Personalized or authenticated state does not belong in a cacheable public response.

## Hydrate instead of rebuilding

On the first route load, `route-view` should recognize that its content matches the current route and keep it in place. It should load the page's element tree without calling `replaceChildren()` and without applying the current loading style to already rendered content.

The base element behavior should follow this shape:

```js
class ShadowHTMLElement extends HTMLElement {
  constructor(templateID) {
    super();

    const root = this.shadowRoot ?? this.attachShadow({ mode: "open" });
    root.adoptedStyleSheets = [window.globalSheet];

    if (!root.hasChildNodes()) {
      const template = document.getElementById(templateID);
      root.appendChild(template.content.cloneNode(true));
    }
  }
}
```

This is illustrative, not production code. Production hydration needs a reliable server-rendered marker or checksum and a mismatch policy. A mismatch could discard the server tree and client-render, log in development, or force a full navigation. The framework should not silently combine incompatible trees.

After the initial load, client-side navigation can keep the current constructor-based rendering path. It must also update route metadata and restore it correctly for back/forward navigation.

## Solve CSS before choosing Declarative Shadow DOM

Declarative `open` roots have the same CSS boundary as the current imperative open roots. The problem is timing, not root mode.

Today, JavaScript creates `window.globalSheet` from accessible document stylesheets and assigns it through `shadowRoot.adoptedStyleSheets`. A constructed stylesheet can be shared by several roots, but it must be created in the root's parent document ([MDN: `adoptedStyleSheets`](https://developer.mozilla.org/en-US/docs/Web/API/ShadowRoot/adoptedStyleSheets)). Before JavaScript runs, a server-created shadow root cannot use that JavaScript-created sheet. The content may be visible but unstyled.

Candidate style strategies are:

1. Put a shared `<link rel="stylesheet">` in each declarative root, then remove or supersede it when the constructed sheet is adopted.
2. Inline the shared rules in each root. Compression reduces transfer cost, but the uncompressed document and browser parsing cost grow with every component instance.
3. Generate one server-known component stylesheet per root and reserve document styles for inherited values and CSS custom properties.
4. Render semantic light DOM first and add shadow behavior later. This gives the broadest source-HTML compatibility but changes component CSS and slot semantics.

The implementation should prototype and measure options 1 and 4 before choosing. It should verify first paint, no-JavaScript rendering, CSP behavior, cache behavior, and a page with many repeated components.

The example's broad rule also needs to change:

```css
:not(:defined) {
  display: none;
}
```

That rule hides a host even when it already contains useful declarative shadow content. Basic Web should own a targeted loading state for client-only elements. Server-rendered hosts and `route-view` must not receive a rule that hides their initial content.

## Add explicit renderers for dynamic pages

A static template can describe `/about`, but it cannot provide the current title, existence, and content of `/items/:uuid`. That route needs application data before the server can return correct HTML or a correct status.

The framework should accept a Go renderer keyed to the same route pattern. A possible API shape is:

```go
// Design draft only.
type PageResult struct {
    Status          int
    Title           string
    Description     string
    CanonicalPath   string
    Robots          string
    Content         template.HTML
    StructuredData  json.RawMessage
}

// Design draft only.
type PageRenderer func(context.Context, RouteRequest) (PageResult, error)

util.SetupHttpMuxWithOptions(mux, webFS, util.SetupHttpMuxOptions{
    SiteOrigin: "https://acmegardening.example",
    PageRenderers: map[string]util.PageRenderer{
        "/items/:uuid": renderItemPage,
    },
})
```

The exact Go API needs separate design. In particular, `template.HTML` is a sharp trust boundary. A safer renderer might return typed metadata plus a framework-owned template and data. The implementation must HTML-escape text and attributes, validate canonical paths, and encode JSON-LD without allowing a `</script>` breakout.

A dynamic renderer should return `404` when an ID does not exist and a redirect when the canonical slug or ID changed. It should not return an empty `200` shell and expect the browser API call to decide later.

Dynamic sitemap entries also need an application provider because a pattern does not enumerate its real values. Only canonical, public, indexable URLs should be included. Exclude authenticated pages, missing entities, search/filter combinations, and arbitrary query variants unless the application explicitly defines them as canonical pages.

## Before and after: a two-page site

### Before

The author puts all route ownership in the browser shell:

```html
<!-- web/index.html -->
<script src="/framework/basic-web.js"></script>
<script>
  Router.register("/", "page-home");
  Router.register("/about", "page-about");
</script>

<route-view></route-view>
```

The two page files contain ordinary inert templates. Both `GET /` and `GET /about` receive the same empty route outlet. The browser must build the actual page.

### After

The two page templates own their public routes and metadata through the draft attributes shown earlier. `index.html` contains only the shared shell and injection points. The startup scan generates the server/browser route table.

`GET /about` returns a route-specific document similar to:

```html
<!doctype html>
<html lang="en-AU">
  <head>
    <title data-basic-web-title>About us | Acme Gardening</title>
    <meta
      name="description"
      content="Meet the Sydney gardeners behind Acme Gardening."
      data-basic-web-description
    >
    <meta name="robots" content="index,follow" data-basic-web-robots>
    <link
      rel="canonical"
      href="https://acmegardening.example/about"
      data-basic-web-canonical
    >
    <link rel="stylesheet" href="/styles.css">
    <script src="/framework/basic-web.js"></script>
  </head>
  <body>
    <nav>
      <a href="/">Home</a>
      <a href="/about">About</a>
    </nav>

    <route-view data-basic-web-server-rendered data-route-path="/about">
      <page-about data-basic-web-server-rendered>
        <template shadowrootmode="open">
          <main>
            <h1>About Acme Gardening</h1>
            <p>We have helped Sydney gardeners since 2012.</p>
            <a href="/">Read our gardening advice</a>
          </main>
        </template>
      </page-about>
    </route-view>
  </body>
</html>
```

With JavaScript disabled, a compatible browser displays the About page. With JavaScript enabled, Basic Web defines `page-about`, reuses its root, adopts the shared stylesheet, and attaches behavior. The HTTP response contains the page title, canonical URL, page content, and links without requiring a component-source fetch. Whether a non-browser crawler interprets Declarative Shadow DOM is part of the compatibility spike.

The author-facing difference is small: route metadata moves from duplicated router calls to the page template. The framework difference is substantial: the server now understands routes, composes page content, owns statuses and canonical URLs, and coordinates hydration.

## Delivery phases

### Phase 1: define and validate public routes

- Parse `data-route` and draft metadata attributes from page templates.
- Add the generated routes to the in-memory model and browser manifest.
- Make the browser router register generated routes.
- Keep explicit `Router.register()` for backward compatibility and document it as client-only unless paired with server configuration.
- Detect duplicates and invalid route definitions at startup.
- Return a real `404` document for unmatched frontend paths instead of treating the home page as not found.

Completion evidence: route-manifest tests and direct HTTP tests prove that `/`, `/about`, and an unknown path select different route outcomes.

### Phase 2: render static routes into the shell

- Parse `index.html` structurally and fill its marked title, description, robots, canonical, and outlet nodes.
- Recursively instantiate static page and nested element templates.
- Escape all application-controlled metadata and content.
- Preserve the current asset and framework-resource behavior.
- Define cache and `HEAD` behavior for rendered documents.

Completion evidence: `curl` with JavaScript absent receives meaningful, route-specific HTML for each public page.

### Phase 3: hydrate without clearing content

- Make `ShadowHTMLElement` reuse declarative roots.
- Make `route-view` preserve a matching initial route tree.
- Skip loading visibility rules for server-rendered content.
- Update metadata during client navigation and back/forward navigation.
- Define mismatch and error recovery.

Completion evidence: a browser test proves that initial DOM content remains present while definitions load, interaction works after upgrade, and no duplicate page tree appears.

### Phase 4: settle CSS delivery

- Prototype shared links inside roots and a semantic light-DOM alternative.
- Remove the broad `:not(:defined)` rule from the recommended setup.
- Measure first paint, layout shift, HTML size, CSS requests, and style parity on nested and repeated elements.
- Document the supported browser baseline and fallback.

Completion evidence: both JavaScript and no-JavaScript browser runs show the intended styles in light and dark color schemes without a hidden or unstyled content flash.

### Phase 5: support dynamic pages

- Design the typed Go renderer API and route-parameter contract.
- Let renderers set status, metadata, canonical path, content, and optional structured data.
- Add dynamic URL enumeration for sitemaps.
- Establish caching and private-data rules.

Completion evidence: an existing item returns meaningful HTML and `200`; a missing item returns the site's not-found document with `404`; client hydration preserves the rendered item.

### Phase 6: add crawl controls and release guidance

- Generate a sitemap from static routes plus application-provided dynamic URLs.
- Use configured `SiteOrigin` for sitemap and canonical URLs.
- Keep custom `robots.txt` support and advertise the generated sitemap.
- Add optional `noindex` and JSON-LD route fields.
- Document deployment checks with Search Console's URL Inspection tool and the Rich Results Test.

Completion evidence: robots, sitemap, canonical, noindex, and structured-data tests exercise their public HTTP responses.

## Verification plan

Prefer checks at the HTTP and browser boundaries rather than tests that search source files for strings.

### HTTP integration tests

Use `fstest.MapFS` with a two-page site and assert:

- `/` and `/about` return different titles, descriptions, canonicals, and main content.
- Nested element text and links occur in the rendered response.
- Canonical URLs use configured `SiteOrigin`, even when `Host` and forwarding headers are hostile.
- Route metadata and renderer values are escaped safely.
- Unknown routes and missing dynamic entities return `404`.
- Redirected entities return the intended `3xx` status and `Location`.
- Static assets, framework resources, `GET`, and `HEAD` retain correct behavior.
- `noindex` appears only on routes that request it.
- The sitemap contains only absolute, canonical, indexable URLs.
- The default and custom `robots.txt` behavior advertises the correct sitemap origin.

### Browser tests

Run the same fixtures in a real browser:

- With JavaScript disabled, each public route remains readable and navigable.
- With JavaScript enabled, the server tree stays visible during element loading.
- Declarative roots retain their children during upgrade.
- Shared and component CSS match before and after hydration.
- Interactions work after hydration.
- Client navigation updates content and all head metadata.
- Refresh, back, and forward navigation retain the correct route.
- A forced hydration mismatch follows the documented recovery path.

### Deployed-site checks

Use Search Console URL Inspection to compare fetched and rendered HTML, and use the Rich Results Test for pages with structured data. Check a real crawler-visible response with `curl` as well. Do not rely only on a crawler user agent, because the server should not vary the page by bot identity.

## Risks and open decisions

The design needs answers to these questions before implementation:

1. **Declarative Shadow DOM or semantic light DOM?** Declarative Shadow DOM best preserves current encapsulation and composition. Light DOM is simpler for non-rendering parsers and global CSS. A prototype should decide with evidence.
2. **How will server-side CSS work?** Repeated links, repeated inline CSS, compiled component CSS, and light-DOM rendering have different first-paint and payload costs.
3. **How much template syntax will the server support?** Static recursive expansion is tractable. Executing arbitrary component JavaScript on Go is not. Attributes, slots, conditional content, and DOM created in JavaScript need defined boundaries.
4. **How will dynamic renderers avoid duplicate markup?** The framework currently has no shared server/browser template language or data-binding model. The API should not force authors to maintain divergent page copies.
5. **What is the hydration identity check?** Route path alone may be enough for static pages but not for changing dynamic data. Version and content checks may be needed.
6. **Where does route precedence live?** Static patterns, parameterized patterns, explicit browser registrations, application HTTP handlers, and not-found behavior need deterministic ordering.
7. **What is the compatibility promise?** Existing projects must continue to work as client-rendered applications. SEO support may start as opt-in until route metadata and hydration stabilize.
8. **How are sitemaps enumerated?** Static routes are automatic. Dynamic routes require an application provider, pagination, update timestamps, and size limits.
9. **What is safe to cache?** Static rendered pages can share caches. Dynamic and personalized responses need explicit cache policy and must never leak user data.
10. **How large can recursive HTML become?** Repeated component instances duplicate declarative roots. Compression helps transfer size, but parsing and memory still need measurement.

## Proposed acceptance criteria

SEO support is ready for an initial stable release when:

- A new two-page static site declares each public route once.
- Direct route requests return unique, meaningful HTML and correct statuses without JavaScript.
- Nested public component content is present in the first response.
- Hydration does not clear or hide that content.
- Client navigation preserves current router ergonomics and updates metadata.
- Unknown routes return `404`, not the home page with `200`.
- Canonical and sitemap URLs come from trusted site configuration.
- Dynamic routes have a documented server-renderer path and correct missing-entity semantics.
- Automated HTTP and real-browser tests cover the public behavior.
- The documentation states which content remains client-only and what that means for crawling.

## Suggested first design spike

Build only the static two-page example from this document behind an opt-in option. Compare two implementations of the initial page body:

1. Recursive Declarative Shadow DOM with a shared stylesheet link inside each root.
2. Semantic light DOM that custom elements adopt or project during hydration.

Measure response HTML size, requests, first contentful paint, layout shift, no-JavaScript appearance, hydration behavior, and Google rendered HTML. That spike will resolve the highest-risk choice before Basic Web commits to public route or renderer APIs.
