package assessment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	maxFinalResultImportBytes = 5 << 20
	maxFinalResultImportRows  = 1000
)

func (s *AssessmentService) ImportFinalResults(
	ctx context.Context,
	actor Actor,
	offeringID uuid.UUID,
	idempotencyKey string,
	content []byte,
	correlationID string,
) (*FinalResultImportBatch, error) {
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if !actor.HasRole(RoleAdmin) || !actor.HasPermission(PermAssessmentResultImport) {
		return nil, ErrForbidden
	}
	if offeringID == uuid.Nil {
		return nil, newValidationError("offering_id", "must be a valid UUID")
	}
	if !validIdempotencyKey(idempotencyKey) {
		return nil, newValidationError("Idempotency-Key", "must contain 8 to 128 safe characters")
	}
	if len(content) == 0 || len(content) > maxFinalResultImportBytes {
		return nil, newValidationError("file", "must be a non-empty CSV file no larger than 5 MiB")
	}
	results, err := parseFinalResultCSV(content)
	if err != nil {
		return nil, err
	}
	digest := sha256Sum(content)
	if strings.TrimSpace(correlationID) == "" {
		correlationID = uuid.NewString()
	}
	return s.store.ImportFinalResults(
		ctx, offeringID, actor.UserID, idempotencyKey, digest, results, correlationID,
	)
}

func (s *AssessmentService) ListFinalResults(
	ctx context.Context,
	actor Actor,
	offeringID uuid.UUID,
	limit, offset int,
) (*FinalResultList, error) {
	if offeringID == uuid.Nil {
		return nil, newValidationError("offering_id", "must be a valid UUID")
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, newValidationError("limit", "must be between 1 and 100")
	}
	if offset < 0 || offset > 1_000_000 {
		return nil, newValidationError("offset", "must be between 0 and 1000000")
	}
	var studentID uuid.UUID
	switch {
	case actor.HasRole(RoleAdmin) && actor.HasPermission(PermAssessmentStudentRead):
	case actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentRead):
		if err := s.ensureAssignedLecturer(ctx, actor.UserID, offeringID); err != nil {
			return nil, err
		}
	case actor.HasRole(RoleStudent) && actor.HasPermission(PermAssessmentSubmit):
		studentID = actor.UserID
	default:
		return nil, ErrForbidden
	}
	return s.store.ListFinalResults(ctx, offeringID, studentID, limit, offset)
}

func parseFinalResultCSV(content []byte) ([]FinalResultInput, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = 3
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = false
	header, err := reader.Read()
	if err != nil {
		return nil, newValidationError("file", "must contain a CSV header and at least one result")
	}
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}
	header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	if header[0] != "student_user_id" || header[1] != "final_mark" || header[2] != "max_mark" {
		return nil, newValidationError("file", "CSV header must be student_user_id,final_mark,max_mark")
	}
	records, err := reader.ReadAll()
	if err != nil {
		return nil, newValidationError("file", "contains an invalid CSV record")
	}
	if len(records) == 0 || len(records) > maxFinalResultImportRows {
		return nil, newValidationError("file", "must contain between 1 and 1000 result rows")
	}
	results := make([]FinalResultInput, 0, len(records))
	seenStudents := make(map[uuid.UUID]struct{}, len(records))
	for row, record := range records {
		studentID, err := uuid.Parse(strings.TrimSpace(record[0]))
		if err != nil || studentID == uuid.Nil {
			return nil, newValidationError("file", fmt.Sprintf("row %d has an invalid student_user_id", row+2))
		}
		if _, exists := seenStudents[studentID]; exists {
			return nil, newValidationError("file", fmt.Sprintf("row %d duplicates a student_user_id", row+2))
		}
		seenStudents[studentID] = struct{}{}
		finalMark, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		if err != nil || !validHundredth(finalMark) || finalMark < 0 {
			return nil, newValidationError("file", fmt.Sprintf("row %d has an invalid final_mark", row+2))
		}
		maxMark, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
		if err != nil || !validHundredth(maxMark) || maxMark <= 0 ||
			maxMark > 999999.99 || finalMark > maxMark {
			return nil, newValidationError("file", fmt.Sprintf("row %d has an invalid max_mark", row+2))
		}
		results = append(results, FinalResultInput{
			StudentUserID: studentID, FinalMark: finalMark, MaxMark: maxMark,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].StudentUserID.String() < results[j].StudentUserID.String()
	})
	return results, nil
}

func sha256Sum(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
