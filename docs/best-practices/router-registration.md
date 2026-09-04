# Register routes in a deferred file

## Problem

`window.Router` exists only after `/framework/basic-web.js` runs. Route
registration in `index.html` is application code. If it runs too early, no
routes exist and `<route-view not-found="page-home">` renders the home page
for `/plans`, `/dashboard`, and every other path.

## Practice

Keep registration next to the runtime, as two deferred external scripts in
order:

```html
<script defer src="/framework/basic-web.js?v=__WEB_VERSION__"></script>
<script defer src="/scripts/routes.js?v=__WEB_VERSION__"></script>
```

`routes.js` is a classic script (not a module) that only calls
`window.Router.register`. Serve it under `/scripts/` with the same version
query as the rest of the release.

`route-view` upgrades while `basic-web.js` runs and starts routing on a
microtask, which flushes **before** the next deferred script. If
`routes.js` only registers, `start()` has already rendered `not-found`.
After the last `register` call, re-notify so the real route wins:

```js
window.Router.navigate(
  `${location.pathname}${location.search}${location.hash}`,
  { replace: true },
)
```

Same-URL `navigate` notifies subscribers without pushing history.

A test on the production `index.html` filesystem should assert:

- the document contains `src="/scripts/routes.js?v=..."`
- the document does not contain inline `window.Router.register`

## Anti-pattern

Do not inline `Router.register` in `index.html`, even with `defer`.
Do not put registration in a `type="module"` file that imports the router
unless the framework exposes a module entry; today the runtime is a classic
script that assigns `window.Router`.
Do not assume `register` after `route-view` has connected will re-render;
call `navigate` on the current URL once registration is complete.

## How we learned it

Genos moved `Router.register` into a deferred inline block to unblock first
paint. Production then served the homepage for every URL until registration
moved to `web/scripts/routes.js`. See
[defer-external-scripts.md](defer-external-scripts.md).
