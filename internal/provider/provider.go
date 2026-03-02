// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warehouse-13/hammertime/pkg/client"
	"github.com/warehouse-13/hammertime/pkg/defaults"
)

const FLINTLOCK_ENDPOINT_ENV_NAME = "FLINTLOCK_ENDPOINT"
const FLINTLOCK_AUTHTOKEN_ENV_NAME = "FLINTLOCK_AUTHTOKEN"

// Ensure FlintlockProvider satisfies various provider interfaces.
var _ provider.Provider = &FlintlockProvider{}

// FlintlockProvider defines the provider implementation.
type FlintlockProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// FlintlockProviderModel describes the provider data model.
type FlintlockProviderModel struct {
	AuthToken types.String `tfsdk:"authtoken"`
	Endpoint  types.String `tfsdk:"endpoint"`
}

func (p *FlintlockProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "flintlock"
	resp.Version = p.version
}

func (p *FlintlockProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"authtoken": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The authentication token used to authenticate with Flintlock. Alternatively, can be configured using the `%s` environment variable. Only required when the server is enabled for authentication.", FLINTLOCK_AUTHTOKEN_ENV_NAME),
				Optional:            true,
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The control endpoint served by Flintlock. Alternatively, can be configured using the `%s` environment variable. Defaults to `%s` if none are set.", FLINTLOCK_ENDPOINT_ENV_NAME, defaults.DialTarget),
				// TODO add a validator to ensure the endpoint is a valid URL
				Optional: true,
			},
		},
	}
}

func (p *FlintlockProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data FlintlockProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.AuthToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("authtoken"),
			"Unknown Flintlock API AuthToken",
			fmt.Sprintf("The provider cannot create the Flintlock API client as there is an unknown configuration value for the Flintlock API auth token. Either target apply the source of the value first, set the value statically in the configuration, or use the `%s` environment variable.", FLINTLOCK_AUTHTOKEN_ENV_NAME),
		)
	}
	if data.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Unknown Flintlock API Endpoint",
			fmt.Sprintf("The provider cannot create the Flintlock API client as there is an unknown configuration value for the Flintlock API endpoint. Either target apply the source of the value first, set the value statically in the configuration, or use the `%s` environment variable.", FLINTLOCK_ENDPOINT_ENV_NAME),
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	authToken := os.Getenv(FLINTLOCK_AUTHTOKEN_ENV_NAME)
	if !data.AuthToken.IsNull() {
		authToken = data.AuthToken.ValueString()
	}

	endpoint := os.Getenv(FLINTLOCK_ENDPOINT_ENV_NAME)
	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}
	// If an endpoint configuration value was not provided, use the Flintlock default
	if endpoint == "" {
		endpoint = defaults.DialTarget
	}

	// Example client configuration for data sources and resources
	client, err := client.New(endpoint, authToken)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create Flintlock API client",
			fmt.Sprintf("The provider cannot create the Flintlock API client. Please check the configuration and try again. Received error: %s", err),
		)
		return
	}
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *FlintlockProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *FlintlockProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVMsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FlintlockProvider{
			version: version,
		}
	}
}
