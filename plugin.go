// Package main implements the Maven plugin for Relicta.
package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta-plugin-sdk/helpers"
	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// Security validation patterns.
var (
	// Maven coordinates pattern: alphanumerics, dots, dashes, underscores.
	mavenCoordinatePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

	// Profile name pattern: alphanumerics, dashes, underscores.
	profilePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

// CommandExecutor abstracts command execution for testability.
type CommandExecutor interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// RealCommandExecutor executes actual system commands.
type RealCommandExecutor struct{}

// Run executes a command and returns combined output.
func (e *RealCommandExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// MavenPlugin implements the Publish artifacts to Maven Central (Java) plugin.
type MavenPlugin struct {
	executor CommandExecutor
}

// getExecutor returns the command executor, defaulting to RealCommandExecutor.
func (p *MavenPlugin) getExecutor() CommandExecutor {
	if p.executor != nil {
		return p.executor
	}
	return &RealCommandExecutor{}
}

// Config represents the Maven plugin configuration.
type Config struct {
	GroupID    string
	ArtifactID string
	PomPath    string
	Username   string
	Password   string
	Repository string
	SkipTests  bool
	Settings   string
	Profiles   []string
}

// validateMavenCoordinate validates a Maven group ID or artifact ID.
func validateMavenCoordinate(coord, fieldName string) error {
	if coord == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	if len(coord) > 256 {
		return fmt.Errorf("%s too long (max 256 characters)", fieldName)
	}
	if !mavenCoordinatePattern.MatchString(coord) {
		return fmt.Errorf("invalid %s: contains disallowed characters", fieldName)
	}
	// Check for path traversal attempts.
	if strings.Contains(coord, "..") {
		return fmt.Errorf("%s cannot contain '..'", fieldName)
	}
	return nil
}

// validatePath validates a file path to prevent path traversal.
func validatePath(path string) error {
	if path == "" {
		return nil
	}

	// Clean the path.
	cleaned := filepath.Clean(path)

	// Check for absolute paths (potential escape from working directory).
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("absolute paths are not allowed")
	}

	// Check for path traversal attempts.
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return fmt.Errorf("path traversal detected: cannot use '..' to escape working directory")
	}

	return nil
}

// validateProfile validates a Maven profile name.
func validateProfile(profile string) error {
	if profile == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if len(profile) > 128 {
		return fmt.Errorf("profile name too long (max 128 characters)")
	}
	if !profilePattern.MatchString(profile) {
		return fmt.Errorf("invalid profile name: must be alphanumeric with dashes or underscores")
	}
	return nil
}

// validateRepositoryURL validates a Maven repository URL with SSRF protection.
func validateRepositoryURL(rawURL string) error {
	if rawURL == "" {
		return nil // Optional field.
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Hostname()

	// Allow localhost for testing purposes (HTTP is allowed only for localhost/127.0.0.1).
	isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"

	// Require HTTPS for non-localhost URLs.
	if parsedURL.Scheme != "https" && !isLocalhost {
		return fmt.Errorf("only HTTPS URLs are allowed (got %s)", parsedURL.Scheme)
	}

	// For localhost, allow HTTP but skip the private IP check (it is intentionally local).
	if isLocalhost {
		return nil
	}

	// Resolve hostname to check for private IPs.
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URLs pointing to private networks are not allowed")
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private/reserved range.
func isPrivateIP(ip net.IP) bool {
	// Private IPv4 ranges.
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // Link-local.
		"0.0.0.0/8",
	}

	// Cloud metadata endpoints.
	cloudMetadata := []string{
		"169.254.169.254/32", // AWS/GCP/Azure metadata.
		"fd00:ec2::254/128",  // AWS IMDSv2 IPv6.
	}

	allRanges := append(privateRanges, cloudMetadata...)

	for _, cidr := range allRanges {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if block.Contains(ip) {
			return true
		}
	}

	// Check for IPv6 private ranges.
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}

	return false
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
				"skip_tests": {"type": "boolean", "description": "Skip tests during deploy", "default": false},
				"settings": {"type": "string", "description": "Path to settings.xml (optional)"},
				"profiles": {"type": "array", "items": {"type": "string"}, "description": "Maven profiles to activate (optional)"}
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

// buildMavenCommand constructs the Maven deploy command arguments.
func (p *MavenPlugin) buildMavenCommand(cfg *Config) ([]string, error) {
	args := []string{"deploy"}

	// Add pom file path.
	pomPath := cfg.PomPath
	if pomPath == "" {
		pomPath = "pom.xml"
	}
	if err := validatePath(pomPath); err != nil {
		return nil, fmt.Errorf("invalid pom_path: %w", err)
	}
	args = append(args, "-f", pomPath)

	// Add skip tests flag.
	if cfg.SkipTests {
		args = append(args, "-DskipTests")
	}

	// Add settings file if specified.
	if cfg.Settings != "" {
		if err := validatePath(cfg.Settings); err != nil {
			return nil, fmt.Errorf("invalid settings path: %w", err)
		}
		args = append(args, "-s", cfg.Settings)
	}

	// Add profiles if specified.
	if len(cfg.Profiles) > 0 {
		for _, profile := range cfg.Profiles {
			if err := validateProfile(profile); err != nil {
				return nil, fmt.Errorf("invalid profile '%s': %w", profile, err)
			}
		}
		args = append(args, "-P", strings.Join(cfg.Profiles, ","))
	}

	return args, nil
}

func (p *MavenPlugin) deploy(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Validate coordinates.
	if err := validateMavenCoordinate(cfg.GroupID, "group_id"); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if err := validateMavenCoordinate(cfg.ArtifactID, "artifact_id"); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Validate repository URL if provided.
	if err := validateRepositoryURL(cfg.Repository); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid repository URL: %v", err),
		}, nil
	}

	// Build the command arguments.
	args, err := p.buildMavenCommand(cfg)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would deploy Maven artifact",
			Outputs: map[string]any{
				"group_id":    cfg.GroupID,
				"artifact_id": cfg.ArtifactID,
				"version":     releaseCtx.Version,
				"pom_path":    cfg.PomPath,
				"command":     "mvn " + strings.Join(args, " "),
				"skip_tests":  cfg.SkipTests,
				"profiles":    cfg.Profiles,
			},
		}, nil
	}

	// Execute the Maven deploy command.
	executor := p.getExecutor()
	output, err := executor.Run(ctx, "mvn", args...)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("Maven deploy failed: %v\nOutput: %s", err, string(output)),
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

	pomPath := parser.GetString("pom_path", "", "")
	if pomPath == "" {
		pomPath = "pom.xml"
	}

	return &Config{
		GroupID:    parser.GetString("group_id", "", ""),
		ArtifactID: parser.GetString("artifact_id", "", ""),
		PomPath:    pomPath,
		Username:   parser.GetString("username", "MAVEN_USERNAME", ""),
		Password:   parser.GetString("password", "MAVEN_PASSWORD", ""),
		Repository: parser.GetString("repository", "", ""),
		SkipTests:  parser.GetBool("skip_tests", false),
		Settings:   parser.GetString("settings", "", ""),
		Profiles:   parser.GetStringSlice("profiles", nil),
	}
}

// Validate validates the plugin configuration.
func (p *MavenPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := helpers.NewValidationBuilder()
	parser := helpers.NewConfigParser(config)

	// Validate group_id.
	groupID := parser.GetString("group_id", "", "")
	if groupID == "" {
		vb.AddError("group_id", "Maven group ID is required")
	} else if err := validateMavenCoordinate(groupID, "group_id"); err != nil {
		vb.AddError("group_id", err.Error())
	}

	// Validate artifact_id.
	artifactID := parser.GetString("artifact_id", "", "")
	if artifactID == "" {
		vb.AddError("artifact_id", "Maven artifact ID is required")
	} else if err := validateMavenCoordinate(artifactID, "artifact_id"); err != nil {
		vb.AddError("artifact_id", err.Error())
	}

	// Validate pom_path if provided.
	pomPath := parser.GetString("pom_path", "", "pom.xml")
	if err := validatePath(pomPath); err != nil {
		vb.AddError("pom_path", err.Error())
	}

	// Validate repository URL if provided.
	repository := parser.GetString("repository", "", "")
	if repository != "" {
		if err := validateRepositoryURL(repository); err != nil {
			vb.AddError("repository", err.Error())
		}
	}

	// Validate settings path if provided.
	settings := parser.GetString("settings", "", "")
	if settings != "" {
		if err := validatePath(settings); err != nil {
			vb.AddError("settings", err.Error())
		}
	}

	// Validate profiles if provided.
	profiles := parser.GetStringSlice("profiles", nil)
	for _, profile := range profiles {
		if err := validateProfile(profile); err != nil {
			vb.AddError("profiles", fmt.Sprintf("invalid profile '%s': %s", profile, err.Error()))
		}
	}

	return vb.Build(), nil
}
