# Unique first-response HTML for public routes

## Problem

Basic Web serves `index.html` for `/` and for non-file paths. That shell has
one title, one description, and an empty `<route-view>`. Crawlers that do not
run JavaScript, and Lighthouse SEO, see the same document for every URL.
Google can render JS later, but the first HTML is what gets queued, titled,
and linked. Duplicate shells also look like soft 404s when unknown paths
return `200` and the homepage.

## Practice

Consuming apps should return **route-specific first HTML** for every
indexable URL, then enhance it with the custom-element runtime. The same
bytes go to people and bots. Do not branch on `User-Agent`.

Each indexable response needs:

- A unique `<title>` and a unique meta description
- An absolute self-canonical from a configured site origin, not `Host`
- Visible semantic copy in the first HTML (headings, paragraphs, images
  with alt text). A populated `<template>` or an empty `<route-view>` is
  not visible.
- Ordinary `<a href>` links to other public URLs. History API navigation is
  fine; `#` fragment routes are not crawlable.
- A sitemap of those absolute canonicals, advertised from `robots.txt`
- A public cache policy (`public` with a positive `max-age`). `no-store`
  and missing `Cache-Control` keep crawlers and repeat visits from reusing
  the document.
- A real `404`/`410` for unknown or removed public paths, not the homepage
  shell with `200`

Lighthouse SEO is a floor (titles, description, crawlable `href`s,
canonical). It is not the product. Ranking and Search Console are out of
band.

Genos does this in the app by rewriting `index.html` for
`/plans/{game}/{region}` before the framework catch-all. A framework hook
is only needed if an app cannot intercept those routes itself.

See [seo-support-plan.md](../seo-support-plan.md) for the longer research.

## Anti-pattern

Do not ship one app-shell title for every path and wait for JS to fill it
in. Do not put the only unique copy in `<noscript>` or in a bot-only
variant. Do not list hash URLs in the sitemap. Do not use request `Host`
as the canonical origin.

## How we learned it

Genos needed pages for intents such as “Factorio server in Australia”. A
query-string SPA (`/plans?game=factorio`) returned the same first HTML as
`/`. Unique documents on path URLs, a sitemap of those canonicals, and
public HTML cache made the first GET useful without changing the homepage
cache policy.
