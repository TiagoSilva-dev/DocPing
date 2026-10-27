import { useEffect } from "react";

// The public identity of the site, in one place. The canonical host is also
// hardcoded in index.html: social and search crawlers read those tags before
// any JavaScript runs, so they cannot be derived from here.
export const SITE_URL = "https://docping.com.br";
export const SITE_NAME = "DocPing";

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
    index ? "index, follow, max-image-preview:large" : "noindex, nofollow",
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

// Structured data lives next to the content it describes, so the two cannot
// drift apart: the builders below take the very arrays the page renders.
export function useJsonLd(id: string, data: unknown) {
  const payload = JSON.stringify(data);
  useEffect(() => {
    const script = document.createElement("script");
    script.type = "application/ld+json";
    script.id = id;
    script.textContent = payload;
    document.head.append(script);
    return () => script.remove();
  }, [id, payload]);
}

export type PlanForLd = { name: string; price: string };
export type QuestionForLd = readonly [question: string, answer: string];

// Only the prices the page itself prints. No rating, no review count: nothing
// here may claim more than the product shows.
export function softwareApplicationLd(plans: readonly PlanForLd[]) {
  const prices = plans
    .map((plan) => Number(plan.price))
    .filter((price) => Number.isFinite(price));
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: SITE_NAME,
    url: `${SITE_URL}/`,
    applicationCategory: "BusinessApplication",
    operatingSystem: "Web",
    inLanguage: "pt-BR",
    description:
      "Plataforma de coleta e conferência de documentos de clientes: solicitações por link, lembretes automáticos e análise automática de cada arquivo recebido.",
    offers: {
      "@type": "AggregateOffer",
      priceCurrency: "BRL",
      offerCount: plans.length,
      lowPrice: Math.min(...prices),
      highPrice: Math.max(...prices),
      offers: plans.map((plan) => ({
        "@type": "Offer",
        name: plan.name,
        price: Number(plan.price),
        priceCurrency: "BRL",
        url: `${SITE_URL}/#precos`,
      })),
    },
  };
}

export function faqPageLd(questions: readonly QuestionForLd[]) {
  return {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    inLanguage: "pt-BR",
    mainEntity: questions.map(([question, answer]) => ({
      "@type": "Question",
      name: question,
      acceptedAnswer: { "@type": "Answer", text: answer },
    })),
  };
}
