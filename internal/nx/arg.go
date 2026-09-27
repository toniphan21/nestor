package nx

import (
	"context"
	"log/slog"
	"os/exec"
)

type ArgsBuilder struct {
	args   []string
	logged []string
}

func (b *ArgsBuilder) Add(args ...string) *ArgsBuilder {
	for _, v := range args {
		b.args = append(b.args, v)
		b.logged = append(b.logged, v)
	}
	return b
}

func (b *ArgsBuilder) AddPair(exec string, log string) *ArgsBuilder {
	b.args = append(b.args, exec)
	b.logged = append(b.logged, log)
	return b
}

func (b *ArgsBuilder) Values() []string {
	return b.args
}

func (b *ArgsBuilder) ToCommand(cmd string) *exec.Cmd {
	return exec.Command(cmd, b.args...)
}

func (b *ArgsBuilder) ToCommandContext(ctx context.Context, cmd string) *exec.Cmd {
	return exec.CommandContext(ctx, cmd, b.args...)
}

func (b *ArgsBuilder) Log(logger *slog.Logger, group string) *slog.Logger {
	return logger.WithGroup(group).With(slog.Any("args", b.logged))
}
