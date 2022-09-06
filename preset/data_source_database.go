package preset

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vercel/terraform-provider-preset/client"
)

type Database struct {
	Name    types.String `tfsdk:"name"`
	Id      types.Int64  `tfsdk:"id"`
	Schemas types.List   `tfsdk:"schemas"`
}

type dataSourceDatabaseType struct{}

func (r dataSourceDatabaseType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.Int64Type,
				Computed: true,
			},
			"schemas": {
				Type:     types.ListType{ElemType: types.StringType},
				Computed: true,
			},
			"name": {
				Required: true,
				Type:     types.StringType,
			},
		},
	}, nil
}

func (r dataSourceDatabaseType) NewDataSource(ctx context.Context, p provider.Provider) (datasource.DataSource, diag.Diagnostics) {
	return dataSourceDatabase{
		p: *(p.(*presetProvider)),
	}, nil
}

type dataSourceDatabase struct {
	p presetProvider
}

func (r dataSourceDatabase) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config Database
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.GetApiV1DatabaseWithResponse(ctx, &client.GetApiV1DatabaseParams{
		Q: &client.GetListSchema{
			Filters: &[]struct {
				Col   string      `json:"col"`
				Opr   string      `json:"opr"`
				Value interface{} `json:"value"`
			}{{
				Col:   "database_name",
				Opr:   "==",
				Value: config.Name.Value,
			}},
		},
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading database",
			fmt.Sprintf("Could not read database %s, unexpected error: %s",
				config.Name.Value,
				err,
			),
		)
		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading database",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	if len(*res.JSON200.Result) != 1 {
		resp.Diagnostics.AddError(
			"Error reading database",
			fmt.Sprintf("Zero or more than one dashboard return for dashboard %s", config.Name.Value),
		)

		return
	}

	dashResult := (*res.JSON200.Result)[0]

	force := false
	resSchemas, err := r.p.client.GetApiV1DatabasePkSchemasWithResponse(ctx, int(*dashResult.Id), &client.GetApiV1DatabasePkSchemasParams{
		Q: &client.DatabaseSchemasQuerySchema{
			Force: &force,
		},
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading database schemas",
			fmt.Sprintf("Could not read database %s schemas, unexpected error: %s",
				config.Name.Value,
				err,
			),
		)
		return
	}

	if resSchemas.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading database schemas",
			fmt.Sprintf("%v response returned: %v", resSchemas.StatusCode(), string(resSchemas.Body)),
		)

		return
	}

	var schemas []attr.Value

	for _, schema := range *resSchemas.JSON200.Result {
		schemas = append(schemas, types.String{Value: schema})
	}

	result := &Database{
		Name:    config.Name,
		Id:      types.Int64{Value: int64(*dashResult.Id)},
		Schemas: types.List{ElemType: types.StringType, Elems: schemas},
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
