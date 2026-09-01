# Changelog

## v0.20260901.2 - 2026-09-01

### Added

- Generate static custom-element dependency graphs so route component trees can begin fetching known templates concurrently.
- Add `SetupHttpMuxWithOptions` for applications to provide their release web version.

### Changed

- Preload the versioned element manifest when a release version is configured, matching the loader request exactly.

### Fixed

- Cache a release-versioned manifest immutably while keeping unversioned manifest requests revalidatable for compatibility.

## v0.20260901.1 - 2026-09-01

### Changed

- Start route hydration when the route outlet connects so component requests can overlap deferred third-party scripts instead of waiting for `DOMContentLoaded`.
- Keep route content marked busy until hydration and follow-up scans complete, then reveal the final route on the next animation frame.

### Fixed

- Serve the SPA entry point for unknown non-asset paths, preserving client-side routes on direct navigation.

## v0.20260901.0 - 2026-09-01

### Changed

- Hydrate route component trees atomically: direct template dependencies now fetch concurrently and become visible only after their modules are registered.
- Avoid rebuilding the shared adopted stylesheet for every custom-element construction.

## v0.20260823.0 - 2026-08-23

### Fixed

- Install every template from a component file so files that define multiple custom elements render each element with its matching template.
- Deduplicate component loads by resolved file URL while an element definition is pending, avoiding duplicate custom-element registration for shared component files.

## v0.20260521.1 - 2026-05-21

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
