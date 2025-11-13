package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// AuthClients holds authentication clients for each cloud provider
type AuthClients struct {
	OCI    *OCIAuthClient
	Azure  *AzureAuthClient
	Google *GoogleAuthClient
	AWS    *AWSAuthClient
}

// RESTClient provides HTTP client with authentication capabilities
type RESTClient struct {
	authClients *AuthClients
	httpClient  *http.Client
}

// NewRESTClient creates a new REST client with authentication
func NewRESTClient(authClients *AuthClients) *RESTClient {
	return &RESTClient{
		authClients: authClients,
		httpClient:  http.DefaultClient,
	}
}

// CloudContext represents the context information for a cloud provider
type CloudContext struct {
	Provider string
	Scope    string
	Region   string
	Verified bool
	Error    string
}

// GetCloudContext retrieves context information for all configured clouds
func (c *RESTClient) GetCloudContext(ctx context.Context) map[string]*CloudContext {
	contexts := make(map[string]*CloudContext)

	if c.authClients.OCI != nil {
		contexts["oci"] = c.authClients.OCI.GetContext(ctx)
	}
	if c.authClients.Azure != nil {
		contexts["azure"] = c.authClients.Azure.GetContext(ctx)
	}
	if c.authClients.Google != nil {
		contexts["google"] = c.authClients.Google.GetContext(ctx)
	}
	if c.authClients.AWS != nil {
		contexts["aws"] = c.authClients.AWS.GetContext(ctx)
	}

	return contexts
}

// MakeRequest makes an authenticated HTTP request to the specified cloud
func (c *RESTClient) MakeRequest(ctx context.Context, cloud, method, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		req.Body = http.NoBody
		req.ContentLength = int64(len(body))
	}

	// Inject authentication headers based on cloud provider
	switch cloud {
	case "oci":
		if c.authClients.OCI != nil {
			headers, err := c.authClients.OCI.GetAuthHeaders(req)
			if err != nil {
				return nil, err
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	case "azure":
		if c.authClients.Azure != nil {
			headers, err := c.authClients.Azure.GetAuthHeaders(req)
			if err != nil {
				return nil, err
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	case "google":
		if c.authClients.Google != nil {
			headers, err := c.authClients.Google.GetAuthHeaders(req)
			if err != nil {
				return nil, err
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	case "aws":
		if c.authClients.AWS != nil {
			headers, err := c.authClients.AWS.GetAuthHeaders(req)
			if err != nil {
				return nil, err
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	return c.httpClient.Do(req)
}

// OCIAuthClient handles OCI authentication
type OCIAuthClient struct {
	TenancyOCID string
	UserOCID    string
	Fingerprint string
	PrivateKey  string
	Region      string
	configProvider common.ConfigurationProvider
}

func (c *OCIAuthClient) GetAuthHeaders(req *http.Request) (map[string]string, error) {
	// OCI uses request signing, not simple headers
	// The SDK handles signing internally when making requests
	// This method is kept for interface compatibility but not used for OCI
	return make(map[string]string), nil
}

func (c *OCIAuthClient) GetContext(ctx context.Context) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "oci",
		Region:   c.Region,
		Scope:    c.TenancyOCID,
		Verified: false,
	}

	// Create OCI configuration provider
	configProvider := common.NewRawConfigurationProvider(
		c.TenancyOCID,
		c.UserOCID,
		c.Region,
		c.Fingerprint,
		c.PrivateKey,
		nil, // passphrase
	)

	// Create identity client
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to create OCI identity client: %v", err)
		return cloudCtx
	}

	// Call GetTenancy to verify authentication and get tenancy details
	getTenancyRequest := identity.GetTenancyRequest{
		TenancyId: common.String(c.TenancyOCID),
	}

	tenancyResp, err := identityClient.GetTenancy(ctx, getTenancyRequest)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to get OCI tenancy: %v", err)
		return cloudCtx
	}

	cloudCtx.Verified = true
	if tenancyResp.Name != nil {
		cloudCtx.Scope = fmt.Sprintf("%s (Tenancy: %s)", c.TenancyOCID, *tenancyResp.Name)
	}

	return cloudCtx
}

// AzureAuthClient handles Azure authentication
type AzureAuthClient struct {
	Token  string
	Region string
	Scope  string
}

func (c *AzureAuthClient) GetAuthHeaders(req *http.Request) (map[string]string, error) {
	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + c.Token
	return headers, nil
}

func (c *AzureAuthClient) GetContext(ctx context.Context) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "azure",
		Region:   c.Region,
		Scope:    c.Scope,
		Verified: false,
	}

	// Verify authentication by fetching subscription details
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s?api-version=2020-01-01", c.Scope)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to create request: %v", err)
		return cloudCtx
	}

	headers, err := c.GetAuthHeaders(req)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to get auth headers: %v", err)
		return cloudCtx
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("API call failed: %v", err)
		return cloudCtx
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		cloudCtx.Verified = true

		// Parse response to get subscription details
		body, _ := io.ReadAll(resp.Body)
		var subData map[string]interface{}
		if err := json.Unmarshal(body, &subData); err == nil {
			if displayName, ok := subData["displayName"].(string); ok {
				cloudCtx.Scope = fmt.Sprintf("%s (Subscription: %s)", c.Scope, displayName)
			}
		}
	} else {
		cloudCtx.Error = fmt.Sprintf("Authentication failed with status: %d", resp.StatusCode)
	}

	return cloudCtx
}

// GoogleAuthClient handles Google Cloud authentication
type GoogleAuthClient struct {
	Token  string
	Region string
	Scope  string
}

func (c *GoogleAuthClient) GetAuthHeaders(req *http.Request) (map[string]string, error) {
	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + c.Token
	return headers, nil
}

func (c *GoogleAuthClient) GetContext(ctx context.Context) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "google",
		Region:   c.Region,
		Scope:    c.Scope,
		Verified: false,
	}

	// Verify authentication by fetching project details
	url := fmt.Sprintf("https://cloudresourcemanager.googleapis.com/v1/projects/%s", c.Scope)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to create request: %v", err)
		return cloudCtx
	}

	headers, err := c.GetAuthHeaders(req)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to get auth headers: %v", err)
		return cloudCtx
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("API call failed: %v", err)
		return cloudCtx
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		cloudCtx.Verified = true

		// Parse response to get project details
		body, _ := io.ReadAll(resp.Body)
		var projectData map[string]interface{}
		if err := json.Unmarshal(body, &projectData); err == nil {
			if name, ok := projectData["name"].(string); ok {
				cloudCtx.Scope = fmt.Sprintf("%s (Project: %s)", c.Scope, name)
			}
			if projectNumber, ok := projectData["projectNumber"].(string); ok {
				cloudCtx.Scope = fmt.Sprintf("%s (Number: %s)", cloudCtx.Scope, projectNumber)
			}
		}
	} else {
		cloudCtx.Error = fmt.Sprintf("Authentication failed with status: %d", resp.StatusCode)
	}

	return cloudCtx
}

// AWSAuthClient handles AWS authentication
type AWSAuthClient struct {
	Token  string
	Region string
	Scope  string
}

func (c *AWSAuthClient) GetAuthHeaders(req *http.Request) (map[string]string, error) {
	headers := make(map[string]string)
	headers["X-Amz-Security-Token"] = c.Token
	headers["X-Amz-Region"] = c.Region
	return headers, nil
}

func (c *AWSAuthClient) GetContext(ctx context.Context) *CloudContext {
	cloudCtx := &CloudContext{
		Provider: "aws",
		Region:   c.Region,
		Scope:    c.Scope,
		Verified: false,
	}

	// Verify authentication by calling STS GetCallerIdentity
	// This is a simple API call that returns information about the AWS credentials
	url := "https://sts.amazonaws.com/?Action=GetCallerIdentity&Version=2011-06-15"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to create request: %v", err)
		return cloudCtx
	}

	headers, err := c.GetAuthHeaders(req)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("Failed to get auth headers: %v", err)
		return cloudCtx
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		cloudCtx.Error = fmt.Sprintf("API call failed: %v", err)
		return cloudCtx
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		cloudCtx.Verified = true

		// Parse response to get account details
		body, _ := io.ReadAll(resp.Body)
		// AWS STS returns XML, we'll keep it simple and just extract account ID
		bodyStr := string(body)
		if len(bodyStr) > 0 {
			// Simple extraction - in production use proper XML parsing
			cloudCtx.Scope = fmt.Sprintf("Account: %s", c.Scope)
		}
	} else {
		cloudCtx.Error = fmt.Sprintf("Authentication failed with status: %d", resp.StatusCode)
	}

	return cloudCtx
}
