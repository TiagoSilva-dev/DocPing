// Everything on the public pages that is a fact about the company rather than
// about the product. It lives here because Google Ads requires a reachable
// contact, a privacy policy and terms on the landing page, and because an
// address or a measurement id hard-coded inside a section is a thing nobody
// finds again when it changes.
//
// The measurement ids are build-time environment variables with no default on
// purpose: a wrong id reports conversions to someone else's account, and an
// absent one simply means no tag is loaded. Nothing here is a secret — a
// measurement id is public by design, it travels in the page.

export const CONTACT_EMAIL = "contato@docping.com.br";
export const SUPPORT_EMAIL = "suporte@docping.com.br";

// Legal identification. Google reviews the advertiser against what the site
// says about itself, and the LGPD requires the controller to be identifiable.
export const COMPANY_NAME = "DocPing";
export const COMPANY_LEGAL_NAME = import.meta.env.VITE_COMPANY_LEGAL_NAME || "";
export const COMPANY_TAX_ID = import.meta.env.VITE_COMPANY_TAX_ID || "";
export const COMPANY_ADDRESS = import.meta.env.VITE_COMPANY_ADDRESS || "";

// Optional scheduling link (Calendly, Google Agenda and the like). Empty means
// the contact block offers e-mail only, never a dead button.
export const MEETING_URL = import.meta.env.VITE_MEETING_URL || "";

// Google tags. GA4 is "G-...", Google Ads is "AW-...". Either may be absent.
export const GA4_ID = import.meta.env.VITE_GA4_ID || "";
export const GOOGLE_ADS_ID = import.meta.env.VITE_GOOGLE_ADS_ID || "";
// Conversion label for the Ads account, as "AW-000/AbC-D_efG". Without it the
// signup conversion is still sent to GA4 as an event, just not to Ads.
export const ADS_SIGNUP_LABEL = import.meta.env.VITE_ADS_SIGNUP_LABEL || "";

export const hasMeasurement = Boolean(GA4_ID || GOOGLE_ADS_ID);

// Date the published policies took effect, printed on both legal pages.
export const POLICY_DATE = "26 de outubro de 2026";
