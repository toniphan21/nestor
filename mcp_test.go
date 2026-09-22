package nestor

import (
	"strings"
	"testing"
)

func Test_MCPToolPolicy_String(t *testing.T) {
	tests := []struct {
		name   string
		policy MCPToolPolicy
		want   string
	}{
		{"all", AllMCPTools(), "all"},
		{"none", UseMCPTools(), "allow=[]"},
		{"one", UseMCPTools("issue_read"), "allow=[issue_read]"},
		{"sorted", UseMCPTools("list_issues", "issue_read"), "allow=[issue_read list_issues]"},
		{"deduped", UseMCPTools("issue_read", "issue_read"), "allow=[issue_read]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_MCP_String(t *testing.T) {
	tests := []struct {
		name string
		mcp  MCP
		want string
	}{
		{
			name: "local",
			mcp: LocalMCP("gh",
				[]string{"github-mcp-server", "stdio"},
				map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_secret"},
				UseMCPTools("issue_read"),
			),
			want: "gh [local], command: [github-mcp-server stdio], " +
				"env: [GITHUB_PERSONAL_ACCESS_TOKEN], tools: allow=[issue_read]",
		},
		{
			name: "local without env",
			mcp:  LocalMCP("gh", []string{"github-mcp-server", "stdio"}, nil, AllMCPTools()),
			want: "gh [local], command: [github-mcp-server stdio], env: [], tools: all",
		},
		{
			name: "remote",
			mcp: RemoteMCP("github",
				"https://api.githubcopilot.com/mcp/",
				map[string]string{"Authorization": "Bearer ghp_secret", "X-MCP-Toolsets": "issues"},
				AllMCPTools(),
			),
			want: "github [remote], url: https://api.githubcopilot.com/mcp/, " +
				"headers: [Authorization X-MCP-Toolsets], tools: all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mcp.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_MCP_String_HidesSecrets(t *testing.T) {
	const secret = "ghp_secret"

	mcps := []MCP{
		LocalMCP("gh", []string{"github-mcp-server"},
			map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": secret}, AllMCPTools()),
		RemoteMCP("github", "https://api.githubcopilot.com/mcp/",
			map[string]string{"Authorization": "Bearer " + secret}, AllMCPTools()),
	}

	for _, m := range mcps {
		if s := m.String(); strings.Contains(s, secret) {
			t.Errorf("%s leaked a secret: %q", m.Name(), s)
		}
	}
}
