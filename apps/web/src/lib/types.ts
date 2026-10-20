export type ItemStatus =
  "pending" | "uploaded" | "approved" | "correction_required";
export type RequestStatus =
  | "pending"
  | "in_progress"
  | "under_review"
  | "correction_required"
  | "completed"
  | "cancelled";
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
}
export interface RequestItem extends TemplateItem {
  status: ItemStatus;
  correctionReason?: string;
  correctionFileId?: string;
  files: DocumentFile[];
  reviews?: {
    action: string;
    reason?: string;
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
