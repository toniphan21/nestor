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

type dockerContainer struct {
	name        string
	containerID string
	isRunning   bool
}

type dockerPage struct {
	*basePage

	index  int
	api    nestor.API
	images []dockerImage
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

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		for _, spec := range specs {
			h, profile, err := spec.FindHarnessAndProfile(runtime)
			if err != nil {
				continue
			}

			img := spec.DockerImageName(runtime.Template)

			image := dockerImage{
				spec:       spec.Name,
				image:      img,
				dockerfile: profile.Dockerfile(h, runtime),
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

func (p dockerPage) rowCount() int {
	var count int
	for _, v := range p.images {
		count += 1 + len(v.containers)
	}
	return count
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
		p.images = msg.images
		if p.index >= p.rowCount() {
			p.index = 0
		}

		return p, p.tick()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "j":
			if p.index < p.rowCount()-1 { // there is a header so just len is enough
				p.index = p.index + 1
			}
			return p, nil

		case "k":
			if p.index > 0 {
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
	if len(p.images) == 0 {
		return "there is no images or container created by nestor"
	}

	var nameW, idW, extraW int
	rows := []dockerRow{
		{name: " NAME", id: "ID", extra: "EXTRA"},
	}

	for _, v := range p.images {
		rows = append(rows, p.buildRows(v)...)
	}

	for _, v := range rows {
		nameW = max(nameW, widthOf(v.name))
		idW = max(idW, widthOf(v.id))
		extraW = max(extraW, widthOf(v.extra))
	}

	var out []string
	for i, v := range rows {
		cols := []string{
			v.name, pad(nameW - widthOf(v.name)),
			v.id, pad(idW - widthOf(v.id)),
			v.extra, pad(extraW - widthOf(v.extra)),
		}

		if p.index == i-1 {
			sel := pterm.NewStyle(pterm.BgGray)
			out = append(out, pad(2)+sel.Sprint(strings.Join(cols, " "))+pad(2))
		} else {
			out = append(out, pad(2)+strings.Join(cols, " ")+pad(2))
		}
	}
	return strings.Join(out, "\n")
}

func (p *dockerPage) buildRows(v dockerImage) []dockerRow {
	rows := []dockerRow{
		{
			name:  " " + v.image,
			id:    p.shortID(v.imageID),
			extra: v.dockerfile,
		},
	}

	for i, vv := range v.containers {
		row := dockerRow{
			id: p.shortID(vv.containerID),
		}

		if i == len(v.containers)-1 {
			row.name = pterm.Gray(" └─ ") + vv.name
		} else {
			row.name = pterm.Gray(" ├─ ") + vv.name
		}

		if vv.isRunning {
			row.extra = pterm.Green("running")
		} else {
			row.extra = pterm.Gray("stopped")
		}
		rows = append(rows, row)
	}

	return rows
}

func (p dockerPage) shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 8 {
		id = id[:8]
	}
	return id
}

type dockerRow struct {
	name  string
	id    string
	extra string
}
