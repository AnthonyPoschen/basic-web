# Defer only external classic scripts

## Problem

The HTML `defer` attribute applies to classic scripts that have a `src`. An
inline `<script defer>` is not deferred. The browser runs it as soon as it
parses the tag, which is before any deferred external file in `<head>`.

That looks fine in source:

```html
<script defer src="/framework/basic-web.js"></script>
<script defer>
  window.Router.register("/", "page-home");
</script>
```

On a minified production page it still fails: `window.Router` is undefined,
every path falls through to `not-found`, and Lighthouse records a console error
under Best Practices.

## Practice

- Put deferred work in a real file: `<script defer src="/scripts/routes.js">`.
- Keep deferred files in document order. The runtime file must come first.
- Use `type="module"` when the code is a module. Modules are deferred by
  default and still run after classic `defer` scripts in the same document.

## Anti-pattern

Do not add `defer` to an inline classic script and assume it waits for
`basic-web.js`. Do not "fix" the race by dropping `defer` from the runtime
file unless you have measured that extra render-blocking request.

## How we learned it

A Genos production Lighthouse run after the first performance ship scored
Performance 99 and Best Practices 96 because of
`TypeError: Cannot read properties of undefined (reading 'register')`. The
shell HTML still had `<script defer>window.Router.register(...)`. Live HTML
confirmed `defer` was present; the browser ignored it.

See [router-registration.md](router-registration.md).
