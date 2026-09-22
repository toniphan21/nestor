package nestor

import (
	"fmt"
	"maps"
	"slices"
)

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
