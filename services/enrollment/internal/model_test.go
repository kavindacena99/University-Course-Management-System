package enrollment

import "testing"

func TestEnrollmentStatusTransitions(t *testing.T) {
	if !EnrollmentActive.CanTransitionTo(EnrollmentCancelled) {
		t.Fatal("active enrollment should be cancellable")
	}
	for _, transition := range [][2]EnrollmentStatus{
		{EnrollmentActive, EnrollmentActive},
		{EnrollmentCancelled, EnrollmentActive},
		{EnrollmentCancelled, EnrollmentCancelled},
	} {
		if transition[0].CanTransitionTo(transition[1]) {
			t.Fatalf("unexpected transition %q -> %q", transition[0], transition[1])
		}
	}
	if EnrollmentStatus("pending").Valid() {
		t.Fatal("unknown enrollment status must be rejected")
	}
}
