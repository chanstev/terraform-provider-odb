terraform {
  required_providers {
    omc = {
      source  = "local/chanstev/omc"
      version = "~> 0.1"
    }
  }
}

provider "omc" {
  # Azure authentication
  azure {
    subscription_id = var.azure_subscription_id
    tenant_id       = var.azure_tenant_id
    # Uses Azure CLI authentication by default
    # Can also use service principal or managed identity
  }

  # OCI authentication (for cross-cloud operations)
  oci {
    config_file_profile = "DEFAULT"
    # Uses ~/.oci/config by default
  }
}

# Create an Autonomous Database on Azure
resource "omc_autonomous_database" "example" {
  # Specify which cloud manages this resource
  cloud = "azure"

  # Azure-specific configuration
  azure_region         = var.azure_region
  azure_resource_group = var.azure_resource_group

  # Resource naming
  name         = var.database_name
  display_name = var.database_display_name

  # Database configuration
  db_version    = "19c"
  character_set = "AL32UTF8"
  ncharacter_set = "AL16UTF16"

  # Compute and storage
  compute_model             = "ECPU"
  compute_count             = 2
  data_storage_size_in_tbs  = 1

  # Security
  admin_password                = var.admin_password
  is_mtls_connection_required   = true
  whitelisted_ips               = var.whitelisted_ips

  # Networking (Azure VNet)
  subnet_id = var.azure_subnet_id
  vnet_id   = var.azure_vnet_id

  # Auto-scaling
  is_auto_scaling_enabled             = true
  is_auto_scaling_for_storage_enabled = true

  # Licensing
  license_model = "LICENSE_INCLUDED"

  # Backup
  backup_retention_period_in_days = 60

  # Maintenance
  autonomous_maintenance_schedule_type = "REGULAR"

  # Azure tags (for Azure resource management)
  azure_tags = {
    Environment = "Development"
    Project     = "MyProject"
    ManagedBy   = "Terraform"
  }

  # OCI-specific fields (updatable via OCI API only)
  # IMPORTANT: These fields are set AFTER Azure creation completes
  # 1. Azure creates the database (5-15 minutes)
  # 2. Provider waits for lifecycleState = "AVAILABLE"
  # 3. Provider updates these fields via OCI API
  oci_open_mode = "READ_ONLY"  # READ_ONLY or READ_WRITE - controls database access

  # Optional: Set OCI tags (separate from Azure tags)
  # oci_tags = {
  #   Department = "Engineering"
  #   CostCenter = "1234"
  # }

  # Optional: Other OCI-only updatable fields
  # oci_db_workload = "OLTP"  # OLTP, DW, AJD, APEX, LH - OCI workload type
}

# Output
output "autonomous_database" {
  description = "Complete Autonomous Database object with all attributes"
  value       = omc_autonomous_database.example
  sensitive   = true  # Marked sensitive because it may contain connection strings
}

# Example: You can mix different clouds in the same configuration
#
# resource "omc_autonomous_database" "azure_db" {
#   cloud        = "azure"
#   azure_region = "eastus"
#   # ... Azure-specific config
# }
#
# resource "omc_autonomous_database" "oci_db" {
#   cloud          = "oci"
#   oci_region     = "us-ashburn-1"
#   compartment_id = var.oci_compartment_id
#   # ... OCI-specific config
# }
