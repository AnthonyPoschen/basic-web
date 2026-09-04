# Basic Web

> The usage is basic, the framework is not.

Basic Web is a small Go-served web framework for applications built from native custom elements. It provides a single browser runtime, lazy element loading, shadow-DOM templates with shared global styles, and an optional client-side router.

You write ordinary HTML custom elements. Basic Web discovers their files, builds an element dependency manifest at server startup, and loads the full static element tree before showing a route.

## Start with the example

Clone the repository and run the example application:

```sh
go run .
```

Open `http://localhost:42069`. The example source lives in `web/`:

```text
web/
  index.html
  elements/
    x-counter.html
```

Set `DEV=1` while developing to serve the local `web/` directory and enable reload events:

```sh
DEV=1 go run .
```

## Add Basic Web to a Go server

Install the module:

```sh
go get github.com/AnthonyPoschen/basic-web
```

Embed your frontend and register Basic Web as the fallback handler after your API routes. `SetupHttpMuxWithOptions` is useful when your deployment has a stable release or build version.

```go
package main

import (
    "embed"
    "io/fs"
    "net/http"

    "github.com/AnthonyPoschen/basic-web/pkg/util"
)

//go:embed web/*
var embeddedFS embed.FS

func main() {
    webFS, err := fs.Sub(embeddedFS, "web")
    if err != nil {
        panic(err)
    }

    mux := http.NewServeMux()
    mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })
    util.SetupHttpMuxWithOptions(mux, webFS, util.SetupHttpMuxOptions{
        WebVersion: "build-20260901",
    })

    http.ListenAndServe(":42069", mux)
}
```

Basic Web serves `index.html` for non-file routes, so direct requests to routes such as `/items/42` work. Register API handlers before `SetupHttpMuxWithOptions`, because Basic Web owns the frontend fallback.

## Build a page from custom elements

Include the one framework script in `web/index.html`, register routes, and add a `route-view` where the page should render.

```html
<!doctype html>
<html>
  <head>
    <script src="/framework/basic-web.js"></script>
    <script>
      window.Router.register("/", "page-home");
      window.Router.register("/about", "page-about");
    </script>
  </head>
  <body>
    <route-view not-found="page-home"></route-view>
  </body>
</html>
```

Define an element in `web/elements/`. Files may be nested; their path does not need to match the element name.

```html
<!-- web/elements/pages/home.html -->
<template id="page-home">
  <main>
    <h1>Hello</h1>
    <x-counter></x-counter>
  </main>
</template>

<script type="module">
  customElements.define("page-home", class extends ShadowHTMLElement {
    constructor() {
      super("page-home");
    }
  });
</script>
```

Each element needs:

- A `<template>` whose `id` is the custom-element name.
- A `customElements.define("name", ...)` call in the same file.
- A constructor that calls `super("name")` when it extends `ShadowHTMLElement`.

`ShadowHTMLElement` creates an open shadow root, clones the matching template, and adopts Basic Web's shared global stylesheet. Put element-specific CSS in the element template. Global styles already loaded by the page remain available inside each shadow root through the shared stylesheet.

Use native custom elements for composition. When `page-home` contains `<x-counter>`, the loader includes `x-counter` in the same element tree and defines both before revealing the page.

## Load JavaScript efficiently

Use ordinary static ES module imports inside an element's `type="module"` script:

```html
<script type="module">
  import { getProfile } from "/scripts/profile.js";
  import { formatName } from "./format-name.js";

  customElements.define("profile-card", class extends ShadowHTMLElement {
    constructor() {
      super("profile-card");
    }
  });
</script>
```

Basic Web records local literal imports that begin with `/`, `./`, or `../` in the generated manifest. Before it fetches and evaluates the element scripts, the browser receives matching `modulepreload` hints. Shared imports are deduplicated, so several elements importing `/scripts/profile.js` produce one preload.

This is automatic. Do not add preload tags, route metadata, or a second dependency list in the consuming application.

Only static import declarations are included. Dynamic imports such as `import("/scripts/optional.js")`, bare package specifiers such as `import "lit"`, remote URLs, and runtime-generated paths are intentionally left to the browser at runtime. They cannot be predicted safely from the element manifest.

The browser resolves an imported module's own static imports as normal. Keep imports literal and local when you want Basic Web to start that request as early as possible.

## Understand the generated manifest and cache behavior

At startup, Basic Web scans `elements/**/*.html` and generates `/framework/element-manifest.json`. It records:

- Element name to source-file path mappings.
- Static custom-element dependencies found in templates.
- Local static JavaScript module imports for each element file.

The browser begins fetching the manifest as soon as `/framework/basic-web.js` runs. On route hydration it uses the manifest to request the element tree concurrently, preload known modules, install all templates, then evaluate the element modules. The route remains hidden while that atomic tree is resolving, which prevents fast-network layout shifts caused by elements arriving one at a time.

Pass a stable `WebVersion` in production. Basic Web adds it to the document manifest preload and runtime manifest request. A matching manifest or `/framework/basic-web.js` response is cached for one year with `immutable`; an unversioned request uses `no-cache`. Change the version whenever the deployed frontend changes.

Keep global CSS in separate source files. When `index.html` links two or more local stylesheets, Basic Web serves them as one `/framework/styles.css` file and rewrites the document to that single render-blocking link. Relative `url(...)` paths are rewritten against the original file so fonts and images keep working. External stylesheets are left in place. Source CSS files remain available at their original URLs. This exists because each extra stylesheet is render-blocking and, on HTTP/1.1, steals connections from LCP images; see [Why Basic Web ships these performance features](docs/performance.md).

Basic Web does not fingerprint or cache your application's JavaScript, images, fonts, or other assets for you. Serve those with cache headers and asset URLs appropriate for your deployment. The module preload hints include the same web version query so they match the current page load. The generated stylesheet bundle reuses the query string from the first local stylesheet link, so a `?v=` cache-busting convention on those links also versions `/framework/styles.css`.

## Use the router

Register patterns with literal segments and named parameters:

```html
<script>
  window.Router.register("/items", "page-items");
  window.Router.register("/items/:uuid", "page-item", { section: "catalog" });
</script>
```

The rendered route element receives a `route` property with `path`, `search`, `hash`, `params`, `query`, `queryKeys`, `pattern`, `element`, and `meta`.

`window.Router` provides:

- `register(pattern, element, meta?)`
- `navigate(target, options?)`
- `subscribe(listener)`
- `start()`
- `current`

Same-origin links are handled with the History API. Add `data-router-ignore`, `download`, or a non-`_self` target when a link should not be intercepted.

## Use the element loader directly

Most applications only need `route-view`. The runtime also exposes `window.elementLoader` for an explicit or dynamically inserted element:

```js
await window.elementLoader.hydrate(document.querySelector("profile-card"));
await window.elementLoader.scan();
```

Available methods are `loadManifest()`, `resolveUrl(name)`, `hydrate(root)`, `scan()`, and `scheduleScan()`.

The loader also scans after `DOMContentLoaded`, HTMX settle events, shadow-root creation, and DOM mutations. Use `hydrate` when you need to wait until a specific root and its static element tree are ready before making it visible.

## Guidance for coding agents and maintainers

When changing Basic Web itself, read [Why Basic Web ships these performance features](docs/performance.md) first. Several serving and loader behaviors look like extra machinery. They exist to keep first paint and LCP fast while apps keep CSS and elements in separate source files. Do not undo them to tidy the code without measuring a throttled mobile load, including HTTP/1.1.

When changing an application that uses Basic Web:

- Keep each custom element in `web/elements/` and keep every custom-element name unique across that directory tree.
- Keep the template `id`, custom-element tag, `customElements.define` name, and `super` argument identical and kebab-case.
- Prefer literal local ES module imports. They give the framework enough information to start fetching code in parallel.
- Do not add manual element manifests, import hints, route dependency declarations, or consumer-specific preload configuration. Basic Web generates those from the element source.
- Keep runtime-dependent code as runtime-dependent code. Do not pretend user-specific API responses, authentication state, dynamic imports, or image URLs are statically predictable.
- Register API routes before Basic Web's fallback handler.
- Use `SetupHttpMuxWithOptions` and a new `WebVersion` for production frontend releases.
- Keep global CSS in separate source files. Do not concatenate them in the app to "fix Lighthouse"; the framework already combines local `index.html` stylesheets at serve time.
- Put page-specific or dashboard-only JavaScript behind dynamic `import()` so public layout elements do not preload it.
- Test the server with `go test ./...`. For a frontend performance change, also inspect a real browser network trace and verify that the manifest, element sources, and module preloads begin together.

## Verify a Basic Web change

Run the unit tests:

```sh
go test ./...
```

Run the example in development mode, make a change under `web/`, and confirm that the browser reloads:

```sh
DEV=1 go run .
```

For production behavior, build and run without `DEV=1`, then inspect the response headers and browser network panel. Confirm that the HTML response preloads the versioned manifest, that the matching manifest and `/framework/basic-web.js` responses are immutable, and that local `index.html` stylesheets were combined into one `/framework/styles.css` request. The reasons those checks matter are in [Why Basic Web ships these performance features](docs/performance.md).
