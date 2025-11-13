package provider

import (
	"context"
	"encoding/json"
	"fmt"
)

// CLIAuthClient provides authentication via cloud CLIs
type CLIAuthClient struct {
	executor *CLIExecutor
}

// NewCLIAuthClient creates a new CLI-based auth client
func NewCLIAuthClient() *CLIAuthClient {
	return &CLIAuthClient{
		executor: NewCLIExecutor(),
	}
}

// GetOCIContext retrieves OCI context using CLI
func (c *CLIAuthClient) GetOCIContext(ctx context.Context, region, tenancyOCID string) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "oci",
		Region:   region,
		Scope:    tenancyOCID,
		Verified: false,
	}

	// Use OCI CLI to get tenancy information
	url := fmt.Sprintf("https://identity.%s.oraclecloud.com/20160918/tenancies/%s", region, tenancyOCID)

	resp, err := c.executor.ExecuteOCICLI(ctx, "GET", url, nil)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to get OCI tenancy: %v", err)
		return cloudCtx
	}

	// Parse response
	var ociResp struct {
		Data struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &ociResp); err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to parse OCI response: %v", err)
		return cloudCtx
	}

	cloudCtx.Verified = true
	cloudCtx.Scope = fmt.Sprintf("%s (Tenancy: %s)", tenancyOCID, ociResp.Data.Name)

	return cloudCtx
}

// GetAzureContext retrieves Azure context using CLI
func (c *CLIAuthClient) GetAzureContext(ctx context.Context, subscriptionID string) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "azure",
		Scope:    subscriptionID,
		Verified: false,
	}

	// Use Azure CLI to get subscription information
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s?api-version=2020-01-01", subscriptionID)

	resp, err := c.executor.ExecuteAzureCLI(ctx, "GET", url, nil)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to get Azure subscription: %v", err)
		return cloudCtx
	}

	// Parse response
	var azureResp struct {
		DisplayName      string `json:"displayName"`
		SubscriptionID   string `json:"subscriptionId"`
		State            string `json:"state"`
	}

	if err := json.Unmarshal(resp, &azureResp); err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to parse Azure response: %v", err)
		return cloudCtx
	}

	cloudCtx.Verified = true
	cloudCtx.Scope = fmt.Sprintf("%s (Subscription: %s)", subscriptionID, azureResp.DisplayName)

	return cloudCtx
}

// GetGCPContext retrieves GCP context (placeholder for now)
func (c *CLIAuthClient) GetGCPContext(ctx context.Context, projectID string) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "gcp",
		Scope:    projectID,
		Verified: false,
	}

	// GCP CLI support can be added later
	cloudCtx.Error = "GCP CLI support not yet implemented"

	return cloudCtx
}

// GetAWSContext retrieves AWS context (placeholder for now)
func (c *CLIAuthClient) GetAWSContext(ctx context.Context, accountID string) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "aws",
		Scope:    accountID,
		Verified: false,
	}

	// AWS CLI support can be added later
	cloudCtx.Error = "AWS CLI support not yet implemented"

	return cloudCtx
}
