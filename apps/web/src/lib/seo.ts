import { useEffect } from "react";
import {
  SITE_NAME,
  SITE_URL,
  faqPageLd,
  softwareApplicationLd,
  type PlanForLd,
  type QuestionForLd,
} from "./answers";

// This module is the browser half of the subject: it writes tags into the
// document for the route that is open. What the site says about itself lives
// in lib/answers.ts, which has no React in it because apps/web/vite.config.ts
// reads it at build time -- the answer engines do not run our JavaScript. The
// names below are re-exported so every page keeps importing them from here.
export {
  SITE_NAME,
  SITE_URL,
  faqPageLd,
  softwareApplicationLd,
  type PlanForLd,
  type QuestionForLd,
};

export type PageMeta = {
  // The whole <title>, already including the brand.
  title: string;
  // Omitted on private pages, where no crawler ever gets far enough to read it.
  description?: string;
  // Canonical path, with the leading slash. Pages without one get no canonical
  // tag — a workspace or portal URL must never be offered as an address to index.
  path?: string;
  // Opt in, never the default: a new page is private until someone says it is not.
  index?: boolean;
};

function tag(selector: string, create: () => HTMLElement): HTMLElement {
  const existing = document.head.querySelector(selector);
  if (existing instanceof HTMLElement) return existing;
  const element = create();
  document.head.append(element);
  return element;
}

function meta(attribute: "name" | "property", value: string, content: string) {
  const element = tag(`meta[${attribute}="${value}"]`, () => {
    const created = document.createElement("meta");
    created.setAttribute(attribute, value);
    return created;
  });
  element.setAttribute("content", content);
}

// Every route calls this, so the values left over from the previous route are
// always replaced — a stale noindex or canonical is worse than none at all.
export function applyPageMeta({ title, description, path, index }: PageMeta) {
  document.title = title;
  meta(
    "name",
    "robots",
    // max-snippet:-1 lifts the cap on how much of the page an engine may
    // quote in an answer. There is nothing on a public page of ours that we
    // would rather it paraphrase than quote.
    index
      ? "index, follow, max-image-preview:large, max-snippet:-1"
      : "noindex, nofollow",
  );
  meta("property", "og:title", title);
  meta("property", "og:site_name", SITE_NAME);
  meta("property", "og:type", "website");
  if (description) {
    meta("name", "description", description);
    meta("property", "og:description", description);
  }
  const canonical = document.head.querySelector('link[rel="canonical"]');
  if (path) {
    const href = new URL(path, SITE_URL).toString();
    const link =
      canonical instanceof HTMLLinkElement
        ? canonical
        : (() => {
            const created = document.createElement("link");
            created.rel = "canonical";
            document.head.append(created);
            return created;
          })();
    link.href = href;
    meta("property", "og:url", href);
  } else {
    canonical?.remove();
    document.head.querySelector('meta[property="og:url"]')?.remove();
  }
}

export function usePageMeta(page: PageMeta) {
  const { title, description, path, index } = page;
  useEffect(() => {
    applyPageMeta({ title, description, path, index });
  }, [title, description, path, index]);
}

// Structured data is no longer injected from here. It is written into
// index.html at build time by the answer-engine plugin in vite.config.ts, from
// the same arrays the landing page renders: a crawler that does not run
// JavaScript -- which is most of the answer engines -- would never have seen a
// script this module appended, and emitting it twice would mean two FAQPage
// blocks for the one crawler that does.
