# nestor

A Go library with a built-in binary that runs `Claude Code` or `OpenCode` inside a sandboxed
container, interactive or headless, with git worktree support.

Credentials stay on the host: an optional proxy injects auth on the way out, so the agent runs
with no secrets in the container. This also covers MCP servers - the proxy adds MCP secrets on
the way out too, so the harness never sees them. Use the binary as-is, or build your own
orchestrator on the library.

## Use as standalone binary

### Installation

Download the archive for your platform from the [Releases](https://github.com/toniphan21/nestor/releases) page.

#### macOS

```bash
# arm64 (Apple Silicon); use nestor_darwin_amd64 on Intel
curl -sSfL https://github.com/toniphan21/nestor/releases/latest/download/nestor_darwin_arm64.tar.gz \
  | tar -xz nestor
sudo install -m 755 nestor /usr/local/bin/nestor
nestor version
```

#### Linux

```bash
# amd64; use nestor_linux_arm64 on arm
curl -sSfL https://github.com/toniphan21/nestor/releases/latest/download/nestor_linux_amd64.tar.gz \
  | tar -xz nestor
sudo install -m 755 nestor /usr/local/bin/nestor
nestor version
```

### Quick Usage

Use the `setup` subcommand to initialize nestor in your project:

```bash
# cd <to-your-project>

# nestor will ask a few questions and set it up for you
nestor setup
```

Setup writes everything into `./.nestor`: a Dockerfile per harness, `profile.yml` for
credentials and model metadata, and `sandbox.yml` for mounts and instance count. Edit those,
then `nestor down && nestor build` to pick up the changes.

Run your coding agent:

```bash
nestor launch
```

options
```bash
❯ go run ./cmd/nestor --help
Run coding agents in sandboxed containers

Usage:
  nestor [flags]
  nestor [command]

Available Commands:
  build       Build a sandbox image from a spec
  completion  Generate the autocompletion script for the specified shell
  delete      Remove a single sandbox
  destroy     Remove all sandboxes, images
  down        Stop all running sandboxes so the next run picks up config changes
  explain     Show the nestor architecture and how it works
  help        Help about any command
  launch      Start an interactive harness session in a sandbox
  prompt      Run a headless prompt in a sandbox (demo of library usage)
  release     Release the lease held on a sandbox
  rename      Give a sandbox a memorable name
  setup       Initialize the nestor directory
  version     Print the nestor version
  view        Show the current nestor state

Flags:
  -d, --dir string   nestor directory; use NESTOR_DIR if not specified
  -h, --help         help for nestor
```

### Configuration files

Full example of the three config files. `mcp.yml` is the MCP part: nestor can proxy MCP servers
to your agent, and the agent never sees MCP secrets - the proxy adds them on the way out, same
as it does for API keys.

1. List your MCP servers in `mcp.yml`.
2. Pick which ones a sandbox can use with the `mcp` field in `sandbox.yml`.

`profile.yml` - store credentials and model metadata:

```yaml
version: "1"
type: "profile"
data:
  claude-subscription:
    auth: credentials
    settings:
      CLAUDE_CODE_OAUTH_TOKEN: "your oauth token" # run 'claude setup-token' to get one
    default_target: base
    default_model: claude-sonnet-5
    targets:
      - base
      - go
    models:
      claude-opus-5: ["opus"]
      claude-sonnet-5: ["sonnet"]
      claude-haiku-4-5: ["haiku"]
```

`mcp.yml` - configure your MCP servers, local (stdio) or remote (http) - run on the host via a built-in proxy:

```yaml
version: "1"
type: "mcp"
data:
  gh: # name, also used as the tool prefix
    type: "local"
    command: [github-mcp-server, stdio, --toolsets, "repos,issues"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "ghp_xxx"

  github:
    type: "remote"
    url: "https://api.githubcopilot.com/mcp/"
    headers:
      Authorization: "Bearer ghp_xxx"
      X-MCP-Toolsets: "pull_requests"
    use_tools: [pull_request_read, list_pull_requests] # optional, pick tools; omit to allow all
```

`sandbox.yml` - pick harness, profile, mcp and project config:

```yaml
version: "1"
type: "sandbox-spec"
data:
  claude-go:
    harness: claude
    profile: claude-subscription
    mcp: [gh, github]
    state_scope: shared
    target: go
    max_instances: 3
    mounts:
      - type: git_worktree
        path: /your/project
        read_only: false
```

## Use as library

nestor can be used as a library to build your own agent orchestrator:

```go
package main

import (
	"log"

	"nhatp.com/go/nestor"
)

func main() {
	options := []nestor.Option{}
	api, err := nestor.New(options...)
	if err != nil {
		log.Fatal(err.Error())
	}

	...
}
```

Profiles, MCP and sandbox specs do not have to come from disk — pass them directly with
`WithProfiles()`, `WithMCP()` and `WithSandboxSpecs()`.

## Contributing & License

PRs are welcome! See [CONTRIBUTING](CONTRIBUTING.md). Distributed under the Apache License 2.0.