package preset

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vercel/terraform-provider-preset/client"
)

type Chart struct {
	Id          types.Int64  `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	DashboardId types.Int64  `tfsdk:"dashboard_id"`
	DatasetId   types.Int64  `tfsdk:"dataset_id"`
	VizType     types.String `tfsdk:"viz_type"`
	Params      types.String `tfsdk:"params"`
}

type resourceChartType struct{}

func (r resourceChartType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:      true,
				Type:          types.Int64Type,
				PlanModifiers: tfsdk.AttributePlanModifiers{resource.UseStateForUnknown()},
			},
			"title": {
				Required: true,
				Type:     types.StringType,
			},
			"dashboard_id": {
				Required: true,
				Type:     types.Int64Type,
			},
			"dataset_id": {
				Required: true,
				Type:     types.Int64Type,
			},
			"viz_type": {
				Required: true,
				Type:     types.StringType,
			},
			"params": {
				Required: true,
				Type:     types.StringType,
			},
		},
	}, nil
}

func (r resourceChartType) NewResource(_ context.Context, p provider.Provider) (resource.Resource, diag.Diagnostics) {
	return resourceChart{
		p: *p.(*presetProvider),
	}, nil
}

type resourceChart struct {
	p presetProvider
}

func (r resourceChart) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var chart Chart
	diags := req.Plan.Get(ctx, &chart)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboardId := int32(chart.DashboardId.Value)
	isManagedExternally := true
	res, err := r.p.client.PostApiV1ChartWithResponse(ctx, client.PostApiV1ChartJSONRequestBody{
		SliceName:           chart.Title.Value,
		DatasourceId:        int32(chart.DatasetId.Value),
		DatasourceType:      "table",
		Dashboards:          &[]int32{dashboardId},
		VizType:             &chart.VizType.Value,
		Params:              &chart.Params.Value,
		IsManagedExternally: &isManagedExternally,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating chart",
			"Could not create chart, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error creating chart",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	result := &Chart{
		Id:          types.Int64{Value: int64(*res.JSON201.Id)},
		Title:       types.String{Value: res.JSON201.Result.SliceName},
		DatasetId:   chart.DatasetId,
		DashboardId: chart.DashboardId,
		VizType:     types.String{Value: *res.JSON201.Result.VizType},
		Params:      chart.Params,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceChart) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var chart Chart
	diags := req.State.Get(ctx, &chart)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.GetApiV1ChartPkWithResponse(ctx, int(chart.Id.Value), &client.GetApiV1ChartPkParams{})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating chart",
			"Could not update chart, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error creating chart",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	result := &Chart{
		Id:          types.Int64{Value: int64(*res.JSON200.Id)},
		Title:       types.String{Value: *res.JSON200.Result.SliceName},
		DatasetId:   chart.DatasetId,
		DashboardId: chart.DashboardId,
		VizType:     types.String{Value: *res.JSON200.Result.VizType},
		Params:      chart.Params,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceChart) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var chart Chart
	diags := req.Plan.Get(ctx, &chart)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state Chart
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isManagedExternally := true
	datasourceId := int32(chart.DatasetId.Value)
	dashboardId := int32(chart.DashboardId.Value)
	var datasourceType client.ChartRestApiPutDatasourceType = "table"
	res, err := r.p.client.PutApiV1ChartPkWithResponse(ctx, int(state.Id.Value), client.PutApiV1ChartPkJSONRequestBody{
		SliceName:           &chart.Title.Value,
		DatasourceId:        &datasourceId,
		DatasourceType:      &datasourceType,
		Dashboards:          &[]int32{dashboardId},
		VizType:             &chart.VizType.Value,
		Params:              &chart.Params.Value,
		IsManagedExternally: &isManagedExternally,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading chart",
			"Could not read chart, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating chart",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	result := &Chart{
		Id:          types.Int64{Value: int64(*res.JSON200.Id)},
		Title:       types.String{Value: *res.JSON200.Result.SliceName},
		DatasetId:   chart.DatasetId,
		DashboardId: chart.DashboardId,
		VizType:     types.String{Value: *res.JSON200.Result.VizType},
		Params:      chart.Params,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceChart) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Chart
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.DeleteApiV1ChartPkWithResponse(ctx, int(state.Id.Value))

	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting chart",
			"Could not delete chart, unexpected error: "+err.Error(),
		)
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error deleting chart",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}
}
