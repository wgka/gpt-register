package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestExecuteOnlineAccountsMaintenance(t *testing.T) {
	var (
		mu            sync.Mutex
		disabledNames []string
		deletedNames  []string
	)

	files := []map[string]any{
		{
			"id":             "1",
			"name":           "alpha.json",
			"account":        "alpha@example.com",
			"email":          "alpha@example.com",
			"disabled":       false,
			"status_message": map[string]any{"error": map[string]any{"code": "token_invalidated"}},
		},
		{
			"id":             "2",
			"name":           "beta.json",
			"account":        "beta@example.com",
			"email":          "beta@example.com",
			"disabled":       true,
			"status_message": `{"detail":{"code":"deactivated_workspace"}}`,
		},
		{
			"id":             "3",
			"name":           "gamma.json",
			"account":        "gamma@example.com",
			"email":          "gamma@example.com",
			"disabled":       false,
			"status_message": map[string]any{"error": map[string]any{"code": "ok"}},
		},
	}

	previousFactory := onlineAccountsHTTPClientFactory
	onlineAccountsHTTPClientFactory = func(string, time.Duration) *http.Client {
		return &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v0/management/auth-files":
					body, err := json.Marshal(map[string]any{"files": files})
					if err != nil {
						t.Fatalf("marshal files payload: %v", err)
					}
					return jsonResponse(http.StatusOK, body), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v0/management/auth-files/status":
					var payload struct {
						Name     string `json:"name"`
						Disabled bool   `json:"disabled"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode patch payload: %v", err)
					}
					if !payload.Disabled {
						t.Fatalf("expected disable=true, got false for %s", payload.Name)
					}
					mu.Lock()
					disabledNames = append(disabledNames, payload.Name)
					mu.Unlock()
					return jsonResponse(http.StatusOK, []byte(`{}`)), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/v0/management/auth-files":
					name := req.URL.Query().Get("name")
					mu.Lock()
					deletedNames = append(deletedNames, name)
					mu.Unlock()
					return jsonResponse(http.StatusOK, []byte(`{}`)), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		}
	}
	defer func() {
		onlineAccountsHTTPClientFactory = previousFactory
	}()

	t.Setenv("APP_CPA_API_URL", "https://example.com")
	t.Setenv("APP_CPA_API_TOKEN", "token")
	t.Setenv("APP_CPA_PROXY_URL", "")

	result, err := ExecuteOnlineAccountsMaintenance(context.Background(), OnlineAccountsScheduleConfig{
		DeleteInvalid: true,
	})
	if err != nil {
		t.Fatalf("ExecuteOnlineAccountsMaintenance returned error: %v", err)
	}

	if result.InvalidFound != 2 {
		t.Fatalf("expected 2 invalid accounts, got %d", result.InvalidFound)
	}
	if result.DisabledCount != 0 {
		t.Fatalf("expected 0 disabled account, got %d", result.DisabledCount)
	}
	if result.DeletedCount != 2 {
		t.Fatalf("expected 2 deleted accounts, got %d", result.DeletedCount)
	}
	if result.FailedCount != 0 {
		t.Fatalf("expected 0 failed operations, got %d", result.FailedCount)
	}

	slices.Sort(disabledNames)
	slices.Sort(deletedNames)

	if !slices.Equal(disabledNames, []string{}) {
		t.Fatalf("unexpected disabled names: %#v", disabledNames)
	}
	if !slices.Equal(deletedNames, []string{"alpha.json", "beta.json"}) {
		t.Fatalf("unexpected deleted names: %#v", deletedNames)
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func TestNormalizeOnlineAccountsManagementEndpoint(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "base url gets management path",
			in:   "https://example.com",
			want: "https://example.com/v0/management/auth-files",
		},
		{
			name: "existing path preserved",
			in:   "https://example.com/custom/path/",
			want: "https://example.com/custom/path",
		},
		{
			name: "invalid url returns empty",
			in:   "://bad",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOnlineAccountsManagementEndpoint(tt.in); got != tt.want {
				t.Fatalf("normalizeOnlineAccountsManagementEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeFixedTimes(t *testing.T) {
	got := normalizeFixedTimes([]string{"9:00", "09:00", "18:30", "bad", "24:00", "03:05"})
	want := []string{"03:05", "09:00", "18:30"}
	if !slices.Equal(got, want) {
		t.Fatalf("normalizeFixedTimes() = %#v, want %#v", got, want)
	}
}

func TestNextRegularRunFixedTimes(t *testing.T) {
	base := time.Date(2026, 3, 24, 10, 15, 0, 0, time.FixedZone("CST", 8*3600))
	next, reason := nextRegularRun(base, OnlineAccountsScheduleConfig{
		Enabled:       true,
		Mode:          OnlineAccountsScheduleModeFixedTimes,
		FixedTimes:    []string{"09:00", "11:30", "20:00"},
		DeleteInvalid: true,
	})

	if reason != OnlineAccountsScheduleModeFixedTimes {
		t.Fatalf("expected fixed_times reason, got %q", reason)
	}

	localNext := next.In(base.Location())
	if localNext.Hour() != 11 || localNext.Minute() != 30 {
		t.Fatalf("expected next fixed run at 11:30, got %s", localNext.Format(time.RFC3339))
	}
}
