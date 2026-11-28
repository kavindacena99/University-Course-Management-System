import type {
  Assessment,
  AssessmentList,
  CourseOffering,
  EnrollmentList,
  FinalResult,
  Grade,
  StudentReport,
  Submission,
  SubmissionDetail,
  SubmissionList,
} from "../types/api";
import { apiFile, apiRequest } from "./api";

const submissionKeys = new WeakMap<File, string>();
const resultImportKeys = new WeakMap<File, string>();

export type CreateAssessmentInput = Pick<
  Assessment,
  | "title"
  | "description"
  | "component_kind"
  | "due_at"
  | "max_mark"
  | "weight_percent"
  | "max_attempts"
  | "late_policy"
  | "allowed_file_types"
>;

export function listAssignedOfferings(): Promise<{
  offerings: CourseOffering[];
  total: number;
  limit: number;
  offset: number;
}> {
  return apiRequest("/api/v1/offerings/assigned?limit=100&offset=0");
}

export function getOffering(offeringId: string): Promise<{ offering: CourseOffering }> {
  return apiRequest(`/api/v1/offerings/${encodeURIComponent(offeringId)}`);
}

export function listStudentEnrollments(studentId: string): Promise<EnrollmentList> {
  return apiRequest(
    `/api/v1/students/${encodeURIComponent(studentId)}/enrollments?status=active&limit=100&offset=0`,
  );
}

export function listAssessments(
  offeringId: string,
  status?: Assessment["status"],
): Promise<AssessmentList> {
  const params = new URLSearchParams({ limit: "100", offset: "0" });
  if (status) params.set("status", status);
  return apiRequest(
    `/api/v1/offerings/${encodeURIComponent(offeringId)}/assessments?${params.toString()}`,
  );
}

export function getAssessment(assessmentId: string): Promise<{ assessment: Assessment }> {
  return apiRequest(`/api/v1/assessments/${encodeURIComponent(assessmentId)}`);
}

export function createAssessment(
  offeringId: string,
  input: CreateAssessmentInput,
): Promise<{ assessment: Assessment }> {
  return apiRequest(`/api/v1/offerings/${encodeURIComponent(offeringId)}/assessments`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function changeAssessmentStatus(
  assessment: Assessment,
  action: "publish" | "close",
): Promise<{ assessment: Assessment }> {
  return apiRequest(`/api/v1/assessments/${encodeURIComponent(assessment.id)}/${action}`, {
    method: "POST",
    body: JSON.stringify({ version: assessment.version }),
  });
}

export function listSubmissions(assessmentId: string, offset = 0): Promise<SubmissionList> {
  return apiRequest(
    `/api/v1/assessments/${encodeURIComponent(assessmentId)}/submissions?limit=100&offset=${offset}`,
  );
}

export function getSubmission(submissionId: string): Promise<SubmissionDetail> {
  return apiRequest(`/api/v1/submissions/${encodeURIComponent(submissionId)}`);
}

export function saveGrade(
  submissionId: string,
  input: { version: number; mark: number; feedback: string },
): Promise<{ grade: Grade }> {
  return apiRequest(`/api/v1/submissions/${encodeURIComponent(submissionId)}/grade`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function releaseGrade(submissionId: string, version: number): Promise<{ grade: Grade }> {
  return apiRequest(`/api/v1/submissions/${encodeURIComponent(submissionId)}/release`, {
    method: "POST",
    body: JSON.stringify({ version }),
  });
}

export function submitAssignment(assessmentId: string, file: File) {
  let idempotencyKey = submissionKeys.get(file);
  if (!idempotencyKey) {
    if (!globalThis.crypto?.randomUUID) {
      throw new Error("Secure submission identifiers are unavailable in this browser.");
    }
    idempotencyKey = globalThis.crypto.randomUUID();
    submissionKeys.set(file, idempotencyKey);
  }
  const body = new FormData();
  body.append("file", file);
  return apiRequest<{ submission: Submission }>(
    `/api/v1/assessments/${encodeURIComponent(assessmentId)}/submissions`,
    {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body,
    },
  ).then((response) => {
    submissionKeys.delete(file);
    return response;
  });
}

export function getStudentReport(): Promise<{ report: StudentReport }> {
  return apiRequest("/api/v1/analytics/students/me");
}

export function listFinalResults(offeringId: string): Promise<{
  results: FinalResult[];
  total: number;
  limit: number;
  offset: number;
}> {
  return apiRequest(
    `/api/v1/offerings/${encodeURIComponent(offeringId)}/results?limit=100&offset=0`,
  );
}

export function importFinalResults(offeringId: string, file: File) {
  let idempotencyKey = resultImportKeys.get(file);
  if (!idempotencyKey) {
    if (!globalThis.crypto?.randomUUID) {
      throw new Error("Secure import identifiers are unavailable in this browser.");
    }
    idempotencyKey = globalThis.crypto.randomUUID();
    resultImportKeys.set(file, idempotencyKey);
  }
  const body = new FormData();
  body.append("file", file);
  return apiRequest<{ import: { id: string; row_count: number; already_processed: boolean } }>(
    `/api/v1/offerings/${encodeURIComponent(offeringId)}/results/import`,
    {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body,
    },
  ).then((response) => {
    resultImportKeys.delete(file);
    return response;
  });
}

export async function downloadSubmission(submissionId: string): Promise<Blob> {
  return apiFile(`/api/v1/submissions/${encodeURIComponent(submissionId)}/file`);
}
