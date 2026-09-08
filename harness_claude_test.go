package nestor

import "testing"

func TestHarnessClaudeCaptureSessionID(t *testing.T) {
	const initLine = `{"type":"system","subtype":"init","cwd":"/chats","session_id":"60365672-05cf-4733-b167-fbc5430fcd85","model":"claude-sonnet-5","uuid":"db14fc74-f774-4ec0-ac2b-ee4878695c0c"}`

	tests := []struct {
		name   string
		line   string
		want   string
		wantOK bool
	}{
		{
			name:   "init line",
			line:   initLine,
			want:   "60365672-05cf-4733-b167-fbc5430fcd85",
			wantOK: true,
		},
		{
			name:   "assistant line",
			line:   `{"type":"assistant","session_id":"abc","message":{"role":"assistant"}}`,
			want:   "abc",
			wantOK: true,
		},
		{
			name: "missing key",
			line: `{"type":"system","subtype":"init"}`,
		},
		{
			name: "empty value",
			line: `{"session_id":""}`,
		},
		{
			name: "non-string value",
			line: `{"session_id":123}`,
		},
		{
			name: "null value",
			line: `{"session_id":null}`,
		},
		{
			name: "not json",
			line: `warning: something went wrong`,
		},
		{
			name: "truncated json",
			line: `{"session_id":"abc"`,
		},
		{
			name: "json array",
			line: `["session_id","abc"]`,
		},
		{
			name: "empty line",
			line: ``,
		},
	}

	var h harnessClaude
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
