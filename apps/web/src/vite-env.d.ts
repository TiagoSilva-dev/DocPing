/// <reference types="vite/client" />

// Only the public, build-time variables the site reads. Declaring them keeps
// a typo in an env name a compile error instead of a silently empty string.
interface ImportMetaEnv {
  readonly VITE_GA4_ID?: string;
  readonly VITE_GOOGLE_ADS_ID?: string;
  readonly VITE_ADS_SIGNUP_LABEL?: string;
  readonly VITE_MEETING_URL?: string;
  readonly VITE_COMPANY_LEGAL_NAME?: string;
  readonly VITE_COMPANY_TAX_ID?: string;
  readonly VITE_COMPANY_ADDRESS?: string;
}
interface ImportMeta {
  readonly env: ImportMetaEnv;
}
