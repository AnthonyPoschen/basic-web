# Load third-party scripts when the page needs them

## Problem

A document-level Clerk (or analytics) `<script>` downloads on every public
page. On a Genos mobile Lighthouse run that was ~173KB of unused Clerk JS and
a Time to Interactive of **6.4s**, even though the visitor was anonymous.

`defer` still starts the download on the critical path. Unused JS still
counts.

## Practice

- Store the third-party URL on the document (`data-clerk-js-url`) instead of
  a blocking or deferred `<script src>`.
- Inject the script from application code when the page needs it: sign-in,
  dashboard, admin, or a detectable session cookie (`__client_uat` for Clerk).
- Show the signed-out header immediately. Do not wait for `window` load to
  paint "Sign in".

The framework does not load Clerk. Consuming apps own this.

## Anti-pattern

Do not put `clerk.browser.js` in `index.html` "because the SPA might need it".
Do not wait for `DOMContentLoaded` before starting a script that a sign-in
page needs now.

## How we learned it

Removing the Clerk document script from Genos and loading it from `clerk.js`
dropped TTI from 6.4s to 2.0s on the anonymous homepage. Sign-in and
dashboard still call `waitForClerk()`.
