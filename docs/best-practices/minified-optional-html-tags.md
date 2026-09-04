# Minified HTML may omit `</html>`

## Problem

Release `index.html` is minified by `memfs.CreateMinifiedFS` (tdewolff). That
minifier drops optional HTML5 tags: `</head>`, `<body>`, `</body>`, `</html>`.
View Source on production can end at `<route-view …></route-view>` even though
the git file closes `body` and `html`.

Agents often treat that as a truncated document or a missing-tag bug and
"fix" it by disabling minify or forcing end tags.

## Practice

Keep readable `</body></html>` in source. Leave minify as-is. Browsers infer
the missing optional tags. This is not a Lighthouse failure and not a broken
document.

`DEV=1` serves `os.DirFS` without minify, so local View Source still shows
the closing tags.

## Anti-pattern

Do not disable HTML minify to restore `</html>`.
Do not add a post-process that reinserts optional end tags unless a consumer
has a real parser that cannot handle HTML5.

## How we learned it

A Genos production curl of `/` had no `</html>`. The source file still ended
with `</body></html>`. The gap was minify, not a missing close in git.
