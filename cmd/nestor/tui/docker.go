package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
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
	name      string
	id        string
	isRunning bool
}

type dockerPage struct {
	busy    bool
	focused bool
	gen     int
	api     nestor.API
	images  []dockerImage
}

func (p *dockerPage) Focus() tea.Cmd {
	p.focused = true
	p.gen++
	return p.fetch()
}

func (p *dockerPage) Blur() {
	p.focused = false
	p.gen++
}

func (p *dockerPage) Busy() bool {
	return p.busy
}

func (p dockerPage) fetch() tea.Cmd {
	gen := p.gen
	return func() tea.Msg {
		var images []dockerImage
		specs := p.api.Runtime().Registry.SandboxSpecs()
		runtime := p.api.Runtime()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		for _, spec := range specs {
			h, p, err := spec.FindHarnessAndProfile(runtime)
			if err != nil {
				return err
			}

			img := spec.DockerImageName(runtime.Template)
			images = append(images, dockerImage{
				spec:       spec.Name,
				image:      img,
				dockerfile: p.Dockerfile(h, runtime),
				imageID:    runtime.Docker().ImageID(ctx, img),
			})
		}
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
		p.images = msg.images

		return p, p.tick()
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
		{name: "NAME", id: "ID", extra: "EXTRA"},
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
	for _, v := range rows {
		cols := []string{
			v.name, pad(nameW - widthOf(v.name)),
			v.id, pad(idW - widthOf(v.id)),
			v.extra, pad(extraW - widthOf(v.extra)),
		}
		out = append(out, strings.Join(cols, " "))
	}
	return strings.Join(out, "\n")
}

func (p *dockerPage) buildRows(v dockerImage) []dockerRow {
	rows := []dockerRow{
		{
			name:  v.image,
			id:    p.shortID(v.imageID),
			extra: v.dockerfile,
		},
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
