// Package main provides tests for the Maven plugin.
package main

import (
	"context"
	"os"
	"testing"

	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &MavenPlugin{}
	info := p.GetInfo()

	if info.Name != "maven" {
		t.Errorf("expected name 'maven', got '%s'", info.Name)
	}

	if info.Version == "" {
		t.Error("expected non-empty version")
	}

	if info.Description == "" {
		t.Error("expected non-empty description")
	}

	if info.Author == "" {
		t.Error("expected non-empty author")
	}

	// Check hooks
	if len(info.Hooks) == 0 {
		t.Error("expected at least one hook")
	}

	hasPostPublish := false
	for _, hook := range info.Hooks {
		if hook == plugin.HookPostPublish {
			hasPostPublish = true
			break
		}
	}
	if !hasPostPublish {
		t.Error("expected PostPublish hook")
	}

	// Check config schema is valid JSON
	if info.ConfigSchema == "" {
		t.Error("expected non-empty config schema")
	}
}

func TestValidate(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    map[string]any
		wantValid bool
		wantErrs  []string
	}{
		{
			name:      "missing both group_id and artifact_id",
			config:    map[string]any{},
			wantValid: false,
			wantErrs:  []string{"group_id", "artifact_id"},
		},
		{
			name: "missing group_id",
			config: map[string]any{
				"artifact_id": "my-artifact",
			},
			wantValid: false,
			wantErrs:  []string{"group_id"},
		},
		{
			name: "missing artifact_id",
			config: map[string]any{
				"group_id": "com.example",
			},
			wantValid: false,
			wantErrs:  []string{"artifact_id"},
		},
		{
			name: "valid config with required fields",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
			},
			wantValid: true,
			wantErrs:  nil,
		},
		{
			name: "valid config with all options",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"pom_path":    "custom/pom.xml",
				"username":    "user",
				"password":    "pass",
				"repository":  "https://repo.example.com",
				"skip_tests":  true,
			},
			wantValid: true,
			wantErrs:  nil,
		},
		{
			name: "empty string group_id",
			config: map[string]any{
				"group_id":    "",
				"artifact_id": "my-artifact",
			},
			wantValid: false,
			wantErrs:  []string{"group_id"},
		},
		{
			name: "empty string artifact_id",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "",
			},
			wantValid: false,
			wantErrs:  []string{"artifact_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Valid != tt.wantValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.wantValid, resp.Valid, resp.Errors)
			}

			if tt.wantErrs != nil {
				for _, expectedField := range tt.wantErrs {
					found := false
					for _, e := range resp.Errors {
						if e.Field == expectedField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error for field '%s', but not found in errors: %v", expectedField, resp.Errors)
					}
				}
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	p := &MavenPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		envVars  map[string]string
		expected Config
	}{
		{
			name:   "defaults",
			config: map[string]any{},
			expected: Config{
				GroupID:    "",
				ArtifactID: "",
				PomPath:    "pom.xml",
				Username:   "",
				Password:   "",
				Repository: "",
				SkipTests:  false,
			},
		},
		{
			name: "custom values",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"pom_path":    "custom/pom.xml",
				"username":    "testuser",
				"password":    "testpass",
				"repository":  "https://repo.example.com",
				"skip_tests":  true,
			},
			expected: Config{
				GroupID:    "com.example",
				ArtifactID: "my-artifact",
				PomPath:    "custom/pom.xml",
				Username:   "testuser",
				Password:   "testpass",
				Repository: "https://repo.example.com",
				SkipTests:  true,
			},
		},
		{
			name:   "env var fallback for username and password",
			config: map[string]any{},
			envVars: map[string]string{
				"MAVEN_USERNAME": "envuser",
				"MAVEN_PASSWORD": "envpass",
			},
			expected: Config{
				GroupID:    "",
				ArtifactID: "",
				PomPath:    "pom.xml",
				Username:   "envuser",
				Password:   "envpass",
				Repository: "",
				SkipTests:  false,
			},
		},
		{
			name: "config overrides env vars",
			config: map[string]any{
				"username": "configuser",
				"password": "configpass",
			},
			envVars: map[string]string{
				"MAVEN_USERNAME": "envuser",
				"MAVEN_PASSWORD": "envpass",
			},
			expected: Config{
				GroupID:    "",
				ArtifactID: "",
				PomPath:    "pom.xml",
				Username:   "configuser",
				Password:   "configpass",
				Repository: "",
				SkipTests:  false,
			},
		},
		{
			name: "pom_path default when not set",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
			},
			expected: Config{
				GroupID:    "com.example",
				ArtifactID: "my-artifact",
				PomPath:    "pom.xml",
				Username:   "",
				Password:   "",
				Repository: "",
				SkipTests:  false,
			},
		},
		{
			name: "custom pom_path",
			config: map[string]any{
				"pom_path": "submodule/pom.xml",
			},
			expected: Config{
				GroupID:    "",
				ArtifactID: "",
				PomPath:    "submodule/pom.xml",
				Username:   "",
				Password:   "",
				Repository: "",
				SkipTests:  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set env vars
			os.Unsetenv("MAVEN_USERNAME")
			os.Unsetenv("MAVEN_PASSWORD")
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			cfg := p.parseConfig(tt.config)

			if cfg.GroupID != tt.expected.GroupID {
				t.Errorf("group_id: expected '%s', got '%s'", tt.expected.GroupID, cfg.GroupID)
			}
			if cfg.ArtifactID != tt.expected.ArtifactID {
				t.Errorf("artifact_id: expected '%s', got '%s'", tt.expected.ArtifactID, cfg.ArtifactID)
			}
			if cfg.PomPath != tt.expected.PomPath {
				t.Errorf("pom_path: expected '%s', got '%s'", tt.expected.PomPath, cfg.PomPath)
			}
			if cfg.Username != tt.expected.Username {
				t.Errorf("username: expected '%s', got '%s'", tt.expected.Username, cfg.Username)
			}
			if cfg.Password != tt.expected.Password {
				t.Errorf("password: expected '%s', got '%s'", tt.expected.Password, cfg.Password)
			}
			if cfg.Repository != tt.expected.Repository {
				t.Errorf("repository: expected '%s', got '%s'", tt.expected.Repository, cfg.Repository)
			}
			if cfg.SkipTests != tt.expected.SkipTests {
				t.Errorf("skip_tests: expected %v, got %v", tt.expected.SkipTests, cfg.SkipTests)
			}
		})
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	tests := []struct {
		name               string
		config             map[string]any
		releaseCtx         plugin.ReleaseContext
		expectedGroupID    string
		expectedArtifactID string
		expectedVersion    string
		expectedPomPath    string
	}{
		{
			name: "basic execution",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.2.3",
			},
			expectedGroupID:    "com.example",
			expectedArtifactID: "my-app",
			expectedVersion:    "v1.2.3",
			expectedPomPath:    "pom.xml",
		},
		{
			name: "with custom pom path",
			config: map[string]any{
				"group_id":    "org.myorg",
				"artifact_id": "my-lib",
				"pom_path":    "module/pom.xml",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v2.0.0",
			},
			expectedGroupID:    "org.myorg",
			expectedArtifactID: "my-lib",
			expectedVersion:    "v2.0.0",
			expectedPomPath:    "module/pom.xml",
		},
		{
			name: "with all config options",
			config: map[string]any{
				"group_id":    "io.github.example",
				"artifact_id": "sample-artifact",
				"pom_path":    "custom/pom.xml",
				"repository":  "https://oss.sonatype.org",
				"skip_tests":  true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v0.1.0",
			},
			expectedGroupID:    "io.github.example",
			expectedArtifactID: "sample-artifact",
			expectedVersion:    "v0.1.0",
			expectedPomPath:    "custom/pom.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook:    plugin.HookPostPublish,
				Config:  tt.config,
				Context: tt.releaseCtx,
				DryRun:  true,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !resp.Success {
				t.Errorf("expected success, got error: %s", resp.Error)
			}

			if resp.Message != "Would deploy Maven artifact" {
				t.Errorf("expected dry run message, got: %s", resp.Message)
			}

			// Check outputs
			if resp.Outputs == nil {
				t.Fatal("expected outputs to be set")
			}

			if groupID, ok := resp.Outputs["group_id"].(string); !ok || groupID != tt.expectedGroupID {
				t.Errorf("group_id: expected '%s', got '%v'", tt.expectedGroupID, resp.Outputs["group_id"])
			}

			if artifactID, ok := resp.Outputs["artifact_id"].(string); !ok || artifactID != tt.expectedArtifactID {
				t.Errorf("artifact_id: expected '%s', got '%v'", tt.expectedArtifactID, resp.Outputs["artifact_id"])
			}

			if version, ok := resp.Outputs["version"].(string); !ok || version != tt.expectedVersion {
				t.Errorf("version: expected '%s', got '%v'", tt.expectedVersion, resp.Outputs["version"])
			}

			if pomPath, ok := resp.Outputs["pom_path"].(string); !ok || pomPath != tt.expectedPomPath {
				t.Errorf("pom_path: expected '%s', got '%v'", tt.expectedPomPath, resp.Outputs["pom_path"])
			}
		})
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	tests := []struct {
		name string
		hook plugin.Hook
	}{
		{
			name: "PreInit hook",
			hook: plugin.HookPreInit,
		},
		{
			name: "PostInit hook",
			hook: plugin.HookPostInit,
		},
		{
			name: "PreVersion hook",
			hook: plugin.HookPreVersion,
		},
		{
			name: "PostVersion hook",
			hook: plugin.HookPostVersion,
		},
		{
			name: "PreNotes hook",
			hook: plugin.HookPreNotes,
		},
		{
			name: "PostNotes hook",
			hook: plugin.HookPostNotes,
		},
		{
			name: "PrePublish hook",
			hook: plugin.HookPrePublish,
		},
		{
			name: "OnSuccess hook",
			hook: plugin.HookOnSuccess,
		},
		{
			name: "OnError hook",
			hook: plugin.HookOnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: tt.hook,
				Config: map[string]any{
					"group_id":    "com.example",
					"artifact_id": "test",
				},
				DryRun: true,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !resp.Success {
				t.Errorf("expected success for unhandled hook %s", tt.hook)
			}

			expectedMsg := "Hook " + string(tt.hook) + " not handled"
			if resp.Message != expectedMsg {
				t.Errorf("expected message '%s', got '%s'", expectedMsg, resp.Message)
			}
		})
	}
}

func TestExecuteNonDryRun(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"group_id":    "com.example",
			"artifact_id": "my-app",
		},
		Context: plugin.ReleaseContext{
			Version: "v1.0.0",
		},
		DryRun: false,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	expectedMsg := "Deployed Maven artifact com.example:my-app:v1.0.0"
	if resp.Message != expectedMsg {
		t.Errorf("expected message '%s', got '%s'", expectedMsg, resp.Message)
	}

	// Check outputs
	if resp.Outputs == nil {
		t.Fatal("expected outputs to be set")
	}

	if groupID, ok := resp.Outputs["group_id"].(string); !ok || groupID != "com.example" {
		t.Errorf("expected group_id 'com.example', got '%v'", resp.Outputs["group_id"])
	}

	if artifactID, ok := resp.Outputs["artifact_id"].(string); !ok || artifactID != "my-app" {
		t.Errorf("expected artifact_id 'my-app', got '%v'", resp.Outputs["artifact_id"])
	}

	if version, ok := resp.Outputs["version"].(string); !ok || version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%v'", resp.Outputs["version"])
	}
}
