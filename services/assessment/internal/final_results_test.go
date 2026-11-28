package assessment

import "testing"

func TestParseFinalResultCSV(t *testing.T) {
	const studentID = "7e7e6a76-544e-4ad4-a6c1-813241a5e1b5"

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid raw marks",
			content: "student_user_id,final_mark,max_mark\n" + studentID + ",78.50,100\n",
		},
		{
			name:    "UTF-8 BOM header",
			content: "\uFEFFstudent_user_id,final_mark,max_mark\n" + studentID + ",78.50,100\n",
		},
		{
			name: "duplicate student",
			content: "student_user_id,final_mark,max_mark\n" +
				studentID + ",78,100\n" + studentID + ",81,100\n",
			wantErr: true,
		},
		{
			name:    "mark exceeds maximum",
			content: "student_user_id,final_mark,max_mark\n" + studentID + ",101,100\n",
			wantErr: true,
		},
		{
			name:    "mark has too many decimal places",
			content: "student_user_id,final_mark,max_mark\n" + studentID + ",78.501,100\n",
			wantErr: true,
		},
		{
			name:    "unsupported grade columns",
			content: "student_user_id,final_mark,max_mark,grade\n" + studentID + ",78,100,A\n",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results, err := parseFinalResultCSV([]byte(test.content))
			if (err != nil) != test.wantErr {
				t.Fatalf("parseFinalResultCSV() error = %v, wantErr %t", err, test.wantErr)
			}
			if !test.wantErr && len(results) != 1 {
				t.Fatalf("parseFinalResultCSV() returned %d results, want 1", len(results))
			}
		})
	}
}
