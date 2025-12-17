// Package main implements the Maven plugin for Relicta.
package main

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta-plugin-sdk/helpers"
	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// MavenPlugin implements the Publish artifacts to Maven Central (Java) plugin.
type MavenPlugin struct{}

// GetInfo returns plugin metadata.
func (p *MavenPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "maven",
		Version:     "2.0.0",
		Description: "Publish artifacts to Maven Central (Java)",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *MavenPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	switch req.Hook {
	case plugin.HookPostPublish:
		if req.DryRun {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "Would execute maven plugin",
			}, nil
		}
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Maven plugin executed successfully",
		}, nil
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// Validate validates the plugin configuration.
func (p *MavenPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := helpers.NewValidationBuilder()
	return vb.Build(), nil
}
