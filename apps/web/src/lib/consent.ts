// Google measurement, gated on consent.
//
// Nothing is loaded before the visitor accepts. Not the tag, not a cookie, not
// a request to Google: the script is injected by `grant` and by nothing else,
// so a visitor who refuses or never answers is never measured and never
// contacts a third party. That is stricter than Consent Mode alone, where the
// tag loads denied and still pings Google, and it is the only version of this
// we can honestly describe on the privacy page.
//
// The answer is kept in localStorage rather than a cookie, because a cookie
// asking permission to set cookies is the joke that writes itself. Storage can
// throw (private windows, blocked site data), and a throw must read as "has
// not decided" and leave the site working.

import {
  ADS_SIGNUP_LABEL,
  GA4_ID,
  GOOGLE_ADS_ID,
  hasMeasurement,
} from "./site";

const KEY = "docping.consent.v1";

export type Consent = "granted" | "denied" | null;

type Gtag = (...args: unknown[]) => void;

declare global {
  interface Window {
    dataLayer?: unknown[];
    gtag?: Gtag;
  }
}

export function stored(): Consent {
  try {
    const value = localStorage.getItem(KEY);
    return value === "granted" || value === "denied" ? value : null;
  } catch {
    return null;
  }
}

function remember(value: Exclude<Consent, null>) {
  try {
    localStorage.setItem(KEY, value);
  } catch {
    // A visitor with site data blocked will be asked again next time. That is
    // the correct failure: we would rather ask twice than assume a yes.
  }
}

let loaded = false;

// load injects gtag.js once, with consent already granted. Consent Mode v2 is
// still declared because the tags read it on every hit, and because a future
// granular banner only has to call `gtag("consent", "update", ...)`.
function load() {
  if (loaded || !hasMeasurement) return;
  loaded = true;
  window.dataLayer = window.dataLayer || [];
  const gtag: Gtag = (...args) => {
    window.dataLayer!.push(args);
  };
  window.gtag = gtag;
  gtag("consent", "default", {
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
    analytics_storage: "denied",
  });
  gtag("consent", "update", {
    ad_storage: "granted",
    ad_user_data: "granted",
    ad_personalization: "granted",
    analytics_storage: "granted",
  });
  gtag("js", new Date());
  const primary = GA4_ID || GOOGLE_ADS_ID;
  const script = document.createElement("script");
  script.async = true;
  script.src = `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(primary)}`;
  document.head.appendChild(script);
  if (GA4_ID) gtag("config", GA4_ID);
  if (GOOGLE_ADS_ID) gtag("config", GOOGLE_ADS_ID);
}

export function grant() {
  remember("granted");
  load();
}

export function deny() {
  remember("denied");
}

// restore runs at startup: it loads the tag only for a visitor who already
// said yes, and does nothing at all for everyone else.
export function restore() {
  if (stored() === "granted") load();
}

// track reports an event, and silently does nothing when there is no consent
// or no tag. Callers never have to ask whether measurement is on.
export function track(name: string, params?: Record<string, unknown>) {
  if (!loaded || !window.gtag) return;
  window.gtag("event", name, params ?? {});
}

// pageView is called on every client-side navigation. A single-page app loads
// gtag once, so without this only the landing would ever be counted.
export function pageView(path: string) {
  if (!loaded || !window.gtag) return;
  window.gtag("event", "page_view", {
    page_path: path,
    page_location: window.location.origin + path,
    page_title: document.title,
  });
}

// signup is the conversion the campaign optimises for. It reports to GA4 by
// name and to Google Ads by label, and the label being absent is not an error.
export function signup() {
  track("sign_up");
  if (ADS_SIGNUP_LABEL) track("conversion", { send_to: ADS_SIGNUP_LABEL });
}
