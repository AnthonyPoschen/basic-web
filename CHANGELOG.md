# Changelog

## Unreleased

### Breaking

- Replace the split framework bootstrap scripts with a single `/framework/basic-web.js` runtime. Apps must update HTML that loads `/framework/utils.js`, `/framework/loader.js`, and `/framework/router.js` to load `/framework/basic-web.js` instead.

### Changed

- Start fetching `/framework/element-manifest.json` as soon as the framework runtime loads so element discovery can overlap more of the startup path.

### Fixed

- Generate a valid permissive `/robots.txt` fallback when an app does not provide one, and include a `Sitemap:` line when `sitemap.xml` exists.
- Return `404` for missing asset-like paths instead of serving the SPA shell for requests such as `/robots.txt` or other dotted paths.

## v0.20260521.0 - 2026-05-21

### Fixed

- Preserve nested directories in the minified in-memory filesystem so `fs.WalkDir` and `fs.ReadDir` can discover nested web elements.
