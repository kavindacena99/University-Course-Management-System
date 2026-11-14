package course

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateJSONRoundTrip(t *testing.T) {
	item := NewDate(time.Date(2026, 9, 1, 15, 30, 0, 0, time.FixedZone("local", 3600)))
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal date: %v", err)
	}
	if string(encoded) != `"2026-09-01"` {
		t.Fatalf("encoded = %s", encoded)
	}

	var decoded Date
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal date: %v", err)
	}
	if !sameDate(item, decoded) {
		t.Fatalf("decoded = %s", decoded)
	}

	if err := json.Unmarshal([]byte(`"09/01/2026"`), &decoded); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestLifecycleTransitions(t *testing.T) {
	courseTransitions := map[CourseStatus][]CourseStatus{
		CourseDraft:    {CourseActive, CourseArchived},
		CourseActive:   {CourseArchived},
		CourseArchived: {CourseActive},
	}
	for from, allowed := range courseTransitions {
		for _, status := range []CourseStatus{CourseDraft, CourseActive, CourseArchived} {
			want := containsCourseStatus(allowed, status)
			if from.CanTransitionTo(status) != want {
				t.Errorf("%s -> %s = %t, want %t",
					from, status, from.CanTransitionTo(status), want)
			}
		}
	}

	offeringTransitions := map[OfferingStatus][]OfferingStatus{
		OfferingDraft:     {OfferingPublished, OfferingArchived},
		OfferingPublished: {OfferingClosed, OfferingArchived},
		OfferingClosed:    {OfferingArchived},
		OfferingArchived:  {OfferingDraft},
	}
	for from, allowed := range offeringTransitions {
		for _, status := range []OfferingStatus{
			OfferingDraft, OfferingPublished, OfferingClosed, OfferingArchived,
		} {
			want := containsOfferingStatus(allowed, status)
			if from.CanTransitionTo(status) != want {
				t.Errorf("%s -> %s = %t, want %t",
					from, status, from.CanTransitionTo(status), want)
			}
		}
	}
}

func TestActionTransitions(t *testing.T) {
	if next, ok := courseTransitionForAction("publish"); !ok || next != CourseActive {
		t.Fatalf("publish = %s, %t", next, ok)
	}
	if next, ok := courseTransitionForAction("archive"); !ok || next != CourseArchived {
		t.Fatalf("archive = %s, %t", next, ok)
	}
	if _, ok := courseTransitionForAction("unknown"); ok {
		t.Fatal("expected unknown course action rejection")
	}
	if next, ok := offeringTransitionForAction("restore"); !ok || next != OfferingDraft {
		t.Fatalf("restore = %s, %t", next, ok)
	}
	if _, ok := offeringTransitionForAction("unknown"); ok {
		t.Fatal("expected unknown offering action rejection")
	}
}

func containsCourseStatus(values []CourseStatus, target CourseStatus) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsOfferingStatus(values []OfferingStatus, target OfferingStatus) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
