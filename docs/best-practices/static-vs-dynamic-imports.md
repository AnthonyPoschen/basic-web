# Static imports preload on every page that uses the element

## Problem

Basic Web records literal local `import` paths in element files and emits
`modulepreload` as soon as that element is in the tree. A public
`<site-header>` that statically imports `/scripts/server-admin.js` therefore
downloads the dashboard module on the marketing homepage.

Lighthouse then reports unused JavaScript (34KB of `server-admin.js` on `/`
in the Genos run). The bytes also delay Time to Interactive.

## Practice

- Use a static `import` only for code every visitor of that element needs.
- Use `import("/scripts/server-admin.js")` (dynamic) for signed-in-only or
  route-only work. Do not `await` that work before painting public chrome.
  An admin nav link can appear after idle; only one person sees it.
- Put shared auth helpers in a small module (`clerk.js`) that the header can
  static-import without pulling the rest of the admin API.

The framework will not preload dynamic `import()` paths. That is intentional.

## Anti-pattern

Do not static-import a kitchen-sink admin module from layout used on public
pages. Do not add a second hand-written preload list to skip it; change the
import instead.

## How we learned it

Genos `site-header.html` imported `getSystemAdminUser` from `server-admin.js`.
The homepage network log fetched that 37KB module for every visitor. The
header now static-imports `clerk.js` and dynamically imports `server-admin.js`
only after a signed-in session, on idle, so the Admin link cannot block LCP.
