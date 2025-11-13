package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// CLIExecutor handles execution of cloud CLI commands
type CLIExecutor struct {
	// Add any shared configuration if needed
}

// NewCLIExecutor creates a new CLI executor
func NewCLIExecutor() *CLIExecutor {
	return &CLIExecutor{}
}

// ExecuteAzureCLI executes Azure CLI commands using 'az rest'
func (e *CLIExecutor) ExecuteAzureCLI(ctx context.Context, method, url string, body interface{}) ([]byte, error) {
	args := []string{"rest", "--method", method, "--url", url}

	// Add body if provided and not a GET request
	if body != nil && method != "GET" {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		args = append(args, "--body", string(bodyJSON))
	}

	// Log the command for debugging
	tflog.Debug(ctx, "Executing Azure CLI command",
		map[string]interface{}{
			"command": "az",
			"args":    strings.Join(args, " "),
		},
	)

	// Execute command
	cmd := exec.CommandContext(ctx, "az", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		tflog.Error(ctx, "Azure CLI command failed",
			map[string]interface{}{
				"error":  err.Error(),
				"stderr": stderr.String(),
				"stdout": stdout.String(),
			},
		)
		return nil, fmt.Errorf("az rest failed: %w\nstderr: %s\nstdout: %s",
			err, stderr.String(), stdout.String())
	}

	tflog.Debug(ctx, "Azure CLI command succeeded",
		map[string]interface{}{
			"response_length": len(stdout.Bytes()),
		},
	)

	return stdout.Bytes(), nil
}

// ExecuteOCICLI executes OCI CLI commands using 'oci raw-request'
func (e *CLIExecutor) ExecuteOCICLI(ctx context.Context, method, targetURI string, body interface{}) ([]byte, error) {
	args := []string{"raw-request", "--http-method", method, "--target-uri", targetURI}

	// Add request body if provided and not a GET request
	if body != nil && method != "GET" && method != "DELETE" {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		args = append(args, "--request-body", string(bodyJSON))
	}

	// Log the command for debugging
	tflog.Debug(ctx, "Executing OCI CLI command",
		map[string]interface{}{
			"command": "oci",
			"args":    strings.Join(args, " "),
		},
	)

	// Execute command
	cmd := exec.CommandContext(ctx, "oci", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		tflog.Error(ctx, "OCI CLI command failed",
			map[string]interface{}{
				"error":  err.Error(),
				"stderr": stderr.String(),
				"stdout": stdout.String(),
			},
		)
		return nil, fmt.Errorf("oci raw-request failed: %w\nstderr: %s\nstdout: %s",
			err, stderr.String(), stdout.String())
	}

	tflog.Debug(ctx, "OCI CLI command succeeded",
		map[string]interface{}{
			"response_length": len(stdout.Bytes()),
		},
	)

	return stdout.Bytes(), nil
}

// CheckCLIAvailability checks if required CLIs are installed
func (e *CLIExecutor) CheckCLIAvailability(ctx context.Context) map[string]bool {
	availability := make(map[string]bool)

	// Check Azure CLI
	azCmd := exec.CommandContext(ctx, "az", "version")
	availability["azure"] = azCmd.Run() == nil

	// Check OCI CLI
	ociCmd := exec.CommandContext(ctx, "oci", "--version")
	availability["oci"] = ociCmd.Run() == nil

	// Log availability
	tflog.Debug(ctx, "CLI availability check",
		map[string]interface{}{
			"azure_cli": availability["azure"],
			"oci_cli":   availability["oci"],
		},
	)

	return availability
}

// GetCLIVersions returns version information for installed CLIs
func (e *CLIExecutor) GetCLIVersions(ctx context.Context) map[string]string {
	versions := make(map[string]string)

	// Get Azure CLI version
	azCmd := exec.CommandContext(ctx, "az", "version", "--output", "json")
	if output, err := azCmd.Output(); err == nil {
		var azVersion map[string]interface{}
		if json.Unmarshal(output, &azVersion) == nil {
			if cliVersion, ok := azVersion["azure-cli"].(string); ok {
				versions["azure"] = cliVersion
			}
		}
	}

	// Get OCI CLI version
	ociCmd := exec.CommandContext(ctx, "oci", "--version")
	if output, err := ociCmd.Output(); err == nil {
		versions["oci"] = strings.TrimSpace(string(output))
	}

	return versions
}
