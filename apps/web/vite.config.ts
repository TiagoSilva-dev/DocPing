import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import { answerSourceHtml, landingLd, llmsTxt } from "./src/lib/answers";

// Answer engines -- ChatGPT, Perplexity, Claude, Gemini and the fetchers
// behind the AI answers in search -- read the HTML that arrives over the wire
// and do not run our bundle. A client-rendered page is, to them, an empty
// <div id="root">. This plugin writes what the landing page says into the HTML
// at build time, from the very module the page renders (src/lib/answers.ts):
//
//  1. the answer source, inside #root. React clears the container when it
//     mounts, so a person sees it only while the bundle loads -- and someone
//     with JavaScript off reads the page instead of a blank screen.
//  2. the JSON-LD for the product, the FAQ and the three steps. index.html
//     carries Organization and WebSite by hand, which describe the site and
//     must be there even on a route that renders nothing.
//  3. /llms.txt, the file an agent fetches to find out what this site is
//     without parsing a page at all.
//
// Only at build: the dev server keeps serving the HTML it always did, so the
// Playwright suite and a day of work see the same page as before. /llms.txt is
// served in development too, because a file nobody can open is a file nobody
// checks.
function answerEngines(): Plugin {
  const body = () => llmsTxt();
  return {
    name: "docping:answer-engines",
    transformIndexHtml: {
      order: "pre",
      handler(html, ctx) {
        if (ctx.server) return html;
        const root = '<div id="root"></div>';
        if (!html.includes(root))
          throw new Error(
            "index.html no longer has an empty #root: the answer source has nowhere to go.",
          );
        return {
          html: html.replace(
            root,
            `<div id="root">${answerSourceHtml()}</div>`,
          ),
          tags: [
            {
              tag: "script",
              attrs: { type: "application/ld+json" },
              // "<" cannot reach the page as itself: a "</script" inside a
              // data string would close the tag early.
              children: JSON.stringify(landingLd()).replace(/</g, "\\u003c"),
              injectTo: "head",
            },
          ],
        };
      },
    },
    configureServer(server) {
      server.middlewares.use("/llms.txt", (_request, response) => {
        response.setHeader("Content-Type", "text/plain; charset=utf-8");
        response.end(body());
      });
    },
    generateBundle() {
      this.emitFile({ type: "asset", fileName: "llms.txt", source: body() });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), answerEngines()],
  resolve: { alias: { "@": path.resolve(import.meta.dirname, "./src") } },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        target: process.env.API_PROXY_TARGET || "http://127.0.0.1:8080",
        changeOrigin: false,
      },
    },
  },
});
