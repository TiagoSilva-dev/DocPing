export type ItemStatus =
  "pending" | "uploaded" | "approved" | "correction_required";
export type RequestStatus =
  | "pending"
  | "in_progress"
  | "under_review"
  | "correction_required"
  | "completed"
  | "cancelled";
export type AIReviewMode = "off" | "suggest" | "auto";
export type AnalysisStatus =
  | "queued"
  | "running"
  | "completed"
  | "failed"
  | "disabled";
export type AnalysisVerdict = "approved" | "review" | "rejected";
export interface AnalysisSignal {
  key: string;
  label: string;
  kind: "noul" | "score" | "choice";
  answer?: string;
  value: number;
  confidence: number;
}
/** Automated conformity check of one file: Docling/PaddleOCR extract the text,
 *  Jev judges it, and the server composes the verdict. */
export interface Analysis {
  status: AnalysisStatus;
  verdict?: AnalysisVerdict;
  confidence: number;
  reason?: string;
  signals?: AnalysisSignal[];
  extractor?: string;
  model?: string;
  characters?: number;
  applied: boolean;
  error?: string;
  requestedAt?: string;
  completedAt?: string;
}
export interface Organization {
  id: string;
  name: string;
  demo: boolean;
  clientLimit: number;
  plan: string;
  email?: string;
  remindersEnabled?: boolean;
  reminderDays?: number[];
  retentionDays?: number;
  aiReview?: AIReviewMode;
}
export interface Session {
  environment?: string;
  user: { id: string; name: string; email: string };
  organization: Organization;
  csrfToken: string;
}
export interface Client {
  id: string;
  name: string;
  cnpj: string;
  contactName: string;
  email: string;
  phone: string;
  active: boolean;
  createdAt: string;
}
export interface TemplateItem {
  id: string;
  name: string;
  instruction: string;
  required: boolean;
  maxFiles: number;
}
export interface Template {
  id: string;
  name: string;
  description: string;
  items: TemplateItem[];
  active: boolean;
}
export interface DocumentFile {
  id: string;
  name: string;
  mimeType: string;
  size: number;
  createdAt: string;
  scanStatus: string;
  superseded?: boolean;
  sample?: boolean;
  analysis?: Analysis;
}
export interface RequestItem extends TemplateItem {
  status: ItemStatus;
  correctionReason?: string;
  correctionFileId?: string;
  files: DocumentFile[];
  reviews?: {
    action: string;
    reason?: string;
    source?: string;
    userId: string;
    createdAt: string;
  }[];
}
export interface DocumentRequest {
  id: string;
  clientId: string;
  clientName: string;
  title: string;
  competence: string;
  dueDate: string;
  status: RequestStatus;
  items: RequestItem[];
  createdAt: string;
  updatedAt: string;
  recurring?: boolean;
}
export interface Schedule {
  id: string;
  name: string;
  templateId: string;
  clientIds: string[];
  dayOfMonth: number;
  dueDays: number;
  active: boolean;
  lastRun?: string;
  nextRun?: string;
}
export interface Portal {
  environment?: string;
  request: DocumentRequest;
  organization: { name: string; demo?: boolean };
  client: { name: string };
  maxUploadBytes?: number;
}
