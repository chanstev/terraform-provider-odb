// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure OMCProvider satisfies various provider interfaces.
var _ provider.Provider = &OMCProvider{}

// OMCProvider defines the provider implementation.
type OMCProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// AzureConfig describes Azure-specific configuration
type AzureConfig struct {
	SubscriptionID types.String `tfsdk:"subscription_id"`
	TenantID       types.String `tfsdk:"tenant_id"`
}

// OCIConfig describes OCI-specific configuration
type OCIConfig struct {
	ConfigFileProfile types.String `tfsdk:"config_file_profile"`
}

// OMCProviderModel describes the provider data model.
type OMCProviderModel struct {
	Azure *AzureConfig `tfsdk:"azure"`
	OCI   *OCIConfig   `tfsdk:"oci"`
}

func (p *OMCProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "omc"
	resp.Version = p.version
}

func (p *OMCProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Oracle Multi-Cloud (OMC) provider for managing Oracle Database resources across multiple clouds",
		Attributes: map[string]schema.Attribute{
			"azure": schema.SingleNestedAttribute{
				MarkdownDescription: "Azure authentication configuration. Uses Azure CLI by default.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{
						MarkdownDescription: "Azure subscription ID",
						Optional:            true,
					},
					"tenant_id": schema.StringAttribute{
						MarkdownDescription: "Azure tenant ID",
						Optional:            true,
					},
				},
			},
			"oci": schema.SingleNestedAttribute{
				MarkdownDescription: "OCI authentication configuration. Uses ~/.oci/config by default.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"config_file_profile": schema.StringAttribute{
						MarkdownDescription: "OCI config file profile name (default: DEFAULT)",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (p *OMCProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data OMCProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Load YAML configurations
	configLoader, err := NewConfigLoader()
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Configuration Loading Warning",
			fmt.Sprintf("Failed to load YAML configurations: %s. Will use defaults.", err.Error()),
		)
	}

	// Create provider data with CLI executor and config loader
	providerData := &ProviderData{
		CLIExecutor:  NewCLIExecutor(),
		ConfigLoader: configLoader,
		Config:       &data,
	}

	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

// ProviderData contains shared provider configuration
type ProviderData struct {
	CLIExecutor  *CLIExecutor
	ConfigLoader *ConfigLoader
	Config       *OMCProviderModel
}

func (p *OMCProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAutonomousDatabaseResource,
		NewBaseDatabaseResource,
	}
}

func (p *OMCProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		// No ephemeral resources implemented yet
	}
}

func (p *OMCProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOMCContextDataSource,
		NewAutonomousDatabaseDataSource,
		NewBaseDatabaseDataSource,
	}
}

func (p *OMCProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		// No functions implemented yet
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OMCProvider{
			version: version,
		}
	}
}
