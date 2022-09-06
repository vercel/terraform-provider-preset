package preset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vercel/terraform-provider-preset/client"
)

type DatasetColumn struct {
	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`
}

type Dataset struct {
	Id         types.Int64     `tfsdk:"id"`
	Title      types.String    `tfsdk:"title"`
	Sql        types.String    `tfsdk:"sql"`
	DatabaseId types.Int64     `tfsdk:"database_id"`
	Columns    []DatasetColumn `tfsdk:"columns"`
}

type resourceDatasetType struct{}

func (r resourceDatasetType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:      true,
				Type:          types.Int64Type,
				PlanModifiers: tfsdk.AttributePlanModifiers{resource.UseStateForUnknown()},
			},
			"database_id": {
				Required: true,
				Type:     types.Int64Type,
			},
			"title": {
				Required: true,
				Type:     types.StringType,
			},
			"sql": {
				Required: true,
				Type:     types.StringType,
			},
			"columns": {
				Required: true,
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"name": {
						Required: true,
						Type:     types.StringType,
					},
					"type": {
						Required: true,
						Type:     types.StringType,
					},
				}),
			},
		},
	}, nil
}

func (r resourceDatasetType) NewResource(_ context.Context, p provider.Provider) (resource.Resource, diag.Diagnostics) {
	return resourceDataset{
		p: *p.(*presetProvider),
	}, nil
}

type resourceDataset struct {
	p presetProvider
}

type sqllabVizData struct {
	Schema         string `json:"schema"`
	Sql            string `json:"sql"`
	DbId           int    `json:"dbId"`
	DatasourceName string `json:"datasourceName"`
	Columns        []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"columns"`
}

func (r resourceDataset) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var dataset Dataset
	diags := req.Plan.Get(ctx, &dataset)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	force := false
	resSchemas, err := r.p.client.GetApiV1DatabasePkSchemasWithResponse(ctx, int(dataset.DatabaseId.Value), &client.GetApiV1DatabasePkSchemasParams{
		Q: &client.DatabaseSchemasQuerySchema{
			Force: &force,
		},
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading database schemas",
			fmt.Sprintf("Could not read database %d schemas, unexpected error: %s",
				dataset.DatabaseId.Value,
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

	var columns []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	for _, column := range dataset.Columns {
		columns = append(columns, struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}{
			Name: column.Name.Value,
			Type: column.Type.Value,
		})
	}

	var formData bytes.Buffer
	formDataWriter := multipart.NewWriter(&formData)
	data := &sqllabVizData{
		Schema:         (*resSchemas.JSON200.Result)[0],
		Sql:            dataset.Sql.Value,
		DbId:           int(dataset.DatabaseId.Value),
		DatasourceName: dataset.Title.Value,
		Columns:        columns,
	}
	dataSerialized, err := json.Marshal(data)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating dataset",
			"Could not marshal data for create request: "+err.Error(),
		)

		return
	}

	err = formDataWriter.WriteField("data", string(dataSerialized))

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating dataset",
			"Could not write data in create request: "+err.Error(),
		)

		return
	}

	formDataWriter.Close()

	res, err := r.p.client.PostSupersetSqllabVizWithBodyWithResponse(ctx, formDataWriter.FormDataContentType(), &formData)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating dataset",
			"Could not create dataset, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error creating dataset",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	isManagedExternally := true
	putRes, err := r.p.client.PutApiV1DatasetPkWithResponse(ctx, int(*res.JSON200.Data.Id), &client.PutApiV1DatasetPkParams{}, client.PutApiV1DatasetPkJSONRequestBody{
		IsManagedExternally: &isManagedExternally,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating dataset",
			"Could not create dataset, unexpected error: "+err.Error(),
		)

		return
	}

	if putRes.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error creating dataset",
			fmt.Sprintf("%v response returned: %v", putRes.StatusCode(), string(putRes.Body)),
		)

		return
	}

	result := &Dataset{
		Id:         types.Int64{Value: int64(*res.JSON200.Data.Id)},
		Title:      types.String{Value: *res.JSON200.Data.DatasourceName},
		Sql:        types.String{Value: *res.JSON200.Data.Sql},
		DatabaseId: dataset.DatabaseId,
		Columns:    dataset.Columns,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDataset) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var dataset Dataset
	diags := req.State.Get(ctx, &dataset)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.GetApiV1DatasetPkWithResponse(ctx, int(dataset.Id.Value), &client.GetApiV1DatasetPkParams{})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading dataset",
			"Could not read dataset, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading dataset",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	result := &Dataset{
		Id:         types.Int64{Value: int64(*res.JSON200.Result.Id)},
		Title:      types.String{Value: res.JSON200.Result.TableName},
		Sql:        types.String{Value: *res.JSON200.Result.Sql},
		DatabaseId: dataset.DatabaseId,
		Columns:    dataset.Columns,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDataset) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var dataset Dataset
	diags := req.Plan.Get(ctx, &dataset)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state Dataset
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isManagedExternally := true
	res, err := r.p.client.PutApiV1DatasetPkWithResponse(ctx, int(state.Id.Value), &client.PutApiV1DatasetPkParams{}, client.PutApiV1DatasetPkJSONRequestBody{
		Sql:                 &dataset.Sql.Value,
		IsManagedExternally: &isManagedExternally,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating dataset",
			"Could not update dataset, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating dataset",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	result := &Dataset{
		Id:         types.Int64{Value: int64(*res.JSON200.Id)},
		Title:      dataset.Title,
		Sql:        types.String{Value: *res.JSON200.Result.Sql},
		DatabaseId: dataset.DatabaseId,
		Columns:    dataset.Columns,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDataset) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Dataset
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.DeleteApiV1DatasetPkWithResponse(ctx, int(state.Id.Value))

	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting dataset",
			"Could not delete dataset, unexpected error: "+err.Error(),
		)
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error deleting dataset",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}
}
