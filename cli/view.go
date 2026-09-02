package cli

import (
	"nhatp.com/go/nestor"
)

func ListSpecs(api nestor.API) (ListSpecsResult, error) {
	var result ListSpecsResult
	runtime := api.Runtime()

	result.NestorDir = runtime.Platform.NestorDir()
	result.SpecFilePath = runtime.Platform.SandboxYmlFile()
	result.ProfileFilePath = runtime.Platform.ProfileYmlFile()
	result.Specs = runtime.Registry.SandboxSpecs()

	return result, nil
}

type ListSpecsResult struct {
	NestorDir       string
	SpecFilePath    string
	ProfileFilePath string
	Specs           []nestor.SandboxSpec
}

func (r *ListSpecsResult) Print() {
}
