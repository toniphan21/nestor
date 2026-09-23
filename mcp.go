package nestor

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const ProxyMCPEndpoint = "/mcp"

type MCPToolPolicy interface {
	PermitTool(name string) bool

	String() string
}

func AllMCPTools() MCPToolPolicy {
	return &allMCPTools{}
}

type allMCPTools struct {
}

func (p *allMCPTools) PermitTool(name string) bool {
	return true
}

func (p *allMCPTools) String() string {
	return "all"
}

func UseMCPTools(names ...string) MCPToolPolicy {
	allows := make(map[string]bool)
	for _, v := range names {
		allows[v] = true
	}
	return &useMCPTools{allows: allows}
}

type useMCPTools struct {
	allows map[string]bool
}

func (p *useMCPTools) PermitTool(name string) bool {
	return p.allows[name]
}

func (p *useMCPTools) String() string {
	return fmt.Sprintf("allow=%v", slices.Sorted(maps.Keys(p.allows)))
}

// ---

type MCP interface {
	Name() string
	ToolPolicy() MCPToolPolicy
	String() string

	connect(ctx context.Context) (*mcpUpstream, error)
}

func LocalMCP(name string, command []string, env map[string]string, toolPolicy MCPToolPolicy) MCP {
	return &localMCP{name: name, command: command, env: env, toolPolicy: toolPolicy}
}

type localMCP struct {
	name       string
	command    []string
	env        map[string]string
	toolPolicy MCPToolPolicy
}

func (s *localMCP) Name() string {
	return s.name
}

func (s *localMCP) ToolPolicy() MCPToolPolicy {
	return s.toolPolicy
}

func (s *localMCP) String() string {
	return fmt.Sprintf(
		"%s [local], command: %v, env: %v, tools: %v",
		s.name, s.command, slices.Sorted(maps.Keys(s.env)), s.toolPolicy,
	)
}

func (s *localMCP) connect(ctx context.Context) (*mcpUpstream, error) {
	cmd := exec.Command(s.command[0], s.command[1:]...)
	cmd.Stderr = os.Stderr // TODO: set an writer for proxy

	osEnv := os.Environ()
	var env []string
	for k, v := range s.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(osEnv, env...)

	return connect(ctx, s.name, s.toolPolicy, &mcp.CommandTransport{Command: cmd})
}

func RemoteMCP(name string, url string, headers map[string]string, toolPolicy MCPToolPolicy) MCP {
	return &remoteMCP{name: name, url: url, headers: headers, toolPolicy: toolPolicy}
}

type remoteMCP struct {
	name       string
	url        string
	headers    map[string]string
	toolPolicy MCPToolPolicy
}

func (s *remoteMCP) Name() string {
	return s.name
}

func (s *remoteMCP) ToolPolicy() MCPToolPolicy {
	return s.toolPolicy
}

func (s *remoteMCP) String() string {
	return fmt.Sprintf(
		"%s [remote], url: %v, headers: %v, tools: %v",
		s.name, s.url, slices.Sorted(maps.Keys(s.headers)), s.toolPolicy,
	)
}

func (s *remoteMCP) connect(ctx context.Context) (*mcpUpstream, error) {
	headers := http.Header{}
	for k, v := range s.headers {
		headers.Set(k, v)
	}

	return connect(ctx, s.name, s.toolPolicy, &mcp.StreamableClientTransport{
		Endpoint: s.url,
		HTTPClient: &http.Client{ // no Timeout: it would cut long-lived SSE streams
			Transport: &remoteMCPHeaderTransport{base: http.DefaultTransport, headers: headers},
		},
	})
}

// remoteMCPHeaderTransport sets static headers on every upstream request.
type remoteMCPHeaderTransport struct {
	base    http.RoundTripper
	headers http.Header
}

func (t *remoteMCPHeaderTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context()) // RoundTripper must not mutate the caller's request
	for k, vs := range t.headers {
		r.Header[k] = vs
	}
	return t.base.RoundTrip(r)
}

// ---

func connect(ctx context.Context, name string, tp MCPToolPolicy, t mcp.Transport) (*mcpUpstream, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "nestor", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", name, err)
	}
	return &mcpUpstream{Name: name, Session: cs, ToolPolicy: tp}, nil
}

type mcpUpstream struct {
	Name       string
	Session    *mcp.ClientSession
	ToolPolicy MCPToolPolicy
}

func (u *mcpUpstream) mirror(ctx context.Context, srv *mcp.Server) (int, error) {
	n := 0
	for t, err := range u.Session.Tools(ctx, nil) {
		if err != nil {
			return n, fmt.Errorf("list tools %s: %w", u.Name, err)
		}

		if !u.ToolPolicy.PermitTool(t.Name) {
			continue
		}

		local := *t
		local.Name = u.Name + "_" + t.Name
		srv.AddTool(&local, u.forward(t.Name))
		n++
	}
	return n, nil
}

func (u *mcpUpstream) forward(name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return u.Session.CallTool(ctx, &mcp.CallToolParams{
			Name:      name,
			Arguments: req.Params.Arguments,
		})
	}
}

// ---

func mcpProxyHandler(ctx context.Context, runtime Runtime, names []string, log *slog.Logger) (http.Handler, error) {
	var mcps []MCP
	for _, n := range names {
		v, ok := runtime.Registry.MCP(n)
		if !ok {
			continue
		}
		mcps = append(mcps, v)
	}

	if len(mcps) == 0 {
		return nil, nil
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "nestor-mcp-proxy", Version: "v" + Version}, nil)
	for _, m := range mcps {
		u, err := m.connect(ctx)
		if err != nil {
			return nil, err
		}

		if _, err = u.mirror(ctx, server); err != nil {
			return nil, err
		}
	}

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil), nil
}
