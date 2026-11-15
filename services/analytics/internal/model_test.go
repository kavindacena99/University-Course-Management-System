package analytics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAssessmentOutboxEventContracts(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "published grade",
			payload: `{
				"event_id":"11111111-1111-4111-8111-111111111111",
				"event_type":"GradePublished","schema_version":1,
				"occurred_at":"2026-01-02T03:04:05Z",
				"producer_service":"assessment-service",
				"aggregate_id":"22222222-2222-4222-8222-222222222222",
				"aggregate_version":2,
				"grade_id":"22222222-2222-4222-8222-222222222222",
				"submission_id":"33333333-3333-4333-8333-333333333333",
				"student_user_id":"44444444-4444-4444-8444-444444444444",
				"assessment_id":"55555555-5555-4555-8555-555555555555",
				"offering_id":"66666666-6666-4666-8666-666666666666",
				"attempt_no":1,"component_kind":"ca","mark":80,
				"max_mark":100,"weight_percent":40,
				"released_at":"2026-01-02T03:04:05Z"
			}`,
		},
		{
			name: "imported final result",
			payload: `{
				"event_id":"77777777-7777-4777-8777-777777777777",
				"event_type":"FinalResultImported","schema_version":1,
				"occurred_at":"2026-01-02T03:04:05Z",
				"producer_service":"assessment-service",
				"aggregate_id":"88888888-8888-4888-8888-888888888888",
				"aggregate_version":3,
				"result_id":"88888888-8888-4888-8888-888888888888",
				"offering_id":"66666666-6666-4666-8666-666666666666",
				"student_user_id":"44444444-4444-4444-8444-444444444444",
				"final_mark":75,"max_mark":100,
				"imported_at":"2026-01-02T03:04:05Z"
			}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var event EventEnvelope
			if err := json.Unmarshal([]byte(test.payload), &event); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			if err := event.validate(); err != nil {
				t.Fatalf("validate event: %v", err)
			}
		})
	}
}

func TestCreditWeightedGPARepeatPolicies(t *testing.T) {
	courseA, courseB := uuid.New(), uuid.New()
	first := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 6, 0)
	results := []ReportCourse{
		{CourseID: courseA, Credits: 4, GradePoints: float64Pointer(3), ImportedAt: first},
		{CourseID: courseA, Credits: 4, GradePoints: float64Pointer(2), ImportedAt: second},
		{CourseID: courseB, Credits: 2, GradePoints: float64Pointer(3), ImportedAt: second},
	}

	best, bestCount := calculateGPA(results, "best", "include", 2)
	if best == nil || *best != 3 || bestCount != 2 {
		t.Fatalf("best policy = %v, %d courses; want 3.00, 2", best, bestCount)
	}
	latest, latestCount := calculateGPA(results, "latest", "include", 2)
	if latest == nil || *latest != 2.33 || latestCount != 2 {
		t.Fatalf("latest policy = %v, %d courses; want 2.33, 2", latest, latestCount)
	}
	all, allCount := calculateGPA(results, "all", "include", 2)
	if all == nil || *all != 2.6 || allCount != 3 {
		t.Fatalf("all policy = %v, %d courses; want 2.60, 3", all, allCount)
	}
	excludeFailed, eligibleCount := calculateGPA(results, "latest", "exclude", 2.5)
	if excludeFailed == nil || *excludeFailed != 3 || eligibleCount != 1 {
		t.Fatalf("failed-result policy = %v, %d courses; want 3.00, 1", excludeFailed, eligibleCount)
	}
}

func TestGradeBandRanges(t *testing.T) {
	bands := []GradeBand{
		{Code: "A", MinimumPercent: 80, MaximumPercent: 100, GradePoints: 4, DisplayOrder: 1},
		{Code: "B", MinimumPercent: 60, MaximumPercent: 80, GradePoints: 3, DisplayOrder: 2},
	}
	if err := validateGradeBands(bands); err != nil {
		t.Fatalf("validate adjacent bands: %v", err)
	}
	if !bands[0].matches(100) || bands[0].matches(79.99) || !bands[1].matches(60) {
		t.Fatal("grade band boundary matching is incorrect")
	}
	bands[1].MaximumPercent = 81
	if err := validateGradeBands(bands); err == nil {
		t.Fatal("overlapping grade bands were accepted")
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}
