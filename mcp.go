package nestor

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"time"

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

	connect(ctx context.Context, log *slog.Logger) (*mcpUpstream, error)
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

func (s *localMCP) connect(ctx context.Context, log *slog.Logger) (*mcpUpstream, error) {
	cmd := exec.Command(s.command[0], s.command[1:]...)
	//cmd.Stderr = os.Stderr // TODO: set an writer for proxy

	osEnv := os.Environ()
	var env []string
	for k, v := range s.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(osEnv, env...)

	return connect(ctx, s.name, s.toolPolicy, log, &mcp.CommandTransport{Command: cmd})
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

func (s *remoteMCP) connect(ctx context.Context, log *slog.Logger) (*mcpUpstream, error) {
	headers := http.Header{}
	for k, v := range s.headers {
		headers.Set(k, v)
	}

	return connect(ctx, s.name, s.toolPolicy, log, &mcp.StreamableClientTransport{
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
	maps.Copy(r.Header, t.headers)
	return t.base.RoundTrip(r)
}

// ---

func connect(ctx context.Context, name string, tp MCPToolPolicy, log *slog.Logger, t mcp.Transport) (*mcpUpstream, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: BinaryName, Version: "v" + Version}, nil)
	cs, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", name, err)
	}
	return &mcpUpstream{Name: name, Session: cs, ToolPolicy: tp, Log: log}, nil
}

type mcpUpstream struct {
	Name       string
	Session    *mcp.ClientSession
	ToolPolicy MCPToolPolicy
	Log        *slog.Logger
}

func (u *mcpUpstream) mirror(ctx context.Context, srv *mcp.Server) (int, error) {
	n := 0
	var tools []string
	for t, err := range u.Session.Tools(ctx, nil) {
		if err != nil {
			return n, fmt.Errorf("list tools %s: %w", u.Name, err)
		}

		if !u.ToolPolicy.PermitTool(t.Name) {
			continue
		}
		tools = append(tools, t.Name)

		local := *t
		local.Name = u.Name + "_" + t.Name
		srv.AddTool(&local, u.forward(t.Name))
		n++
	}

	u.Log.Info("mirror mcp upstream", slog.String("name", u.Name), slog.Any("tools", tools))
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
	// sort names is important, it ensures that MCP registered and proxied in the same order
	slices.Sort(names)

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

	server := mcp.NewServer(
		&mcp.Implementation{Name: BinaryName, Version: "v" + Version},
		&mcp.ServerOptions{Logger: log},
	)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if r, ok := req.(*mcp.InitializeRequest); ok && r.Params.ClientInfo != nil {
				log.Info("initialize",
					slog.String("client", r.Params.ClientInfo.Name),
					slog.String("version", r.Params.ClientInfo.Version),
					slog.String("protocol", r.Params.ProtocolVersion),
				)
			}

			var tool string
			if r, ok := req.(*mcp.CallToolRequest); ok && r.Params != nil {
				tool = r.Params.Name
			}

			msg := "mcp"
			attrs := []slog.Attr{slog.String("method", method)}
			if tool != "" {
				msg = "mcp " + tool
				attrs = append(attrs, slog.String("tool", tool))
			}

			start := time.Now()
			res, err := next(ctx, method, req)

			if err != nil {
				attrs = append(attrs, slog.Any("error", err))
				log.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
				return res, err
			}

			attrs = append(attrs, slog.Duration("duration", time.Since(start)))
			log.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
			return res, err
		}
	})

	for _, m := range mcps {
		u, err := m.connect(ctx, log)
		if err != nil {
			return nil, err
		}

		if _, err = u.mirror(ctx, server); err != nil {
			return nil, err
		}
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{DisableLocalhostProtection: true},
	)
	allowHosts := func(next http.Handler, hosts ...string) http.Handler {
		allowed := make(map[string]bool, len(hosts))
		for _, h := range hosts {
			allowed[h] = true
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if !allowed[host] {
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	return allowHosts(mcpHandler, DefaultHostAlias, "127.0.0.1", "localhost"), nil
}
