package preset

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/vercel/terraform-provider-preset/client"
)

type presetProvider struct {
	client *client.ClientWithResponses
}

func New() provider.Provider {
	return &presetProvider{}
}

func (p *presetProvider) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"api_token": {
				Type:        types.StringType,
				Optional:    true,
				Description: "The Preset API Token to use. This can also be specified with the `PRESET_API_TOKEN` shell environment variable.",
				Sensitive:   true,
			},
			"api_secret": {
				Type:        types.StringType,
				Optional:    true,
				Description: "The Preset API Token secret to use. This can also be specified with the `PRESET_API_SECRET` shell environment variable.",
				Sensitive:   true,
			},
			"base_url": {
				Type:        types.StringType,
				Optional:    true,
				Description: "The Preset workspace URL.  This can also be specified with the `PRESET_BASE_URL` shell environment variable.",
			},
		},
	}, nil
}

func (p *presetProvider) GetResources(_ context.Context) (map[string]provider.ResourceType, diag.Diagnostics) {
	return map[string]provider.ResourceType{
		"preset_dashboard":        resourceDashboardType{},
		"preset_dashboard_filter": resourceDashboardFilterType{},
		"preset_dataset":          resourceDatasetType{},
		"preset_chart":            resourceChartType{},
	}, nil
}

func (p *presetProvider) GetDataSources(_ context.Context) (map[string]provider.DataSourceType, diag.Diagnostics) {
	return map[string]provider.DataSourceType{
		"preset_database": dataSourceDatabaseType{},
	}, nil
}

type providerData struct {
	ApiToken  types.String `tfsdk:"api_token"`
	ApiSecret types.String `tfsdk:"api_secret"`
	BaseURL   types.String `tfsdk:"base_url"`
}

func (p *presetProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerData
	req.Config.Get(ctx, &config)

	var apiToken string

	if config.ApiToken.Null {
		apiToken = os.Getenv("PRESET_API_TOKEN")
	} else {
		apiToken = config.ApiToken.Value
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Unable to find api_token",
			"api_token cannot be an empty string",
		)
		return
	}

	var apiTokenSecret string

	if config.ApiToken.Null {
		apiTokenSecret = os.Getenv("PRESET_API_SECRET")
	} else {
		apiTokenSecret = config.ApiSecret.Value
	}

	if apiTokenSecret == "" {
		resp.Diagnostics.AddError(
			"Unable to find api_secret",
			"api_secret cannot be an empty string",
		)
		return
	}

	var baseUrl string

	if config.BaseURL.Null {
		baseUrl = os.Getenv("PRESET_BASE_URL")
	} else {
		baseUrl = config.BaseURL.Value
	}

	if baseUrl == "" {
		resp.Diagnostics.AddError(
			"Unable to find base_url",
			"base_url cannot be an empty string",
		)
		return
	}

	tflog.Info(ctx, "Creating new client", map[string]interface{}{
		"baseUrl": baseUrl,
	})

	client, err := client.New(baseUrl, apiToken, apiTokenSecret)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating client",
			"Could not create client, unexpected error: "+err.Error(),
		)
		return
	}

	p.client = client
}
