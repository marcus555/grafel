# grafel.app landing site

A single self-contained static page (`index.html` — all CSS/JS inline, no external
requests) plus Cloudflare Pages support files. Hosted free on **Cloudflare Pages**.

## Files
- `index.html` — the landing page (grep⇄grafel explorer, coverage matrix, install tabs).
- `404.html` — minimal not-found page (Pages serves it automatically).
- `_headers` — security headers + revalidate caching for the HTML.
- `../wrangler.toml` — Pages project config (`pages_build_output_dir = "site"`).

## Deploy (free)

One-time setup — buy `grafel.app` on Cloudflare Registrar (at-cost, ~$12–14/yr; `.app`
is HTTPS-only via HSTS preload, which Cloudflare's automatic SSL covers).

```bash
npm i -g wrangler
wrangler login
# from the repo root:
wrangler pages deploy site --project-name grafel
```

Then in the Cloudflare dashboard: **Pages → grafel → Custom domains → add `grafel.app`**.
Because the domain's DNS is already in the same account, the record + cert are
provisioned automatically.

### Git-connected (auto-deploy on push) — optional
Connect this repo as a Pages project with **build output directory = `site`** and no
build command; every push to the default branch redeploys.

## Editing
`index.html` is self-contained — edit and redeploy. It's theme-aware (light/dark) and
has no build step. The canonical source is kept in sync with the project's landing page.
