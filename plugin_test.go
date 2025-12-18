// Package main provides tests for the Maven plugin.
package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// MockCommandExecutor is a mock implementation of CommandExecutor for testing.
type MockCommandExecutor struct {
	RunFunc func(ctx context.Context, name string, args ...string) ([]byte, error)
	Calls   []MockCall
}

// MockCall records a call to the executor.
type MockCall struct {
	Name string
	Args []string
}

// Run implements CommandExecutor.
func (m *MockCommandExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, MockCall{Name: name, Args: args})
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return []byte("success"), nil
}

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

	// Check hooks.
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

	// Check config schema is valid JSON.
	if info.ConfigSchema == "" {
		t.Error("expected non-empty config schema")
	}

	// Check that new fields are in schema.
	if !strings.Contains(info.ConfigSchema, "settings") {
		t.Error("expected settings field in config schema")
	}
	if !strings.Contains(info.ConfigSchema, "profiles") {
		t.Error("expected profiles field in config schema")
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
				"repository":  "http://localhost:8081/repository/maven-releases",
				"skip_tests":  true,
				"settings":    "custom-settings.xml",
				"profiles":    []any{"release", "gpg-sign"},
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
		{
			name: "invalid pom_path with path traversal",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"pom_path":    "../../../etc/passwd",
			},
			wantValid: false,
			wantErrs:  []string{"pom_path"},
		},
		{
			name: "invalid absolute pom_path",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"pom_path":    "/etc/pom.xml",
			},
			wantValid: false,
			wantErrs:  []string{"pom_path"},
		},
		{
			name: "invalid repository URL - HTTP not allowed",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"repository":  "http://insecure.example.com",
			},
			wantValid: false,
			wantErrs:  []string{"repository"},
		},
		{
			name: "valid localhost repository URL with HTTP",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"repository":  "http://localhost:8081/repository/maven-releases",
			},
			wantValid: true,
			wantErrs:  nil,
		},
		{
			name: "invalid settings path with path traversal",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"settings":    "../../../etc/settings.xml",
			},
			wantValid: false,
			wantErrs:  []string{"settings"},
		},
		{
			name: "invalid profile name",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"profiles":    []any{"valid-profile", "invalid profile with spaces"},
			},
			wantValid: false,
			wantErrs:  []string{"profiles"},
		},
		{
			name: "valid profile names",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"profiles":    []any{"release", "gpg-sign", "ossrh"},
			},
			wantValid: true,
			wantErrs:  nil,
		},
		{
			name: "group_id with invalid characters",
			config: map[string]any{
				"group_id":    "com.example; rm -rf /",
				"artifact_id": "my-artifact",
			},
			wantValid: false,
			wantErrs:  []string{"group_id"},
		},
		{
			name: "artifact_id with invalid characters",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact$(whoami)",
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
				Settings:   "",
				Profiles:   nil,
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
				"settings":    "custom-settings.xml",
				"profiles":    []any{"release", "gpg-sign"},
			},
			expected: Config{
				GroupID:    "com.example",
				ArtifactID: "my-artifact",
				PomPath:    "custom/pom.xml",
				Username:   "testuser",
				Password:   "testpass",
				Repository: "https://repo.example.com",
				SkipTests:  true,
				Settings:   "custom-settings.xml",
				Profiles:   []string{"release", "gpg-sign"},
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
				Settings:   "",
				Profiles:   nil,
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
				Settings:   "",
				Profiles:   nil,
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
				Settings:   "",
				Profiles:   nil,
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
				Settings:   "",
				Profiles:   nil,
			},
		},
		{
			name: "with settings and profiles",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-artifact",
				"settings":    ".mvn/settings.xml",
				"profiles":    []any{"ossrh", "sign"},
			},
			expected: Config{
				GroupID:    "com.example",
				ArtifactID: "my-artifact",
				PomPath:    "pom.xml",
				Username:   "",
				Password:   "",
				Repository: "",
				SkipTests:  false,
				Settings:   ".mvn/settings.xml",
				Profiles:   []string{"ossrh", "sign"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set env vars.
			_ = os.Unsetenv("MAVEN_USERNAME")
			_ = os.Unsetenv("MAVEN_PASSWORD")
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
				defer func(key string) { _ = os.Unsetenv(key) }(k)
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
			if cfg.Settings != tt.expected.Settings {
				t.Errorf("settings: expected '%s', got '%s'", tt.expected.Settings, cfg.Settings)
			}
			if len(cfg.Profiles) != len(tt.expected.Profiles) {
				t.Errorf("profiles: expected %v, got %v", tt.expected.Profiles, cfg.Profiles)
			} else {
				for i, profile := range cfg.Profiles {
					if profile != tt.expected.Profiles[i] {
						t.Errorf("profiles[%d]: expected '%s', got '%s'", i, tt.expected.Profiles[i], profile)
					}
				}
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
		expectedCommand    string
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
			expectedCommand:    "mvn deploy -f pom.xml",
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
			expectedCommand:    "mvn deploy -f module/pom.xml",
		},
		{
			name: "with skip tests",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"skip_tests":  true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			expectedGroupID:    "com.example",
			expectedArtifactID: "my-app",
			expectedVersion:    "v1.0.0",
			expectedPomPath:    "pom.xml",
			expectedCommand:    "mvn deploy -f pom.xml -DskipTests",
		},
		{
			name: "with settings file",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"settings":    "custom-settings.xml",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			expectedGroupID:    "com.example",
			expectedArtifactID: "my-app",
			expectedVersion:    "v1.0.0",
			expectedPomPath:    "pom.xml",
			expectedCommand:    "mvn deploy -f pom.xml -s custom-settings.xml",
		},
		{
			name: "with profiles",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"profiles":    []any{"release", "gpg-sign"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			expectedGroupID:    "com.example",
			expectedArtifactID: "my-app",
			expectedVersion:    "v1.0.0",
			expectedPomPath:    "pom.xml",
			expectedCommand:    "mvn deploy -f pom.xml -P release,gpg-sign",
		},
		{
			name: "with all options",
			config: map[string]any{
				"group_id":    "io.github.example",
				"artifact_id": "sample-artifact",
				"pom_path":    "custom/pom.xml",
				"repository":  "https://oss.sonatype.org",
				"skip_tests":  true,
				"settings":    ".mvn/settings.xml",
				"profiles":    []any{"ossrh", "sign"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v0.1.0",
			},
			expectedGroupID:    "io.github.example",
			expectedArtifactID: "sample-artifact",
			expectedVersion:    "v0.1.0",
			expectedPomPath:    "custom/pom.xml",
			expectedCommand:    "mvn deploy -f custom/pom.xml -DskipTests -s .mvn/settings.xml -P ossrh,sign",
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

			// Check outputs.
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

			if command, ok := resp.Outputs["command"].(string); !ok || command != tt.expectedCommand {
				t.Errorf("command: expected '%s', got '%v'", tt.expectedCommand, resp.Outputs["command"])
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

func TestExecuteWithMockExecutor(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		config         map[string]any
		releaseCtx     plugin.ReleaseContext
		executorFunc   func(ctx context.Context, name string, args ...string) ([]byte, error)
		expectedArgs   []string
		wantSuccess    bool
		wantErrContain string
	}{
		{
			name: "successful deploy",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			executorFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("[INFO] BUILD SUCCESS"), nil
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml"},
			wantSuccess:  true,
		},
		{
			name: "deploy with skip tests",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"skip_tests":  true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			executorFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("[INFO] BUILD SUCCESS"), nil
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml", "-DskipTests"},
			wantSuccess:  true,
		},
		{
			name: "deploy with settings",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"settings":    "custom-settings.xml",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			executorFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("[INFO] BUILD SUCCESS"), nil
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml", "-s", "custom-settings.xml"},
			wantSuccess:  true,
		},
		{
			name: "deploy with profiles",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"profiles":    []any{"release", "sign"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			executorFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("[INFO] BUILD SUCCESS"), nil
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml", "-P", "release,sign"},
			wantSuccess:  true,
		},
		{
			name: "deploy failure",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v1.0.0",
			},
			executorFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("[ERROR] BUILD FAILURE\n[ERROR] Failed to execute goal"), errors.New("exit status 1")
			},
			wantSuccess:    false,
			wantErrContain: "Maven deploy failed",
		},
		{
			name: "full command with all options",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"pom_path":    "submodule/pom.xml",
				"skip_tests":  true,
				"settings":    ".mvn/settings.xml",
				"profiles":    []any{"ossrh", "gpg"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "v2.0.0",
			},
			executorFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("[INFO] BUILD SUCCESS"), nil
			},
			expectedArgs: []string{"deploy", "-f", "submodule/pom.xml", "-DskipTests", "-s", ".mvn/settings.xml", "-P", "ossrh,gpg"},
			wantSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockCommandExecutor{
				RunFunc: tt.executorFunc,
			}

			p := &MavenPlugin{executor: mockExec}

			req := plugin.ExecuteRequest{
				Hook:    plugin.HookPostPublish,
				Config:  tt.config,
				Context: tt.releaseCtx,
				DryRun:  false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success != tt.wantSuccess {
				t.Errorf("expected success=%v, got success=%v, error=%s", tt.wantSuccess, resp.Success, resp.Error)
			}

			if tt.wantErrContain != "" && !strings.Contains(resp.Error, tt.wantErrContain) {
				t.Errorf("expected error to contain '%s', got '%s'", tt.wantErrContain, resp.Error)
			}

			if tt.expectedArgs != nil && len(mockExec.Calls) > 0 {
				call := mockExec.Calls[0]
				if call.Name != "mvn" {
					t.Errorf("expected command 'mvn', got '%s'", call.Name)
				}

				if len(call.Args) != len(tt.expectedArgs) {
					t.Errorf("expected args %v, got %v", tt.expectedArgs, call.Args)
				} else {
					for i, arg := range call.Args {
						if arg != tt.expectedArgs[i] {
							t.Errorf("arg[%d]: expected '%s', got '%s'", i, tt.expectedArgs[i], arg)
						}
					}
				}
			}
		})
	}
}

func TestExecuteValidationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		config         map[string]any
		wantErrContain string
	}{
		{
			name: "empty group_id",
			config: map[string]any{
				"group_id":    "",
				"artifact_id": "my-app",
			},
			wantErrContain: "group_id cannot be empty",
		},
		{
			name: "empty artifact_id",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "",
			},
			wantErrContain: "artifact_id cannot be empty",
		},
		{
			name: "invalid repository URL - HTTP",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"repository":  "http://insecure.example.com",
			},
			wantErrContain: "only HTTPS URLs are allowed",
		},
		{
			name: "path traversal in pom_path",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"pom_path":    "../../../etc/passwd",
			},
			wantErrContain: "path traversal detected",
		},
		{
			name: "absolute path in pom_path",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"pom_path":    "/etc/pom.xml",
			},
			wantErrContain: "absolute paths are not allowed",
		},
		{
			name: "path traversal in settings",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"settings":    "../../../etc/settings.xml",
			},
			wantErrContain: "path traversal detected",
		},
		{
			name: "invalid profile name",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app",
				"profiles":    []any{"valid", "invalid profile"},
			},
			wantErrContain: "invalid profile",
		},
		{
			name: "group_id with shell injection",
			config: map[string]any{
				"group_id":    "com.example; rm -rf /",
				"artifact_id": "my-app",
			},
			wantErrContain: "invalid group_id",
		},
		{
			name: "artifact_id with command substitution",
			config: map[string]any{
				"group_id":    "com.example",
				"artifact_id": "my-app$(whoami)",
			},
			wantErrContain: "invalid artifact_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &MavenPlugin{}

			req := plugin.ExecuteRequest{
				Hook:    plugin.HookPostPublish,
				Config:  tt.config,
				Context: plugin.ReleaseContext{Version: "v1.0.0"},
				DryRun:  false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Success {
				t.Errorf("expected failure, got success")
			}

			if !strings.Contains(resp.Error, tt.wantErrContain) {
				t.Errorf("expected error to contain '%s', got '%s'", tt.wantErrContain, resp.Error)
			}
		})
	}
}

func TestValidateMavenCoordinate(t *testing.T) {
	tests := []struct {
		name      string
		coord     string
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid simple coordinate",
			coord:     "com.example",
			fieldName: "group_id",
			wantErr:   false,
		},
		{
			name:      "valid coordinate with dots",
			coord:     "com.example.project",
			fieldName: "group_id",
			wantErr:   false,
		},
		{
			name:      "valid coordinate with dashes",
			coord:     "my-artifact",
			fieldName: "artifact_id",
			wantErr:   false,
		},
		{
			name:      "valid coordinate with underscores",
			coord:     "my_artifact",
			fieldName: "artifact_id",
			wantErr:   false,
		},
		{
			name:      "empty coordinate",
			coord:     "",
			fieldName: "group_id",
			wantErr:   true,
			errMsg:    "cannot be empty",
		},
		{
			name:      "coordinate too long",
			coord:     strings.Repeat("a", 257),
			fieldName: "group_id",
			wantErr:   true,
			errMsg:    "too long",
		},
		{
			name:      "coordinate with semicolon",
			coord:     "com.example;rm",
			fieldName: "group_id",
			wantErr:   true,
			errMsg:    "disallowed characters",
		},
		{
			name:      "coordinate with space",
			coord:     "com example",
			fieldName: "group_id",
			wantErr:   true,
			errMsg:    "disallowed characters",
		},
		{
			name:      "coordinate with path traversal",
			coord:     "com..example",
			fieldName: "group_id",
			wantErr:   true,
			errMsg:    "cannot contain '..'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMavenCoordinate(tt.coord, tt.fieldName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple path",
			path:    "pom.xml",
			wantErr: false,
		},
		{
			name:    "valid subdirectory path",
			path:    "module/pom.xml",
			wantErr: false,
		},
		{
			name:    "empty path is valid",
			path:    "",
			wantErr: false,
		},
		{
			name:    "path traversal with leading ..",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal",
		},
		{
			name:    "path traversal in middle",
			path:    "foo/../../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal",
		},
		{
			name:    "absolute path",
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "absolute paths are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple profile",
			profile: "release",
			wantErr: false,
		},
		{
			name:    "valid profile with dashes",
			profile: "gpg-sign",
			wantErr: false,
		},
		{
			name:    "valid profile with underscores",
			profile: "gpg_sign",
			wantErr: false,
		},
		{
			name:    "valid profile with numbers",
			profile: "jdk21",
			wantErr: false,
		},
		{
			name:    "empty profile",
			profile: "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "profile too long",
			profile: strings.Repeat("a", 129),
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name:    "profile with space",
			profile: "my profile",
			wantErr: true,
			errMsg:  "alphanumeric with dashes or underscores",
		},
		{
			name:    "profile starting with number",
			profile: "1release",
			wantErr: true,
			errMsg:  "alphanumeric with dashes or underscores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProfile(tt.profile)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRepositoryURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty URL is valid",
			url:     "",
			wantErr: false,
		},
		{
			name:    "valid HTTPS URL",
			url:     "https://oss.sonatype.org/service/local/staging/deploy/maven2",
			wantErr: false,
		},
		{
			name:    "localhost HTTP is allowed",
			url:     "http://localhost:8081/repository/maven-releases",
			wantErr: false,
		},
		{
			name:    "127.0.0.1 HTTP is allowed",
			url:     "http://127.0.0.1:8081/repository/maven-releases",
			wantErr: false,
		},
		{
			name:    "HTTP to external host not allowed",
			url:     "http://insecure.example.com/repository",
			wantErr: true,
			errMsg:  "only HTTPS URLs are allowed",
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
			errMsg:  "only HTTPS URLs are allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepositoryURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildMavenCommand(t *testing.T) {
	p := &MavenPlugin{}

	tests := []struct {
		name         string
		config       *Config
		expectedArgs []string
		wantErr      bool
		errContains  string
	}{
		{
			name: "basic command",
			config: &Config{
				PomPath: "pom.xml",
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml"},
			wantErr:      false,
		},
		{
			name: "with skip tests",
			config: &Config{
				PomPath:   "pom.xml",
				SkipTests: true,
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml", "-DskipTests"},
			wantErr:      false,
		},
		{
			name: "with settings",
			config: &Config{
				PomPath:  "pom.xml",
				Settings: "custom-settings.xml",
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml", "-s", "custom-settings.xml"},
			wantErr:      false,
		},
		{
			name: "with profiles",
			config: &Config{
				PomPath:  "pom.xml",
				Profiles: []string{"release", "sign"},
			},
			expectedArgs: []string{"deploy", "-f", "pom.xml", "-P", "release,sign"},
			wantErr:      false,
		},
		{
			name: "with all options",
			config: &Config{
				PomPath:   "submodule/pom.xml",
				SkipTests: true,
				Settings:  ".mvn/settings.xml",
				Profiles:  []string{"ossrh", "gpg"},
			},
			expectedArgs: []string{"deploy", "-f", "submodule/pom.xml", "-DskipTests", "-s", ".mvn/settings.xml", "-P", "ossrh,gpg"},
			wantErr:      false,
		},
		{
			name: "invalid pom path",
			config: &Config{
				PomPath: "../../../etc/passwd",
			},
			wantErr:     true,
			errContains: "invalid pom_path",
		},
		{
			name: "invalid settings path",
			config: &Config{
				PomPath:  "pom.xml",
				Settings: "/etc/settings.xml",
			},
			wantErr:     true,
			errContains: "invalid settings path",
		},
		{
			name: "invalid profile",
			config: &Config{
				PomPath:  "pom.xml",
				Profiles: []string{"valid", "invalid profile"},
			},
			wantErr:     true,
			errContains: "invalid profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := p.buildMavenCommand(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(args) != len(tt.expectedArgs) {
				t.Errorf("expected args %v, got %v", tt.expectedArgs, args)
				return
			}

			for i, arg := range args {
				if arg != tt.expectedArgs[i] {
					t.Errorf("arg[%d]: expected '%s', got '%s'", i, tt.expectedArgs[i], arg)
				}
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "public IP",
			ip:       "8.8.8.8",
			expected: false,
		},
		{
			name:     "localhost IPv4",
			ip:       "127.0.0.1",
			expected: true,
		},
		{
			name:     "private 10.x.x.x",
			ip:       "10.0.0.1",
			expected: true,
		},
		{
			name:     "private 172.16.x.x",
			ip:       "172.16.0.1",
			expected: true,
		},
		{
			name:     "private 192.168.x.x",
			ip:       "192.168.1.1",
			expected: true,
		},
		{
			name:     "AWS metadata",
			ip:       "169.254.169.254",
			expected: true,
		},
		{
			name:     "link local",
			ip:       "169.254.0.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s): expected %v, got %v", tt.ip, tt.expected, result)
			}
		})
	}
}

// parseIP is a helper function for testing.
func parseIP(s string) []byte {
	var ip [4]byte
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return nil
	}
	for i, p := range parts {
		var n int
		for _, c := range p {
			if c < '0' || c > '9' {
				return nil
			}
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return nil
		}
		ip[i] = byte(n)
	}
	return ip[:]
}
