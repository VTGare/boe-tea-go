# pximg proxy

The Cloudflare Worker behind `boetea.dev`. The Pixiv provider rewrites `https://i.pximg.net` to
this host (`pixiv.proxy_host` in the config), because Discord can't load images from pximg
directly.

`i.pximg.net` serves any image, R-18 included, to a request with `Referer: https://www.pixiv.net/`,
so the Worker needs no Pixiv credentials. It passes GET and HEAD on Pixiv image paths, caches
successful responses at Cloudflare's edge for a year, and serves the guide in `index.html` at `/`.
Everything else gets a 404.

## Deploy

`worker.js` is the main module and `index.html` is uploaded next to it as a text module. The
`boetea.dev/*` and `www.boetea.dev/*` routes and the proxied DNS records are set up once in the
Cloudflare dashboard.

## Limits

Workers Free allows 100,000 requests a day, cached ones included, since the Worker runs before
the cache. Past that, images fail until 00:00 UTC. Workers Paid lifts the cap.
