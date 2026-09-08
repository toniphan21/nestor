package nestor

import "testing"

func TestHarnessOpencodeCaptureSessionID(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   string
		wantOK bool
	}{
		{
			name:   "session line",
			line:   `{"type":"session.updated","sessionID":"ses_8a3f","time":{"created":1730000000}}`,
			want:   "ses_8a3f",
			wantOK: true,
		},
		{
			name: "wrong key casing",
			line: `{"session_id":"ses_8a3f"}`,
		},
		{
			name: "missing key",
			line: `{"type":"session.updated"}`,
		},
		{
			name: "empty value",
			line: `{"sessionID":""}`,
		},
		{
			name: "non-string value",
			line: `{"sessionID":123}`,
		},
		{
			name: "null value",
			line: `{"sessionID":null}`,
		},
		{
			name: "not json",
			line: `INFO  server listening on 127.0.0.1:4096`,
		},
		{
			name: "truncated json",
			line: `{"sessionID":"ses_8a3f"`,
		},
		{
			name: "json array",
			line: `["sessionID","ses_8a3f"]`,
		},
		{
			name: "empty line",
			line: ``,
		},
	}

	var h harnessOpenCode
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := h.CaptureSessionID([]byte(tt.line))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("id = %q, want %q", got, tt.want)
			}
		})
	}
}
