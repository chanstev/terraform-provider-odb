package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &AutonomousDatabaseDataSource{}
	_ datasource.DataSourceWithConfigure = &AutonomousDatabaseDataSource{}
)

// NewAutonomousDatabaseDataSource is a helper function to simplify the provider implementation.
func NewAutonomousDatabaseDataSource() datasource.DataSource {
	return &AutonomousDatabaseDataSource{}
}

// AutonomousDatabaseDataSource is the data source implementation.
type AutonomousDatabaseDataSource struct {
	cliExecutor *CLIExecutor
}

// AutonomousDatabaseDataSourceModel describes the data source data model.
type AutonomousDatabaseDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	PrimaryCloud        types.String `tfsdk:"primary_cloud"`
	DisplayName         types.String `tfsdk:"display_name"`
	CPUCoreCount        types.Int64  `tfsdk:"cpu_core_count"`
	DataStorageSizeInGBs types.Int64 `tfsdk:"data_storage_size_in_gbs"`
	DbVersion           types.String `tfsdk:"db_version"`
	LifecycleState      types.String `tfsdk:"lifecycle_state"`
	Region              types.String `tfsdk:"region"`

	// Cloud-specific fields
	AutonomousDatabaseID types.String `tfsdk:"autonomous_database_id"` // OCI
	CompartmentID        types.String `tfsdk:"compartment_id"`         // OCI
}

// Metadata returns the data source type name.
func (d *AutonomousDatabaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autonomous_database"
}

// Schema defines the schema for the data source.
func (d *AutonomousDatabaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an autonomous database from OCI or Azure",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The OCID or resource ID of the database",
				Required:    true,
			},
			"primary_cloud": schema.StringAttribute{
				Description: "Primary cloud provider (oci or azure)",
				Required:    true,
			},
			"display_name": schema.StringAttribute{
				Description: "The display name of the database",
				Computed:    true,
			},
			"cpu_core_count": schema.Int64Attribute{
				Description: "Number of CPU cores",
				Computed:    true,
			},
			"data_storage_size_in_gbs": schema.Int64Attribute{
				Description: "Data storage size in GB",
				Computed:    true,
			},
			"db_version": schema.StringAttribute{
				Description: "Database version",
				Computed:    true,
			},
			"lifecycle_state": schema.StringAttribute{
				Description: "Lifecycle state of the database",
				Computed:    true,
			},
			"region": schema.StringAttribute{
				Description: "Region where the database is located",
				Computed:    true,
			},
			"autonomous_database_id": schema.StringAttribute{
				Description: "OCI-specific: Autonomous Database OCID",
				Computed:    true,
			},
			"compartment_id": schema.StringAttribute{
				Description: "OCI-specific: Compartment OCID",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *AutonomousDatabaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cliExecutor, ok := req.ProviderData.(*CLIExecutor)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *CLIExecutor, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.cliExecutor = cliExecutor
}

// Read refreshes the Terraform state with the latest data.
func (d *AutonomousDatabaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AutonomousDatabaseDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	primaryCloud := data.PrimaryCloud.ValueString()
	id := data.ID.ValueString()

	tflog.Info(ctx, "Reading autonomous database",
		map[string]interface{}{
			"primary_cloud": primaryCloud,
			"id":            id,
		},
	)

	switch primaryCloud {
	case "oci":
		if err := d.readFromOCI(ctx, &data); err != nil {
			resp.Diagnostics.AddError(
				"Failed to read from OCI",
				fmt.Sprintf("Could not read autonomous database from OCI: %s", err.Error()),
			)
			return
		}

	case "azure":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"Azure autonomous database support is not yet implemented",
		)
		return

	default:
		resp.Diagnostics.AddError(
			"Invalid Primary Cloud",
			fmt.Sprintf("Unsupported primary cloud: %s. Must be 'oci' or 'azure'", primaryCloud),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// readFromOCI reads autonomous database from OCI using CLI
func (d *AutonomousDatabaseDataSource) readFromOCI(ctx context.Context, data *AutonomousDatabaseDataSourceModel) error {
	id := data.ID.ValueString()
	region := data.Region.ValueString()

	// Default to us-ashburn-1 if no region specified
	if region == "" {
		region = "us-ashburn-1"
	}

	// Construct OCI API endpoint
	url := fmt.Sprintf("https://database.%s.oraclecloud.com/20160918/autonomousDatabases/%s", region, id)

	// Execute OCI CLI
	respBytes, err := d.cliExecutor.ExecuteOCICLI(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("OCI CLI execution failed: %w", err)
	}

	// Parse OCI response
	var ociResp struct {
		Data struct {
			ID                   string `json:"id"`
			DisplayName          string `json:"displayName"`
			CPUCoreCount         int64  `json:"cpuCoreCount"`
			DataStorageSizeInGBs int64  `json:"dataStorageSizeInGBs"`
			DbVersion            string `json:"dbVersion"`
			LifecycleState       string `json:"lifecycleState"`
			CompartmentID        string `json:"compartmentId"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &ociResp); err != nil {
		return fmt.Errorf("failed to parse OCI response: %w", err)
	}

	// Map response to model
	data.AutonomousDatabaseID = types.StringValue(ociResp.Data.ID)
	data.DisplayName = types.StringValue(ociResp.Data.DisplayName)
	data.CPUCoreCount = types.Int64Value(ociResp.Data.CPUCoreCount)
	data.DataStorageSizeInGBs = types.Int64Value(ociResp.Data.DataStorageSizeInGBs)
	data.DbVersion = types.StringValue(ociResp.Data.DbVersion)
	data.LifecycleState = types.StringValue(ociResp.Data.LifecycleState)
	data.CompartmentID = types.StringValue(ociResp.Data.CompartmentID)
	data.Region = types.StringValue(region)

	tflog.Info(ctx, "Successfully read autonomous database from OCI",
		map[string]interface{}{
			"display_name": ociResp.Data.DisplayName,
			"state":        ociResp.Data.LifecycleState,
		},
	)

	return nil
}
