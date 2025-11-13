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
	_ datasource.DataSource              = &BaseDatabaseDataSource{}
	_ datasource.DataSourceWithConfigure = &BaseDatabaseDataSource{}
)

// NewBaseDatabaseDataSource is a helper function to simplify the provider implementation.
func NewBaseDatabaseDataSource() datasource.DataSource {
	return &BaseDatabaseDataSource{}
}

// BaseDatabaseDataSource is the data source implementation.
type BaseDatabaseDataSource struct {
	cliExecutor *CLIExecutor
}

// BaseDatabaseDataSourceModel describes the data source data model.
type BaseDatabaseDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	PrimaryCloud types.String `tfsdk:"primary_cloud"`
	DisplayName  types.String `tfsdk:"display_name"`
	DbEdition    types.String `tfsdk:"db_edition"`
	DbVersion    types.String `tfsdk:"db_version"`
	DbSystemID   types.String `tfsdk:"db_system_id"`
	DbHomeID     types.String `tfsdk:"db_home_id"`
	DbName       types.String `tfsdk:"db_name"`
	PdbName      types.String `tfsdk:"pdb_name"`
	Region       types.String `tfsdk:"region"`

	// Cloud-specific fields
	CompartmentID types.String `tfsdk:"compartment_id"` // OCI
	LifecycleState types.String `tfsdk:"lifecycle_state"`
}

// Metadata returns the data source type name.
func (d *BaseDatabaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_base_database"
}

// Schema defines the schema for the data source.
func (d *BaseDatabaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a base database from OCI or Azure",
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
			"db_edition": schema.StringAttribute{
				Description: "Database edition (e.g., ENTERPRISE_EDITION, STANDARD_EDITION)",
				Computed:    true,
			},
			"db_version": schema.StringAttribute{
				Description: "Database version",
				Computed:    true,
			},
			"db_system_id": schema.StringAttribute{
				Description: "The OCID of the DB System",
				Computed:    true,
			},
			"db_home_id": schema.StringAttribute{
				Description: "The OCID of the DB Home",
				Computed:    true,
			},
			"db_name": schema.StringAttribute{
				Description: "Database name",
				Computed:    true,
			},
			"pdb_name": schema.StringAttribute{
				Description: "Pluggable database name",
				Computed:    true,
			},
			"region": schema.StringAttribute{
				Description: "Region where the database is located",
				Computed:    true,
			},
			"compartment_id": schema.StringAttribute{
				Description: "OCI-specific: Compartment OCID",
				Computed:    true,
			},
			"lifecycle_state": schema.StringAttribute{
				Description: "Lifecycle state of the database",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *BaseDatabaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *BaseDatabaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BaseDatabaseDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	primaryCloud := data.PrimaryCloud.ValueString()
	id := data.ID.ValueString()

	tflog.Info(ctx, "Reading base database",
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
				fmt.Sprintf("Could not read base database from OCI: %s", err.Error()),
			)
			return
		}

	case "azure":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"Azure base database support is not yet implemented",
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

// readFromOCI reads base database from OCI using CLI
func (d *BaseDatabaseDataSource) readFromOCI(ctx context.Context, data *BaseDatabaseDataSourceModel) error {
	id := data.ID.ValueString()
	region := data.Region.ValueString()

	// Default to us-ashburn-1 if no region specified
	if region == "" {
		region = "us-ashburn-1"
	}

	// Construct OCI API endpoint for database
	url := fmt.Sprintf("https://database.%s.oraclecloud.com/20160918/databases/%s", region, id)

	// Execute OCI CLI
	respBytes, err := d.cliExecutor.ExecuteOCICLI(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("OCI CLI execution failed: %w", err)
	}

	// Parse OCI response
	var ociResp struct {
		Data struct {
			ID              string `json:"id"`
			DbName          string `json:"dbName"`
			DbUniqueName    string `json:"dbUniqueName"`
			DbWorkload      string `json:"dbWorkload"`
			DbVersion       string `json:"dbVersion"`
			DbSystemID      string `json:"dbSystemId"`
			DbHomeID        string `json:"dbHomeId"`
			CompartmentID   string `json:"compartmentId"`
			CharacterSet    string `json:"characterSet"`
			NcharacterSet   string `json:"ncharacterSet"`
			PdbName         string `json:"pdbName"`
			LifecycleState  string `json:"lifecycleState"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &ociResp); err != nil {
		return fmt.Errorf("failed to parse OCI response: %w", err)
	}

	// Map response to model
	data.DisplayName = types.StringValue(ociResp.Data.DbUniqueName)
	data.DbName = types.StringValue(ociResp.Data.DbName)
	data.DbVersion = types.StringValue(ociResp.Data.DbVersion)
	data.DbSystemID = types.StringValue(ociResp.Data.DbSystemID)
	data.DbHomeID = types.StringValue(ociResp.Data.DbHomeID)
	data.CompartmentID = types.StringValue(ociResp.Data.CompartmentID)
	data.PdbName = types.StringValue(ociResp.Data.PdbName)
	data.LifecycleState = types.StringValue(ociResp.Data.LifecycleState)
	data.Region = types.StringValue(region)

	tflog.Info(ctx, "Successfully read base database from OCI",
		map[string]interface{}{
			"db_name": ociResp.Data.DbName,
			"state":   ociResp.Data.LifecycleState,
		},
	)

	return nil
}
