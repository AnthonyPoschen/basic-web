# Revalidate HTML that pins versioned assets

## Problem

Release HTML requests scripts and styles as `/scripts/foo.js?v=<git sha>`.
Those files are immutable. The HTML document itself has no `?v=` and is the
only URL a returning visitor hits. If that document is `public, max-age=3600`,
the browser keeps the previous deploy for an hour. A normal revisit still
loads the old shell, which still points at the old `?v=` files, until the
person hard-refreshes.

## Practice

Indexable HTML should be stored but revalidated:

```
Cache-Control: public, max-age=0, must-revalidate
```

Crawlers still see a public document. Pair that header with an `ETag` for the
release (the website git SHA). Browsers send `If-None-Match` on the next visit.
Unchanged releases answer `304` with no body. After a deploy they receive new
HTML (and new `?v=` URLs). Without the ETag, `max-age=0` would re-download the
document on every visit.

Keep versioned `/scripts/`, `/styles/`, and `/framework/` URLs
`public, max-age=31536000, immutable`. Do not move that long lifetime onto
`/`, `/games`, `/plans`, or intent pages.

## Anti-pattern

Do not cache the unversioned HTML shell for minutes or hours. Do not set
`no-store` on public pages to force freshness; that blocks reuse and 304s.
Do not use `no-cache` alone on `/` when the SEO contract requires `public`.

## How we learned it

A Genos production pin shipped Discord links and a homepage fix. curl of the
new origin saw them immediately. Browser tabs that had visited during the
previous hour kept serving the old first HTML until a hard refresh, because
`PublicCacheControl` was `public, max-age=3600`.
