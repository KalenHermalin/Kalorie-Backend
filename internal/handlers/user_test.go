package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"main/internal/apperrors"
	"main/internal/models"
	"main/internal/service"
	"main/internal/utils"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type errorResponse struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Fields  []apperrors.FieldError `json:"fields,omitempty"`
}

func TestEgressSyncWeightLogsDateFailure(t *testing.T) {

	tests := []struct {
		name           string
		input          []*models.WeightLog
		expectedFields []string
	}{
		{"Invalid Month", []*models.WeightLog{
			{ID: "invalid_month",
				WeightKg:  2,
				LogDate:   "2026-13-04",
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
		},
			[]string{
				"log_date",
			},
		},
		{"Invalid Day(Every Month)", []*models.WeightLog{
			{ID: "invalid_day(EM)",
				WeightKg:  2,
				LogDate:   "2026-12-35",
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
		},
			[]string{
				"log_date",
			},
		},
		{"Invalid Day (Specfic Month)", []*models.WeightLog{
			{ID: "invalid_day(SP)",
				WeightKg:  2,
				LogDate:   "2026-02-30",
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
		},
			[]string{
				"log_date",
			},
		},
		{"Invalid Year (5 Digits)", []*models.WeightLog{
			{ID: "invalid_year(5D)",
				WeightKg:  2,
				LogDate:   "20265-12-04",
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
		},
			[]string{
				"log_date",
			},
		},
		{"Invalid Year (3 Digits)", []*models.WeightLog{
			{ID: "invalid_year(3D)",
				WeightKg:  2,
				LogDate:   "206-12-04",
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
		},
			[]string{
				"log_date",
			},
		},
		{"Wrong Formatter", []*models.WeightLog{
			{ID: "wrong_format",
				WeightKg:  2,
				LogDate:   "2026/11/06",
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
		},
			[]string{
				"log_date",
			},
		},
		{
			"Null w/ Wrong Format", []*models.WeightLog{
				{
					ID:        "wrong_format_2",
					WeightKg:  2,
					LogDate:   "2026/11/06",
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				nil,
			},
			[]string{
				"log_date",
				"weight_logs",
			},
		},
		{
			"Invalid Date w/ Wrong Format", []*models.WeightLog{
				{
					ID:        "wrong_format_3",
					WeightKg:  2,
					LogDate:   "2026/11/06",
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				{
					ID:        "invalid_day(EM)_2",
					WeightKg:  2,
					LogDate:   "2026-12-35",
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
			},
			[]string{
				"log_date",
				"log_date",
			},
		},
		{
			"Invalid Date w/ Valid /w Wrong Format", []*models.WeightLog{
				{
					ID:        "wrong_format_4",
					WeightKg:  2,
					LogDate:   "2026/11/06",
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				{
					ID:        "invalid_day(EM)_3",
					WeightKg:  2,
					LogDate:   "2026-12-35",
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				{
					ID:        "Valid",
					WeightKg:  2,
					LogDate:   "2026-11-06",
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
			},
			[]string{
				"log_date",
				"log_date",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requestBody := SyncWeightLogsRequestBody{}
			requestBody.WeightLogs = tc.input
			body, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf("Json Marshal Error: %v", err)
			}
			req := httptest.NewRequest(http.MethodPut, "/weight-logs/sync", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			responseRecorder := httptest.NewRecorder()
			us := service.NewUserService(nil)
			h := NewUserHandler(*us)
			hfun := Handle(h.EgressSyncWeightLogs)
			hfun(responseRecorder, req)

			if responseRecorder.Code != 400 {
				t.Fatalf("Response code not a 400, got instead:%d", responseRecorder.Code)
			}

			var resp errorResponse
			err = json.NewDecoder(responseRecorder.Body).Decode(&resp)
			if err != nil {
				t.Fatalf("Error decoding errorResponse: %v", err)
			}
			if resp.Code != apperrors.BadRequest.Code {
				t.Fatalf("Response code was not bad request. Got instead: %s", resp.Code)
			}

			if err = utils.CheckValidString(resp.Message); err != nil {
				t.Fatalf("Message is blank. Message: %v", err)
			}

			if len(resp.Fields) == 0 {
				t.Fatalf("No fields returned")
			}
			if len(resp.Fields) != len(tc.expectedFields) {
				t.Fatalf("Number of response fields is not the same as expected. Expected: %d | Got: %d", len(tc.expectedFields), len(resp.Fields))
			}
			for i, ef := range tc.expectedFields {
				if resp.Fields[i].Field != ef {
					t.Fatalf("Field is not %s. Got instead: %s", ef, resp.Fields[i].Field)
				}
				if tc.input[i] != nil {
					if !strings.Contains(resp.Fields[i].Problem, tc.input[i].ID) {
						t.Fatalf("Problem returned wrong log id. Expected: %s | Got: %s", tc.input[i].ID, resp.Fields[i].Problem)
					}

				} else {
					if !strings.Contains(resp.Fields[i].Problem, fmt.Sprintf("index %d", i)) {
						t.Fatalf("Problem returned wrong idex. Expected: index %d | Got: %s", i, resp.Fields[i].Problem)
					}
				}
			}

			t.Logf("Response: %+v", resp)
		})

	}

}
