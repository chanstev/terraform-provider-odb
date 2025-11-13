package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure OMCContextDataSource satisfies various datasource interfaces.
var _ datasource.DataSource = &OMCContextDataSource{}

// OMCContextDataSource defines the data source implementation.
type OMCContextDataSource struct {
	restClient *RESTClient
}

// OMCContextDataSourceModel describes the data source data model.
type OMCContextDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	OCI    types.String `tfsdk:"oci"`
	Azure  types.String `tfsdk:"azure"`
	Google types.String `tfsdk:"google"`
	AWS    types.String `tfsdk:"aws"`
}

func (d *OMCContextDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_context"
}

func (d *OMCContextDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves cloud context information for all configured providers.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier for the data source.",
			},
			"oci": schema.StringAttribute{
				Computed:    true,
				Description: "OCI cloud context information.",
			},
			"azure": schema.StringAttribute{
				Computed:    true,
				Description: "Azure cloud context information.",
			},
			"google": schema.StringAttribute{
				Computed:    true,
				Description: "Google Cloud context information.",
			},
			"aws": schema.StringAttribute{
				Computed:    true,
				Description: "AWS cloud context information.",
			},
		},
	}
}

func (d *OMCContextDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	restClient, ok := req.ProviderData.(*RESTClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *RESTClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.restClient = restClient
}

func (d *OMCContextDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OMCContextDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set ID
	data.ID = types.StringValue("omc-context")

	// Retrieve cloud context information from all configured providers
	contexts := d.restClient.GetCloudContext(ctx)

	// Populate data model with context information
	if ociCtx, ok := contexts["oci"]; ok {
		if ociCtx.Verified {
			data.OCI = types.StringValue(fmt.Sprintf("Region: %s, Scope: %s (Verified)",
				ociCtx.Region, ociCtx.Scope))
		} else if ociCtx.Error != "" {
			data.OCI = types.StringValue(fmt.Sprintf("Region: %s, Error: %s",
				ociCtx.Region, ociCtx.Error))
			resp.Diagnostics.AddWarning(
				"OCI Authentication Warning",
				fmt.Sprintf("Failed to verify OCI credentials: %s", ociCtx.Error),
			)
		}
	}

	if azureCtx, ok := contexts["azure"]; ok {
		if azureCtx.Verified {
			data.Azure = types.StringValue(fmt.Sprintf("Region: %s, Scope: %s (Verified)",
				azureCtx.Region, azureCtx.Scope))
		} else if azureCtx.Error != "" {
			data.Azure = types.StringValue(fmt.Sprintf("Region: %s, Error: %s",
				azureCtx.Region, azureCtx.Error))
			resp.Diagnostics.AddWarning(
				"Azure Authentication Warning",
				fmt.Sprintf("Failed to verify Azure credentials: %s", azureCtx.Error),
			)
		}
	}

	if googleCtx, ok := contexts["google"]; ok {
		if googleCtx.Verified {
			data.Google = types.StringValue(fmt.Sprintf("Region: %s, Scope: %s (Verified)",
				googleCtx.Region, googleCtx.Scope))
		} else if googleCtx.Error != "" {
			data.Google = types.StringValue(fmt.Sprintf("Region: %s, Error: %s",
				googleCtx.Region, googleCtx.Error))
			resp.Diagnostics.AddWarning(
				"Google Cloud Authentication Warning",
				fmt.Sprintf("Failed to verify Google Cloud credentials: %s", googleCtx.Error),
			)
		}
	}

	if awsCtx, ok := contexts["aws"]; ok {
		if awsCtx.Verified {
			data.AWS = types.StringValue(fmt.Sprintf("Region: %s, Scope: %s (Verified)",
				awsCtx.Region, awsCtx.Scope))
		} else if awsCtx.Error != "" {
			data.AWS = types.StringValue(fmt.Sprintf("Region: %s, Error: %s",
				awsCtx.Region, awsCtx.Error))
			resp.Diagnostics.AddWarning(
				"AWS Authentication Warning",
				fmt.Sprintf("Failed to verify AWS credentials: %s", awsCtx.Error),
			)
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewOMCContextDataSource() datasource.DataSource {
	return &OMCContextDataSource{}
}
