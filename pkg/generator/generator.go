package generator

import (
	"embed"
	"fmt"
	"path"

	"github.com/ashb/oapi-resty-codegen/pkg/openapi31downgrade"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
)

//go:embed templates/*
var f embed.FS

type GenerateArgs struct {
	codegen.Configuration `yaml:",inline"`

	DowngradeOptions openapi31downgrade.Options `yaml:"downgrade-options"`

	// OutputFile is the filename to output.
	OutputFile string `yaml:"output,omitempty"`
	// Input filename is file (or URL) to read input from
	Input string `yaml:"input,omitempty"`
	// Spec is to provide a loaded spec already. Mutually exclusive with Input
	Spec *openapi3.T
}

func Generate(opts GenerateArgs) (string, error) {
	templates := map[string]string{}

	if err := opts.Validate(); err != nil {
		return "", fmt.Errorf("configuration error: %w", err)
	}

	overlayOpts := util.LoadSwaggerWithOverlayOpts{
		Path: opts.OutputOptions.Overlay.Path,
		// default to strict, but can be overridden
		Strict: true,
	}

	if opts.OutputOptions.Overlay.Strict != nil {
		overlayOpts.Strict = *opts.OutputOptions.Overlay.Strict
	}
	opts.AdditionalImports = append(opts.AdditionalImports, codegen.AdditionalImport{Package: "github.com/go-resty/resty/v2"})

	if opts.Input != "" {
		var err error
		opts.Spec, err = util.LoadSwaggerWithOverlay(opts.Input, overlayOpts)
		if err != nil {
			return "", fmt.Errorf("error loading swagger spec in %q\n: %w", opts.Input, err)
		}
	}
	if opts.Spec == nil {
		return "", fmt.Errorf("one of spec or input filename must be provided")
	}

	if opts.Spec.OpenAPI == "3.1.0" {
		var err error
		opts.Spec, err = openapi31downgrade.DowngradeTo3_0(opts.Spec, opts.DowngradeOptions)
		if err != nil {
			return "", fmt.Errorf("error downgrading spec in %q to OpenAPI 3.0.0\n: %w", opts.Input, err)
		}
	}

	dirName := "templates"
	entries, err := f.ReadDir(dirName)
	if err != nil {
		return "", fmt.Errorf("unable to read templates: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := f.ReadFile(path.Join(dirName, entry.Name()))
		if err != nil {
			return "", fmt.Errorf("unable to read template: %w", err)
		}
		templates[entry.Name()] = string(content)
	}
	opts.OutputOptions.UserTemplates = templates

	return codegen.Generate(opts.Spec, opts.Configuration)
}
