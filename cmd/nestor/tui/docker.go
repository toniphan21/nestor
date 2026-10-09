package tui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type dockerTickMsg struct{ gen int }

type dockerDataMsg struct {
	gen    int
	images []dockerImage
}

type dockerImage struct {
	spec       string
	image      string
	imageID    string
	dockerfile string
	containers []dockerContainer
}

func (v *dockerImage) toRows() []dockerRow {
	rows := []dockerRow{
		{
			name:  v.image,
			id:    v.shortID(v.imageID),
			extra: v.dockerfile,
			typ:   "image",
			data:  v,
		},
	}

	for i, vv := range v.containers {
		row := dockerRow{
			id:   v.shortID(vv.containerID),
			data: vv,
		}

		if i == len(v.containers)-1 {
			row.name = pterm.Gray("└─ ") + vv.name
		} else {
			row.name = pterm.Gray("├─ ") + vv.name
		}

		if vv.isRunning {
			row.extra = pterm.Green("running")
		} else {
			row.extra = pterm.Gray("stopped")
		}
		row.typ = "container"
		rows = append(rows, row)
	}

	return rows
}

func (v *dockerImage) shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		id = id[:12]
	}
	return id
}

type dockerContainer struct {
	name        string
	containerID string
	isRunning   bool
}

type dockerRow struct {
	name  string
	id    string
	extra string
	typ   string
	data  any
}

type dockerPage struct {
	*basePage

	index int
	api   nestor.API
	rows  []dockerRow
}

func (p *dockerPage) Focus() tea.Cmd {
	p.basePage.Focus()
	return p.fetch()
}

func (p dockerPage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		var images []dockerImage
		specs := p.api.Runtime().Registry.SandboxSpecs()
		runtime := p.api.Runtime()
		uhome := runtime.Platform.UserHomeDir()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		for _, spec := range specs {
			h, profile, err := spec.FindHarnessAndProfile(runtime)
			if err != nil {
				continue
			}

			img := spec.DockerImageName(runtime.Template)
			df := profile.Dockerfile(h, runtime)
			dfv, haveP := strings.CutPrefix(df, uhome)
			if haveP {
				df = "~" + dfv
			}

			image := dockerImage{
				spec:       spec.Name,
				image:      img,
				dockerfile: df,
				imageID:    runtime.Docker().ImageID(ctx, img),
			}

			sandboxes, err := p.api.ListSandboxes(ctx, spec.Name)
			if err != nil {
				images = append(images, image)
				continue
			}

			for _, v := range sandboxes {
				name := v.Container()
				ctn := dockerContainer{
					name:        name,
					containerID: runtime.Docker().ContainerID(ctx, name),
					isRunning:   v.IsRunning(ctx),
				}
				image.containers = append(image.containers, ctn)
			}

			slices.SortFunc(image.containers, func(a, b dockerContainer) int {
				return strings.Compare(a.name, b.name)
			})
			images = append(images, image)
		}

		slices.SortFunc(images, func(a, b dockerImage) int {
			return strings.Compare(a.image, b.image)
		})
		return dockerDataMsg{gen: gen, images: images}
	}
}

func (p dockerPage) tick() tea.Cmd {
	gen := p.gen
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return dockerTickMsg{gen: gen}
	})
}

func (p *dockerPage) Update(msg tea.Msg) (page, tea.Cmd) {
	switch msg := msg.(type) {
	case dockerTickMsg:
		if msg.gen != p.gen || !p.focused {
			return p, nil
		}
		return p, p.fetch()

	case dockerDataMsg:
		if msg.gen != p.gen {
			return p, nil
		}

		rows := []dockerRow{
			{name: " NAME", id: "ID", extra: "EXTRA", typ: "header"},
		}
		for _, v := range msg.images {
			rows = append(rows, v.toRows()...)
		}
		p.rows = rows

		if p.index <= 0 || p.index >= len(rows) {
			p.index = 1
		}
		return p, p.tick()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if p.index < len(p.rows)-1 {
				p.index = p.index + 1
			}
			return p, nil

		case "k", "up":
			if p.index > 1 {
				p.index = p.index - 1
			}
			return p, nil
		}
	}
	return p, nil
}

func (p *dockerPage) View() string {
	/*
	   NAME                     ID       EXTRA
	   nestor-opencode-go       168cb557 ~/.config/nestor/opencode/Dockerfile
	   |- nestor-sandbox-alpha  5d75ca36 running...
	   |-
	*/
	if len(p.rows) == 0 {
		return "there is no image or container created by nestor"
	}

	var nameW, idW, extraW int
	for _, v := range p.rows {
		nameW = max(nameW, widthOf(v.name))
		idW = max(idW, widthOf(v.id))
	}

	nameW += 1                             // pad 1 on the left
	extraW = p.width - 4 - nameW - idW + 1 // pad 1 on the right + 4 padding

	var out strings.Builder
	for i, v := range p.rows {
		cols := []string{
			fit(padLeft(v.name, 1), nameW),
			fit(v.id, idW),
			pad(fit(v.extra, extraW), 1),
		}

		row := strings.Join(cols, " ")
		if p.index == i {
			row = pterm.NewStyle(pterm.BgGray).Sprint(row)
		}

		out.WriteString(spaces(2))
		out.WriteString(row)
		out.WriteString(spaces(2))
		out.WriteRune('\n')
	}
	return out.String()
}
