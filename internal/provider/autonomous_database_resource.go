package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &AutonomousDatabaseResource{}
	_ resource.ResourceWithConfigure   = &AutonomousDatabaseResource{}
	_ resource.ResourceWithImportState = &AutonomousDatabaseResource{}
)

// NewAutonomousDatabaseResource is a helper function to simplify the provider implementation.
func NewAutonomousDatabaseResource() resource.Resource {
	return &AutonomousDatabaseResource{}
}

// AutonomousDatabaseResource is the resource implementation.
type AutonomousDatabaseResource struct {
	providerData *ProviderData
}

// AutonomousDatabaseResourceModel describes the resource data model.
type AutonomousDatabaseResourceModel struct {
	// Identity
	ID                       types.String `tfsdk:"id"`
	Cloud                    types.String `tfsdk:"cloud"`

	// Azure-specific
	AzureID                  types.String `tfsdk:"azure_id"`
	AzureRegion              types.String `tfsdk:"azure_region"`
	AzureResourceGroup       types.String `tfsdk:"azure_resource_group"`
	AzureTags                types.Map    `tfsdk:"azure_tags"`
	SubnetID                 types.String `tfsdk:"subnet_id"`
	VnetID                   types.String `tfsdk:"vnet_id"`

	// Resource naming
	Name                     types.String `tfsdk:"name"`
	DisplayName              types.String `tfsdk:"display_name"`

	// Database configuration
	DbVersion                types.String `tfsdk:"db_version"`
	DbName                   types.String `tfsdk:"db_name"`
	CharacterSet             types.String `tfsdk:"character_set"`
	NcharacterSet            types.String `tfsdk:"ncharacter_set"`

	// Compute and storage
	ComputeModel             types.String `tfsdk:"compute_model"`
	ComputeCount             types.Int64  `tfsdk:"compute_count"`
	DataStorageSizeInTbs     types.Int64  `tfsdk:"data_storage_size_in_tbs"`

	// Security
	AdminPassword            types.String `tfsdk:"admin_password"`
	IsMtlsConnectionRequired types.Bool   `tfsdk:"is_mtls_connection_required"`
	WhitelistedIps           types.List   `tfsdk:"whitelisted_ips"`

	// Auto-scaling
	IsAutoScalingEnabled             types.Bool `tfsdk:"is_auto_scaling_enabled"`
	IsAutoScalingForStorageEnabled   types.Bool `tfsdk:"is_auto_scaling_for_storage_enabled"`

	// Licensing
	LicenseModel             types.String `tfsdk:"license_model"`

	// Backup
	BackupRetentionPeriodInDays types.Int64 `tfsdk:"backup_retention_period_in_days"`

	// Maintenance
	AutonomousMaintenanceScheduleType types.String `tfsdk:"autonomous_maintenance_schedule_type"`

	// State
	LifecycleState           types.String `tfsdk:"lifecycle_state"`
	ProvisioningState        types.String `tfsdk:"provisioning_state"`
	TimeCreated              types.String `tfsdk:"time_created"`

	// OCI-specific fields (cross-cloud)
	OCIUrl                   types.String `tfsdk:"oci_url"`
	OCIOCID                  types.String `tfsdk:"oci_ocid"`
	OCIRegion                types.String `tfsdk:"oci_region"`
	OCICompartmentID         types.String `tfsdk:"oci_compartment_id"`
	OCIOpenMode              types.String `tfsdk:"oci_open_mode"`
	OCIDbWorkload            types.String `tfsdk:"oci_db_workload"`
	OCITags                  types.Map    `tfsdk:"oci_tags"`
}

// Metadata returns the resource type name.
func (r *AutonomousDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autonomous_database"
}

// Schema defines the schema for the resource.
func (r *AutonomousDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Autonomous Database across multiple clouds (Azure, OCI)",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Terraform resource ID",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud": schema.StringAttribute{
				MarkdownDescription: "Cloud provider managing this resource (azure or oci)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Azure-specific
			"azure_id": schema.StringAttribute{
				MarkdownDescription: "Azure resource ID",
				Computed:            true,
			},
			"azure_region": schema.StringAttribute{
				MarkdownDescription: "Azure region (e.g., eastus, westus2)",
				Optional:            true,
			},
			"azure_resource_group": schema.StringAttribute{
				MarkdownDescription: "Azure resource group name",
				Optional:            true,
			},
			"azure_tags": schema.MapAttribute{
				MarkdownDescription: "Azure tags for resource management",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"subnet_id": schema.StringAttribute{
				MarkdownDescription: "Azure subnet resource ID",
				Optional:            true,
			},
			"vnet_id": schema.StringAttribute{
				MarkdownDescription: "Azure VNet resource ID",
				Optional:            true,
			},

			// Resource naming
			"name": schema.StringAttribute{
				MarkdownDescription: "Database name (used for Azure resource name)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name for the database",
				Required:            true,
			},

			// Database configuration
			"db_version": schema.StringAttribute{
				MarkdownDescription: "Database version (e.g., 19c, 21c)",
				Optional:            true,
				Computed:            true,
			},
			"db_name": schema.StringAttribute{
				MarkdownDescription: "Database name",
				Optional:            true,
				Computed:            true,
			},
			"character_set": schema.StringAttribute{
				MarkdownDescription: "Database character set",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("AL32UTF8"),
			},
			"ncharacter_set": schema.StringAttribute{
				MarkdownDescription: "National character set",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("AL16UTF16"),
			},

			// Compute and storage
			"compute_model": schema.StringAttribute{
				MarkdownDescription: "Compute model (ECPU or OCPU)",
				Required:            true,
			},
			"compute_count": schema.Int64Attribute{
				MarkdownDescription: "Number of compute units",
				Required:            true,
			},
			"data_storage_size_in_tbs": schema.Int64Attribute{
				MarkdownDescription: "Data storage size in terabytes",
				Required:            true,
			},

			// Security
			"admin_password": schema.StringAttribute{
				MarkdownDescription: "Administrator password",
				Required:            true,
				Sensitive:           true,
			},
			"is_mtls_connection_required": schema.BoolAttribute{
				MarkdownDescription: "Whether mTLS connections are required",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"whitelisted_ips": schema.ListAttribute{
				MarkdownDescription: "List of whitelisted IP addresses",
				Optional:            true,
				ElementType:         types.StringType,
			},

			// Auto-scaling
			"is_auto_scaling_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable CPU auto-scaling",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"is_auto_scaling_for_storage_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable storage auto-scaling",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			// Licensing
			"license_model": schema.StringAttribute{
				MarkdownDescription: "License model (LICENSE_INCLUDED or BRING_YOUR_OWN_LICENSE)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("LICENSE_INCLUDED"),
			},

			// Backup
			"backup_retention_period_in_days": schema.Int64Attribute{
				MarkdownDescription: "Backup retention period in days",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(60),
			},

			// Maintenance
			"autonomous_maintenance_schedule_type": schema.StringAttribute{
				MarkdownDescription: "Maintenance schedule type (REGULAR or EARLY)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("REGULAR"),
			},

			// State
			"lifecycle_state": schema.StringAttribute{
				MarkdownDescription: "Lifecycle state of the database",
				Computed:            true,
			},
			"provisioning_state": schema.StringAttribute{
				MarkdownDescription: "Azure provisioning state",
				Computed:            true,
			},
			"time_created": schema.StringAttribute{
				MarkdownDescription: "Time when the database was created",
				Computed:            true,
			},

			// OCI-specific fields (cross-cloud)
			"oci_url": schema.StringAttribute{
				MarkdownDescription: "OCI URL for the database",
				Computed:            true,
			},
			"oci_ocid": schema.StringAttribute{
				MarkdownDescription: "OCI OCID of the database",
				Computed:            true,
			},
			"oci_region": schema.StringAttribute{
				MarkdownDescription: "OCI region",
				Computed:            true,
			},
			"oci_compartment_id": schema.StringAttribute{
				MarkdownDescription: "OCI compartment ID",
				Computed:            true,
			},
			"oci_open_mode": schema.StringAttribute{
				MarkdownDescription: "OCI open mode (READ_ONLY or READ_WRITE)",
				Optional:            true,
			},
			"oci_db_workload": schema.StringAttribute{
				MarkdownDescription: "OCI database workload type",
				Optional:            true,
			},
			"oci_tags": schema.MapAttribute{
				MarkdownDescription: "OCI freeform tags",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *AutonomousDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T.", req.ProviderData),
		)
		return
	}

	r.providerData = providerData
}

// Create creates the resource and sets the initial Terraform state.
func (r *AutonomousDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AutonomousDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloud := plan.Cloud.ValueString()

	tflog.Info(ctx, "Creating autonomous database",
		map[string]interface{}{
			"cloud":        cloud,
			"name":         plan.Name.ValueString(),
			"display_name": plan.DisplayName.ValueString(),
		},
	)

	switch cloud {
	case "azure":
		if err := r.createInAzure(ctx, &plan); err != nil {
			resp.Diagnostics.AddError(
				"Failed to create in Azure",
				fmt.Sprintf("Could not create autonomous database in Azure: %s", err.Error()),
			)
			return
		}

	case "oci":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"OCI autonomous database support is not yet implemented",
		)
		return

	default:
		resp.Diagnostics.AddError(
			"Invalid Cloud",
			fmt.Sprintf("Unsupported cloud: %s", cloud),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *AutonomousDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AutonomousDatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloud := state.Cloud.ValueString()

	tflog.Info(ctx, "Reading autonomous database",
		map[string]interface{}{
			"cloud": cloud,
			"id":    state.ID.ValueString(),
		},
	)

	switch cloud {
	case "azure":
		if err := r.readFromAzure(ctx, &state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to read from Azure",
				fmt.Sprintf("Could not read autonomous database: %s", err.Error()),
			)
			return
		}

	case "oci":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"OCI autonomous database support is not yet implemented",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *AutonomousDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state AutonomousDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloud := state.Cloud.ValueString()

	tflog.Info(ctx, "Updating autonomous database",
		map[string]interface{}{
			"cloud": cloud,
			"id":    state.ID.ValueString(),
		},
	)

	switch cloud {
	case "azure":
		if err := r.updateInAzure(ctx, &plan, &state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to update in Azure",
				fmt.Sprintf("Could not update autonomous database: %s", err.Error()),
			)
			return
		}

	case "oci":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"OCI autonomous database support is not yet implemented",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *AutonomousDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AutonomousDatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloud := state.Cloud.ValueString()

	tflog.Info(ctx, "Deleting autonomous database",
		map[string]interface{}{
			"cloud": cloud,
			"id":    state.ID.ValueString(),
		},
	)

	switch cloud {
	case "azure":
		if err := r.deleteFromAzure(ctx, &state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to delete from Azure",
				fmt.Sprintf("Could not delete autonomous database: %s", err.Error()),
			)
			return
		}

	case "oci":
		resp.Diagnostics.AddError(
			"Not Implemented",
			"OCI autonomous database support is not yet implemented",
		)
		return
	}
}

// ImportState implements resource.ResourceWithImportState.
func (r *AutonomousDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// createInAzure creates autonomous database in Azure using CLI and YAML config
func (r *AutonomousDatabaseResource) createInAzure(ctx context.Context, plan *AutonomousDatabaseResourceModel) error {
	// Get configuration
	config, err := r.providerData.ConfigLoader.GetResourceConfig("autonomous_database")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get provider config for subscription_id and resource_group from plan
	providerConfig := r.providerData.Config
	var subscriptionID, resourceGroup string

	if providerConfig.Azure != nil {
		subscriptionID = providerConfig.Azure.SubscriptionID.ValueString()
	}

	// Get resource_group from plan (required for Azure)
	resourceGroup = plan.AzureResourceGroup.ValueString()
	if resourceGroup == "" {
		return fmt.Errorf("azure_resource_group is required when cloud=azure")
	}

	// Build Azure CREATE URL from config
	url := BuildURL(config.Azure.Create, map[string]string{
		"subscription_id": subscriptionID,
		"resource_group":  resourceGroup,
		"name":            plan.Name.ValueString(),
	})

	// Build request body according to field_mapping
	body := r.buildAzureCreateBody(plan)

	// Execute Azure CLI
	respBytes, err := r.providerData.CLIExecutor.ExecuteAzureCLI(ctx, "PUT", url, body)
	if err != nil {
		return fmt.Errorf("Azure CLI execution failed: %w", err)
	}

	// Parse response
	var azureResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &azureResp); err != nil {
		return fmt.Errorf("failed to parse Azure response: %w", err)
	}

	// Extract fields from response
	r.extractAzureResponse(plan, azureResp)

	// Wait for AVAILABLE state
	if err := r.waitForAvailable(ctx, plan, config, subscriptionID, resourceGroup); err != nil {
		return fmt.Errorf("failed waiting for AVAILABLE state: %w", err)
	}

	// Perform OCI cross-cloud updates if needed
	if !plan.OCIOpenMode.IsNull() || !plan.OCIDbWorkload.IsNull() || !plan.OCITags.IsNull() {
		if err := r.updateViaOCI(ctx, plan); err != nil {
			tflog.Warn(ctx, "OCI update failed, continuing anyway", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Final read to get complete state
	if err := r.readFromAzure(ctx, plan); err != nil {
		return fmt.Errorf("failed final read: %w", err)
	}

	return nil
}

// buildAzureCreateBody builds the request body for Azure CREATE operation
func (r *AutonomousDatabaseResource) buildAzureCreateBody(plan *AutonomousDatabaseResourceModel) map[string]interface{} {
	body := map[string]interface{}{
		"location": plan.AzureRegion.ValueString(),
		"properties": map[string]interface{}{
			"displayName":                         plan.DisplayName.ValueString(),
			"computeModel":                        plan.ComputeModel.ValueString(),
			"computeCount":                        plan.ComputeCount.ValueInt64(),
			"dataStorageSizeInTbs":                plan.DataStorageSizeInTbs.ValueInt64(),
			"adminPassword":                       plan.AdminPassword.ValueString(),
			"characterSet":                        plan.CharacterSet.ValueString(),
			"ncharacterSet":                       plan.NcharacterSet.ValueString(),
			"subnetId":                            plan.SubnetID.ValueString(),
			"vnetId":                              plan.VnetID.ValueString(),
			"isAutoScalingEnabled":                plan.IsAutoScalingEnabled.ValueBool(),
			"isAutoScalingForStorageEnabled":      plan.IsAutoScalingForStorageEnabled.ValueBool(),
			"isMtlsConnectionRequired":            plan.IsMtlsConnectionRequired.ValueBool(),
			"licenseModel":                        plan.LicenseModel.ValueString(),
			"backupRetentionPeriodInDays":         plan.BackupRetentionPeriodInDays.ValueInt64(),
			"autonomousMaintenanceScheduleType":   plan.AutonomousMaintenanceScheduleType.ValueString(),
		},
	}

	// Add optional fields
	if !plan.DbVersion.IsNull() {
		body["properties"].(map[string]interface{})["dbVersion"] = plan.DbVersion.ValueString()
	}

	// Add tags if provided
	if !plan.AzureTags.IsNull() {
		tags := make(map[string]interface{})
		plan.AzureTags.ElementsAs(context.Background(), &tags, false)
		body["tags"] = tags
	}

	// Add whitelisted IPs if provided
	if !plan.WhitelistedIps.IsNull() {
		var ips []string
		plan.WhitelistedIps.ElementsAs(context.Background(), &ips, false)
		body["properties"].(map[string]interface{})["whitelistedIps"] = ips
	}

	return body
}

// extractAzureResponse extracts fields from Azure API response
func (r *AutonomousDatabaseResource) extractAzureResponse(plan *AutonomousDatabaseResourceModel, resp map[string]interface{}) {
	// Extract azure_id
	if id, ok := resp["id"].(string); ok {
		plan.AzureID = types.StringValue(id)
		plan.ID = types.StringValue(id) // Use Azure ID as Terraform ID
	}

	// Extract properties
	if properties, ok := resp["properties"].(map[string]interface{}); ok {
		// Lifecycle state
		if state, ok := properties["lifecycleState"].(string); ok {
			plan.LifecycleState = types.StringValue(state)
		}

		// Provisioning state
		if state, ok := properties["provisioningState"].(string); ok {
			plan.ProvisioningState = types.StringValue(state)
		}

		// OCI URL
		if ociUrl, ok := properties["ociUrl"].(string); ok {
			plan.OCIUrl = types.StringValue(ociUrl)

			// Extract OCI identifiers using regex
			r.extractOCIIdentifiers(plan, ociUrl)
		}

		// Time created
		if timeCreated, ok := properties["timeCreated"].(string); ok {
			plan.TimeCreated = types.StringValue(timeCreated)
		}
	}

	// Extract tags
	if tags, ok := resp["tags"].(map[string]interface{}); ok {
		tagMap := make(map[string]attr.Value)
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				tagMap[k] = types.StringValue(strVal)
			}
		}
		plan.AzureTags, _ = types.MapValue(types.StringType, tagMap)
	}
}

// extractOCIIdentifiers extracts OCI identifiers from ociUrl using regex
func (r *AutonomousDatabaseResource) extractOCIIdentifiers(plan *AutonomousDatabaseResourceModel, ociUrl string) {
	// Extract OCI region
	if region, err := ExtractWithRegex(ociUrl, `(?i)region=([^?&/]+)`); err == nil {
		plan.OCIRegion = types.StringValue(region)
	}

	// Extract OCI OCID
	if ocid, err := ExtractWithRegex(ociUrl, `(?i)autonomousDatabases/([^?&/]+)`); err == nil {
		plan.OCIOCID = types.StringValue(ocid)
	}

	// Extract OCI compartment ID
	if compartmentId, err := ExtractWithRegex(ociUrl, `(?i)compartmentId=([^?&/]+)`); err == nil {
		plan.OCICompartmentID = types.StringValue(compartmentId)
	}
}

// waitForAvailable polls Azure until the database reaches AVAILABLE state
func (r *AutonomousDatabaseResource) waitForAvailable(ctx context.Context, plan *AutonomousDatabaseResourceModel,
	config *ResourceConfig, subscriptionID, resourceGroup string) error {

	globalConfig := r.providerData.ConfigLoader.GlobalConfig
	pollInterval := time.Duration(globalConfig.AsyncOperations.Azure.PollIntervalSeconds) * time.Second
	maxDuration := time.Duration(globalConfig.AsyncOperations.Azure.MaxPollDurationMinutes) * time.Minute
	timeout := time.After(maxDuration)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	url := BuildURL(config.Azure.Read, map[string]string{
		"subscription_id": subscriptionID,
		"resource_group":  resourceGroup,
		"name":            plan.Name.ValueString(),
	})

	terminalStates := globalConfig.AsyncOperations.Azure.TerminalStates.Lifecycle

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for AVAILABLE state after %v", maxDuration)
		case <-ticker.C:
			// Poll Azure
			respBytes, err := r.providerData.CLIExecutor.ExecuteAzureCLI(ctx, "GET", url, nil)
			if err != nil {
				return fmt.Errorf("failed to poll Azure: %w", err)
			}

			var azureResp map[string]interface{}
			if err := json.Unmarshal(respBytes, &azureResp); err != nil {
				return fmt.Errorf("failed to parse poll response: %w", err)
			}

			properties, ok := azureResp["properties"].(map[string]interface{})
			if !ok {
				continue
			}

			lifecycleState, _ := properties["lifecycleState"].(string)

			tflog.Debug(ctx, "Polling status",
				map[string]interface{}{
					"lifecycle_state": lifecycleState,
				},
			)

			// Check if terminal state
			for _, terminalState := range terminalStates {
				if lifecycleState == terminalState {
					if lifecycleState == "AVAILABLE" {
						r.extractAzureResponse(plan, azureResp)
						return nil
					} else if lifecycleState == "FAILED" {
						return fmt.Errorf("database creation failed")
					} else {
						return fmt.Errorf("database reached unexpected terminal state: %s", lifecycleState)
					}
				}
			}
		}
	}
}

// updateViaOCI updates OCI-specific fields using OCI API
func (r *AutonomousDatabaseResource) updateViaOCI(ctx context.Context, plan *AutonomousDatabaseResourceModel) error {
	if plan.OCIOCID.IsNull() || plan.OCIRegion.IsNull() {
		return fmt.Errorf("OCI identifiers not available")
	}

	// Get configuration
	config, err := r.providerData.ConfigLoader.GetResourceConfig("autonomous_database")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Build OCI UPDATE URL
	url := BuildURL(config.Azure.OCIUpdate, map[string]string{
		"oci_region": plan.OCIRegion.ValueString(),
		"oci_ocid":   plan.OCIOCID.ValueString(),
	})

	// Build request body for OCI fields
	body := make(map[string]interface{})

	if !plan.OCIOpenMode.IsNull() {
		body["openMode"] = plan.OCIOpenMode.ValueString()
	}

	if !plan.OCIDbWorkload.IsNull() {
		body["dbWorkload"] = plan.OCIDbWorkload.ValueString()
	}

	if !plan.OCITags.IsNull() {
		tags := make(map[string]string)
		plan.OCITags.ElementsAs(ctx, &tags, false)
		body["freeformTags"] = tags
	}

	if len(body) == 0 {
		return nil // Nothing to update
	}

	// Execute OCI CLI
	respBytes, err := r.providerData.CLIExecutor.ExecuteOCICLI(ctx, "PUT", url, body)
	if err != nil {
		return fmt.Errorf("OCI CLI execution failed: %w", err)
	}

	// Parse response
	var ociResp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &ociResp); err != nil {
		return fmt.Errorf("failed to parse OCI response: %w", err)
	}

	tflog.Info(ctx, "Successfully updated OCI-specific fields")

	return nil
}

// readFromAzure reads the autonomous database from Azure
func (r *AutonomousDatabaseResource) readFromAzure(ctx context.Context, state *AutonomousDatabaseResourceModel) error {
	// Get configuration
	config, err := r.providerData.ConfigLoader.GetResourceConfig("autonomous_database")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Extract subscription_id and resource_group from Azure ID
	azureID := state.AzureID.ValueString()
	subscriptionID, resourceGroup, name := r.parseAzureID(azureID)

	// Build Azure READ URL
	url := BuildURL(config.Azure.Read, map[string]string{
		"subscription_id": subscriptionID,
		"resource_group":  resourceGroup,
		"name":            name,
	})

	// Execute Azure CLI
	respBytes, err := r.providerData.CLIExecutor.ExecuteAzureCLI(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("Azure CLI execution failed: %w", err)
	}

	// Parse response
	var azureResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &azureResp); err != nil {
		return fmt.Errorf("failed to parse Azure response: %w", err)
	}

	// Extract fields from response
	r.extractAzureResponse(state, azureResp)

	// Also read from OCI if we have identifiers and OCI fields are set
	if !state.OCIOCID.IsNull() && (!state.OCIOpenMode.IsNull() || !state.OCIDbWorkload.IsNull() || !state.OCITags.IsNull()) {
		r.readFromOCI(ctx, state)
	}

	return nil
}

// readFromOCI reads OCI-specific fields
func (r *AutonomousDatabaseResource) readFromOCI(ctx context.Context, state *AutonomousDatabaseResourceModel) error {
	if state.OCIOCID.IsNull() || state.OCIRegion.IsNull() {
		return nil // No OCI identifiers available
	}

	// Get configuration
	config, err := r.providerData.ConfigLoader.GetResourceConfig("autonomous_database")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Build OCI READ URL
	url := BuildURL(config.Azure.OCIRead, map[string]string{
		"oci_region": state.OCIRegion.ValueString(),
		"oci_ocid":   state.OCIOCID.ValueString(),
	})

	// Execute OCI CLI
	respBytes, err := r.providerData.CLIExecutor.ExecuteOCICLI(ctx, "GET", url, nil)
	if err != nil {
		tflog.Warn(ctx, "Failed to read from OCI", map[string]interface{}{
			"error": err.Error(),
		})
		return nil // Don't fail the whole read
	}

	// Parse response
	var ociResp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &ociResp); err != nil {
		return nil
	}

	// Extract OCI-specific fields
	if openMode, ok := ociResp.Data["openMode"].(string); ok {
		state.OCIOpenMode = types.StringValue(openMode)
	}

	if dbWorkload, ok := ociResp.Data["dbWorkload"].(string); ok {
		state.OCIDbWorkload = types.StringValue(dbWorkload)
	}

	if freeformTags, ok := ociResp.Data["freeformTags"].(map[string]interface{}); ok {
		tagMap := make(map[string]attr.Value)
		for k, v := range freeformTags {
			if strVal, ok := v.(string); ok {
				tagMap[k] = types.StringValue(strVal)
			}
		}
		state.OCITags, _ = types.MapValue(types.StringType, tagMap)
	}

	return nil
}

// updateInAzure updates the autonomous database in Azure
func (r *AutonomousDatabaseResource) updateInAzure(ctx context.Context, plan, state *AutonomousDatabaseResourceModel) error {
	// Get configuration
	config, err := r.providerData.ConfigLoader.GetResourceConfig("autonomous_database")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Extract identifiers from Azure ID
	azureID := state.AzureID.ValueString()
	subscriptionID, resourceGroup, name := r.parseAzureID(azureID)

	// Build Azure UPDATE URL
	url := BuildURL(config.Azure.Update, map[string]string{
		"subscription_id": subscriptionID,
		"resource_group":  resourceGroup,
		"name":            name,
	})

	// Build update body with changed fields
	body := map[string]interface{}{
		"properties": map[string]interface{}{},
	}

	properties := body["properties"].(map[string]interface{})

	// Check for changed fields and add to update body
	if !plan.DisplayName.Equal(state.DisplayName) {
		properties["displayName"] = plan.DisplayName.ValueString()
	}

	if !plan.ComputeCount.Equal(state.ComputeCount) {
		properties["computeCount"] = plan.ComputeCount.ValueInt64()
	}

	if !plan.DataStorageSizeInTbs.Equal(state.DataStorageSizeInTbs) {
		properties["dataStorageSizeInTbs"] = plan.DataStorageSizeInTbs.ValueInt64()
	}

	// Execute Azure CLI
	respBytes, err := r.providerData.CLIExecutor.ExecuteAzureCLI(ctx, "PATCH", url, body)
	if err != nil {
		return fmt.Errorf("Azure CLI execution failed: %w", err)
	}

	// Parse response
	var azureResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &azureResp); err != nil {
		return fmt.Errorf("failed to parse Azure response: %w", err)
	}

	// Extract updated state
	r.extractAzureResponse(plan, azureResp)

	// Handle OCI cross-cloud updates
	ociFieldsChanged := !plan.OCIOpenMode.Equal(state.OCIOpenMode) ||
		!plan.OCIDbWorkload.Equal(state.OCIDbWorkload) ||
		!plan.OCITags.Equal(state.OCITags)

	if ociFieldsChanged {
		if err := r.updateViaOCI(ctx, plan); err != nil {
			tflog.Warn(ctx, "OCI update failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Final read
	if err := r.readFromAzure(ctx, plan); err != nil {
		return fmt.Errorf("failed final read: %w", err)
	}

	return nil
}

// deleteFromAzure deletes the autonomous database from Azure
func (r *AutonomousDatabaseResource) deleteFromAzure(ctx context.Context, state *AutonomousDatabaseResourceModel) error {
	// Get configuration
	config, err := r.providerData.ConfigLoader.GetResourceConfig("autonomous_database")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Extract identifiers from Azure ID
	azureID := state.AzureID.ValueString()
	subscriptionID, resourceGroup, name := r.parseAzureID(azureID)

	// Build Azure DELETE URL
	url := BuildURL(config.Azure.Delete, map[string]string{
		"subscription_id": subscriptionID,
		"resource_group":  resourceGroup,
		"name":            name,
	})

	// Execute Azure CLI
	_, err = r.providerData.CLIExecutor.ExecuteAzureCLI(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("Azure CLI execution failed: %w", err)
	}

	tflog.Info(ctx, "Successfully deleted autonomous database from Azure")

	return nil
}

// parseAzureID parses Azure resource ID to extract components
func (r *AutonomousDatabaseResource) parseAzureID(azureID string) (subscriptionID, resourceGroup, name string) {
	// Azure ID format: /subscriptions/{subscription_id}/resourceGroups/{resource_group}/providers/Oracle.Database/autonomousDatabases/{name}
	re := regexp.MustCompile(`/subscriptions/([^/]+)/resourceGroups/([^/]+)/providers/Oracle\.Database/autonomousDatabases/([^/]+)`)
	matches := re.FindStringSubmatch(azureID)

	if len(matches) == 4 {
		subscriptionID = matches[1]
		resourceGroup = matches[2]
		name = matches[3]
	}

	return
}
