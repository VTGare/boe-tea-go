import guide from "./index.html";

const IMAGE_PATH = /^\/(img-original|img-master|c|custom-thumb|user-profile|img-zip-ugoira)\//;
const YEAR = 365 * 24 * 60 * 60;

const GUIDE_HEADERS = {
	"content-type": "text/html; charset=utf-8",
	"cache-control": "public, max-age=3600",
	"content-security-policy":
		"default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
	"referrer-policy": "no-referrer",
	"x-content-type-options": "nosniff",
};

export default {
	async fetch(request) {
		const url = new URL(request.url);
		const readOnly = request.method === "GET" || request.method === "HEAD";

		if (readOnly && url.pathname === "/") {
			return new Response(request.method === "HEAD" ? null : guide, { headers: GUIDE_HEADERS });
		}

		if (url.pathname === "/robots.txt") {
			return new Response("User-agent: *\nAllow: /$\nDisallow: /\n", {
				headers: { "content-type": "text/plain" },
			});
		}

		if (!readOnly || !IMAGE_PATH.test(url.pathname)) {
			return new Response("Not found", { status: 404 });
		}

		const headers = { Referer: "https://www.pixiv.net/" };
		const range = request.headers.get("range");
		if (range) {
			headers.Range = range;
		}

		// i.pximg.net serves every image, R18 included, to any request with this Referer.
		// Image paths never change content, so successful responses are cached for a year.
		const upstream = await fetch(`https://i.pximg.net${url.pathname}`, {
			method: request.method,
			headers,
			cf: {
				cacheEverything: true,
				cacheTtlByStatus: { "200-299": YEAR, 404: 3600, "500-599": 0 },
			},
		});

		const response = new Response(upstream.body, upstream);
		response.headers.delete("set-cookie");
		return response;
	},
};
