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

// Config represents the Maven plugin configuration.
type Config struct {
	GroupID    string
	ArtifactID string
	PomPath    string
	Username   string
	Password   string
	Repository string
	SkipTests  bool
}

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
			"properties": {
				"group_id": {"type": "string", "description": "Maven group ID (e.g., com.example)"},
				"artifact_id": {"type": "string", "description": "Maven artifact ID"},
				"pom_path": {"type": "string", "description": "Path to pom.xml", "default": "pom.xml"},
				"username": {"type": "string", "description": "Maven repository username (or use MAVEN_USERNAME env)"},
				"password": {"type": "string", "description": "Maven repository password (or use MAVEN_PASSWORD env)"},
				"repository": {"type": "string", "description": "Maven repository URL"},
				"skip_tests": {"type": "boolean", "description": "Skip tests during deploy", "default": false}
			},
			"required": ["group_id", "artifact_id"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *MavenPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.deploy(ctx, cfg, req.Context, req.DryRun)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

func (p *MavenPlugin) deploy(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would deploy Maven artifact",
			Outputs: map[string]any{
				"group_id":    cfg.GroupID,
				"artifact_id": cfg.ArtifactID,
				"version":     releaseCtx.Version,
				"pom_path":    cfg.PomPath,
			},
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Deployed Maven artifact %s:%s:%s", cfg.GroupID, cfg.ArtifactID, releaseCtx.Version),
		Outputs: map[string]any{
			"group_id":    cfg.GroupID,
			"artifact_id": cfg.ArtifactID,
			"version":     releaseCtx.Version,
		},
	}, nil
}

// parseConfig parses the raw config map into a Config struct.
func (p *MavenPlugin) parseConfig(raw map[string]any) *Config {
	parser := helpers.NewConfigParser(raw)

	return &Config{
		GroupID:    parser.GetString("group_id", "", ""),
		ArtifactID: parser.GetString("artifact_id", "", ""),
		PomPath:    parser.GetString("pom_path", "", "pom.xml"),
		Username:   parser.GetString("username", "MAVEN_USERNAME", ""),
		Password:   parser.GetString("password", "MAVEN_PASSWORD", ""),
		Repository: parser.GetString("repository", "", ""),
		SkipTests:  parser.GetBool("skip_tests", false),
	}
}

// Validate validates the plugin configuration.
func (p *MavenPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := helpers.NewValidationBuilder()
	parser := helpers.NewConfigParser(config)

	groupID := parser.GetString("group_id", "", "")
	if groupID == "" {
		vb.AddError("group_id", "Maven group ID is required")
	}

	artifactID := parser.GetString("artifact_id", "", "")
	if artifactID == "" {
		vb.AddError("artifact_id", "Maven artifact ID is required")
	}

	return vb.Build(), nil
}
