package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &BaseDatabaseResource{}
	_ resource.ResourceWithConfigure   = &BaseDatabaseResource{}
	_ resource.ResourceWithImportState = &BaseDatabaseResource{}
)

// NewBaseDatabaseResource is a helper function to simplify the provider implementation.
func NewBaseDatabaseResource() resource.Resource {
	return &BaseDatabaseResource{}
}

// BaseDatabaseResource is the resource implementation.
type BaseDatabaseResource struct {
	cliExecutor *CLIExecutor
}

// BaseDatabaseResourceModel describes the resource data model.
type BaseDatabaseResourceModel struct {
	ID            types.String `tfsdk:"id"`
	PrimaryCloud  types.String `tfsdk:"primary_cloud"`
	DbName        types.String `tfsdk:"db_name"`
	DbVersion     types.String `tfsdk:"db_version"`
	DbEdition     types.String `tfsdk:"db_edition"`
	DbHomeID      types.String `tfsdk:"db_home_id"`
	PdbName       types.String `tfsdk:"pdb_name"`
	AdminPassword types.String `tfsdk:"admin_password"` // Sensitive
	Region        types.String `tfsdk:"region"`
	CompartmentID types.String `tfsdk:"compartment_id"` // OCI
	LifecycleState types.String `tfsdk:"lifecycle_state"`
}

// Metadata returns the resource type name.
func (r *BaseDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_base_database"
}

// Schema defines the schema for the resource.
func (r *BaseDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a base database in OCI or Azure",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The OCID or resource ID of the database",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"primary_cloud": schema.StringAttribute{
				Description: "Primary cloud provider (oci or azure)",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"db_name": schema.StringAttribute{
				Description: "Database name (must be alphanumeric, max 8 characters for OCI)",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"db_version": schema.StringAttribute{
				Description: "Database version (e.g., 19.0.0.0, 21.0.0.0)",
				Required:    true,
			},
			"db_edition": schema.StringAttribute{
				Description: "Database edition (ENTERPRISE_EDITION, STANDARD_EDITION, etc.)",
				Optional:    true,
				Computed:    true,
			},
			"db_home_id": schema.StringAttribute{
				Description: "OCI-specific: DB Home OCID where the database will be created",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pdb_name": schema.StringAttribute{
				Description: "Pluggable database name",
				Optional:    true,
				Computed:    true,
			},
			"admin_password": schema.StringAttribute{
				Description: "Admin password for the database",
				Required:    true,
				Sensitive:   true,
			},
			"region": schema.StringAttribute{
				Description: "Region where the database should be created",
				Optional:    true,
				Computed:    true,
			},
			"compartment_id": schema.StringAttribute{
				Description: "OCI-specific: Compartment OCID",
				Optional:    true,
			},
			"lifecycle_state": schema.StringAttribute{
				Description: "Lifecycle state of the database",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *BaseDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cliExecutor, ok := req.ProviderData.(*CLIExecutor)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *CLIExecutor, got: %T.", req.ProviderData),
		)
		return
	}

	r.cliExecutor = cliExecutor
}

// Create creates the resource and sets the initial Terraform state.
func (r *BaseDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BaseDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	primaryCloud := plan.PrimaryCloud.ValueString()

	tflog.Info(ctx, "Creating base database",
		map[string]interface{}{
			"primary_cloud": primaryCloud,
			"db_name":       plan.DbName.ValueString(),
		},
	)

	switch primaryCloud {
	case "oci":
		if err := r.createInOCI(ctx, &plan); err != nil {
			resp.Diagnostics.AddError(
				"Failed to create in OCI",
				fmt.Sprintf("Could not create base database in OCI: %s", err.Error()),
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
			fmt.Sprintf("Unsupported primary cloud: %s", primaryCloud),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *BaseDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BaseDatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	primaryCloud := state.PrimaryCloud.ValueString()
	id := state.ID.ValueString()

	tflog.Info(ctx, "Reading base database",
		map[string]interface{}{
			"primary_cloud": primaryCloud,
			"id":            id,
		},
	)

	switch primaryCloud {
	case "oci":
		if err := r.readFromOCI(ctx, &state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to read from OCI",
				fmt.Sprintf("Could not read base database: %s", err.Error()),
			)
			return
		}

	case "azure":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"Azure base database support is not yet implemented",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *BaseDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state BaseDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	primaryCloud := state.PrimaryCloud.ValueString()

	tflog.Info(ctx, "Updating base database",
		map[string]interface{}{
			"primary_cloud": primaryCloud,
			"id":            state.ID.ValueString(),
		},
	)

	switch primaryCloud {
	case "oci":
		if err := r.updateInOCI(ctx, &plan, &state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to update in OCI",
				fmt.Sprintf("Could not update base database: %s", err.Error()),
			)
			return
		}

	case "azure":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"Azure base database support is not yet implemented",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *BaseDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BaseDatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	primaryCloud := state.PrimaryCloud.ValueString()
	id := state.ID.ValueString()

	tflog.Info(ctx, "Deleting base database",
		map[string]interface{}{
			"primary_cloud": primaryCloud,
			"id":            id,
		},
	)

	switch primaryCloud {
	case "oci":
		if err := r.deleteFromOCI(ctx, &state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to delete from OCI",
				fmt.Sprintf("Could not delete base database: %s", err.Error()),
			)
			return
		}

	case "azure":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"Azure base database support is not yet implemented",
		)
		return
	}
}

// ImportState implements resource.ResourceWithImportState.
func (r *BaseDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// createInOCI creates base database in OCI using CLI
func (r *BaseDatabaseResource) createInOCI(ctx context.Context, plan *BaseDatabaseResourceModel) error {
	region := plan.Region.ValueString()
	if region == "" {
		region = "us-ashburn-1"
		plan.Region = types.StringValue(region)
	}

	url := fmt.Sprintf("https://database.%s.oraclecloud.com/20160918/databases", region)

	// Prepare request body
	body := map[string]interface{}{
		"dbHomeId": plan.DbHomeID.ValueString(),
		"database": map[string]interface{}{
			"dbName":        plan.DbName.ValueString(),
			"adminPassword": plan.AdminPassword.ValueString(),
		},
	}

	// Add optional fields
	if !plan.PdbName.IsNull() && plan.PdbName.ValueString() != "" {
		body["database"].(map[string]interface{})["pdbName"] = plan.PdbName.ValueString()
	}

	respBytes, err := r.cliExecutor.ExecuteOCICLI(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("OCI CLI execution failed: %w", err)
	}

	// Parse response
	var ociResp struct {
		Data struct {
			ID             string `json:"id"`
			LifecycleState string `json:"lifecycleState"`
			DbName         string `json:"dbName"`
			DbVersion      string `json:"dbVersion"`
			PdbName        string `json:"pdbName"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &ociResp); err != nil {
		return fmt.Errorf("failed to parse OCI response: %w", err)
	}

	plan.ID = types.StringValue(ociResp.Data.ID)
	plan.LifecycleState = types.StringValue(ociResp.Data.LifecycleState)
	plan.DbVersion = types.StringValue(ociResp.Data.DbVersion)
	plan.PdbName = types.StringValue(ociResp.Data.PdbName)

	return nil
}

// readFromOCI reads base database from OCI
func (r *BaseDatabaseResource) readFromOCI(ctx context.Context, state *BaseDatabaseResourceModel) error {
	id := state.ID.ValueString()
	region := state.Region.ValueString()

	url := fmt.Sprintf("https://database.%s.oraclecloud.com/20160918/databases/%s", region, id)

	respBytes, err := r.cliExecutor.ExecuteOCICLI(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("OCI CLI execution failed: %w", err)
	}

	var ociResp struct {
		Data struct {
			ID             string `json:"id"`
			DbName         string `json:"dbName"`
			DbVersion      string `json:"dbVersion"`
			DbHomeID       string `json:"dbHomeId"`
			CompartmentID  string `json:"compartmentId"`
			PdbName        string `json:"pdbName"`
			LifecycleState string `json:"lifecycleState"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &ociResp); err != nil {
		return fmt.Errorf("failed to parse OCI response: %w", err)
	}

	// Update state
	state.DbName = types.StringValue(ociResp.Data.DbName)
	state.DbVersion = types.StringValue(ociResp.Data.DbVersion)
	state.DbHomeID = types.StringValue(ociResp.Data.DbHomeID)
	state.CompartmentID = types.StringValue(ociResp.Data.CompartmentID)
	state.PdbName = types.StringValue(ociResp.Data.PdbName)
	state.LifecycleState = types.StringValue(ociResp.Data.LifecycleState)

	return nil
}

// updateInOCI updates base database in OCI
func (r *BaseDatabaseResource) updateInOCI(ctx context.Context, plan, state *BaseDatabaseResourceModel) error {
	id := state.ID.ValueString()
	region := state.Region.ValueString()

	url := fmt.Sprintf("https://database.%s.oraclecloud.com/20160918/databases/%s", region, id)

	body := map[string]interface{}{}

	// Check if db_version changed
	if !plan.DbVersion.Equal(state.DbVersion) {
		body["dbVersion"] = plan.DbVersion.ValueString()
	}

	// Only send request if there are changes
	if len(body) > 0 {
		respBytes, err := r.cliExecutor.ExecuteOCICLI(ctx, "PUT", url, body)
		if err != nil {
			return fmt.Errorf("OCI CLI execution failed: %w", err)
		}

		var ociResp struct {
			Data struct {
				LifecycleState string `json:"lifecycleState"`
			} `json:"data"`
		}

		if err := json.Unmarshal(respBytes, &ociResp); err != nil {
			return fmt.Errorf("failed to parse OCI response: %w", err)
		}

		plan.LifecycleState = types.StringValue(ociResp.Data.LifecycleState)
	} else {
		// No changes, keep current lifecycle state
		plan.LifecycleState = state.LifecycleState
	}

	return nil
}

// deleteFromOCI deletes base database from OCI
func (r *BaseDatabaseResource) deleteFromOCI(ctx context.Context, state *BaseDatabaseResourceModel) error {
	id := state.ID.ValueString()
	region := state.Region.ValueString()

	url := fmt.Sprintf("https://database.%s.oraclecloud.com/20160918/databases/%s", region, id)

	_, err := r.cliExecutor.ExecuteOCICLI(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("OCI CLI execution failed: %w", err)
	}

	return nil
}
