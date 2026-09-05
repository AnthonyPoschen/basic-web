# Preserve host attributes on nested SSR custom elements

## Problem

First-response HTML expands nested custom elements into declarative shadow
DOM. If that rewrite drops the original host attributes, `id`, `class`, and
`data-*` on the nested tag never reach the browser.

`connectedCallback` that looks up the nested element with `getElementById`
then throws (`Cannot read properties of null`). The page stays on its SSR
loading skeleton until a client navigation clones the template, which still
has the attributes.

Client-side `createElement` after the custom element is defined does not hit
this path, so “click another page and come back” looks fine.

## Practice

When expanding a nested host such as
`<server-setup-chooser id="setup-chooser"></server-setup-chooser>`, copy the
original attribute string onto the SSR tag and add
`data-basic-web-server-rendered`. Do not emit a bare `<name data-basic-web-server-rendered>`.

Consuming apps should look up nested custom elements by tag
(`querySelector("server-setup-chooser")`) when the host is unique in that
shadow tree. Do not require an `id` that SSR might not keep until the
framework copy is updated.

## Anti-pattern

Do not rebuild nested hosts as `<name data-basic-web-server-rendered>` and
discard `id` / `class` / `data-*`. Do not assume first-load and client
navigation share the same host attributes.

## How we learned it

Genos `/dashboard` SSR dropped `id="setup-chooser"` from
`<server-setup-chooser>`. `page-app-dashboard` called
`getElementById("setup-chooser").addEventListener` in `connectedCallback`,
threw, and never called `loadDashboard`. A hard refresh stayed on the
placeholder card; in-app navigation rendered the fleet.
