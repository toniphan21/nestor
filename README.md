# nestor

A Go library with a built-in binary that runs `Claude Code` or `OpenCode` inside a sandboxed
container, interactive or headless, with git worktree support.

Credentials stay on the host: an optional proxy injects auth on the way out, so the agent runs
with no secrets in the container. Use the binary as-is, or build your own orchestrator on the
library.

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

Profiles and sandbox specs do not have to come from disk — pass them directly with
`WithProfiles()` and `WithSandboxSpecs()`.

## Contributing & License

PRs are welcome! See [CONTRIBUTING](CONTRIBUTING.md). Distributed under the Apache License 2.0.