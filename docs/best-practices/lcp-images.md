# Size and preload the LCP image

## Problem

Largest Contentful Paint is often a CSS background or hero image. A 319KB
WebP, queued behind render-blocking CSS on HTTP/1.1, produced a **5.3s** LCP
on mobile Genos. The image was also preloaded at desktop size on a 412px
viewport.

Extra font preloads (mono, a fallback family that is never used) compete for
the same connections.

## Practice

- Compress the full-size asset. Add a viewport-sized variant for mobile
  (`home-server-worlds-800.webp` for a `max-width: 800px` hero).
- Preload with `media` so the phone does not fetch the desktop file:

  ```html
  <link rel="preload" as="image" type="image/webp" fetchpriority="high"
    href="/images/home-server-worlds-800.webp" media="(max-width: 800px)">
  <link rel="preload" as="image" type="image/webp" fetchpriority="high"
    href="/images/home-server-worlds.webp" media="(min-width: 801px)">
  ```

- Preload only the font that paints the first heading. Optional display
  fonts and mono can wait for `@font-face`.
- Do not use a 171KB PNG as `rel="icon"` / `apple-touch-icon`. Use the 48px
  ICO and a small 180px PNG.
- Below-the-fold catalog images should be `loading="lazy"` with width and
  height.

Lighthouse will still ask for more compression on artwork. Chase that only
when LCP is still the score limiter.

## Anti-pattern

Do not preload every font in `fonts.css`.
Do not use `image-set(1x, 2x)` for a mobile LCP if 2x selects the 1500px
file on a retina phone.

## How we learned it

After bundling CSS and enabling HTTP/2, a 78KB mobile hero plus media
preloads dropped LCP from 5.3s to 2.0s. Remaining image-delivery bytes were
catalog cards displayed at 354×165 from 460×215 sources.
