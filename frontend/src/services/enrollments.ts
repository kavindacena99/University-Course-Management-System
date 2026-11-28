import type { Enrollment, EnrollmentList } from "../types/api";
import { apiRequest } from "./api";

export function createEnrollment(offeringId: string): Promise<{ enrollment: Enrollment }> {
  return apiRequest("/api/v1/enrollments", {
    method: "POST",
    body: JSON.stringify({ offering_id: offeringId }),
  });
}

export function cancelEnrollment(
  enrollment: Enrollment,
): Promise<{ enrollment: Enrollment }> {
  return apiRequest(`/api/v1/enrollments/${encodeURIComponent(enrollment.id)}/cancel`, {
    method: "POST",
    body: JSON.stringify({ version: enrollment.version }),
  });
}

export function listStudentEnrollmentHistory(studentId: string): Promise<EnrollmentList> {
  return apiRequest(
    `/api/v1/students/${encodeURIComponent(studentId)}/enrollments?limit=100&offset=0`,
  );
}
