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

type Dashboard struct {
	Id     types.Int64  `tfsdk:"id"`
	Title  types.String `tfsdk:"title"`
	Slug   types.String `tfsdk:"slug"`
	Status types.String `tfsdk:"status"`
}

type resourceDashboardType struct{}

func (r resourceDashboardType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
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
			"slug": {
				Required:      true,
				Type:          types.StringType,
				PlanModifiers: tfsdk.AttributePlanModifiers{resource.RequiresReplace()},
			},
			"status": {
				Required: true,
				Type:     types.StringType,
				Validators: []tfsdk.AttributeValidator{
					stringOneOf(
						"draft",
						"published",
					),
				},
			},
		},
	}, nil
}

func (r resourceDashboardType) NewResource(_ context.Context, p provider.Provider) (resource.Resource, diag.Diagnostics) {
	return resourceDashboard{
		p: *p.(*presetProvider),
	}, nil
}

type resourceDashboard struct {
	p presetProvider
}

func (r resourceDashboard) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var dashboard Dashboard
	diags := req.Plan.Get(ctx, &dashboard)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isManagedExternally := true
	isPublished := dashboard.Status.Value == "published"
	res, err := r.p.client.PostApiV1DashboardWithResponse(ctx, client.PostApiV1DashboardJSONRequestBody{
		DashboardTitle:      &dashboard.Title.Value,
		Slug:                &dashboard.Slug.Value,
		IsManagedExternally: &isManagedExternally,
		Published:           &isPublished,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating dashboard",
			"Could not create dashboard, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error creating dashboard",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	var status string

	if *res.JSON201.Result.Published {
		status = "published"
	} else {
		status = "draft"
	}

	result := &Dashboard{
		Id:     types.Int64{Value: int64(*res.JSON201.Id)},
		Title:  types.String{Value: *res.JSON201.Result.DashboardTitle},
		Slug:   types.String{Value: *res.JSON201.Result.Slug},
		Status: types.String{Value: status},
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDashboard) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Dashboard
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.GetApiV1DashboardIdOrSlugWithResponse(ctx, fmt.Sprint(state.Id.Value))

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading dashboard",
			"Could not read dashboard, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading dashboard",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	var status string

	if *res.JSON200.Result.Published {
		status = "published"
	} else {
		status = "draft"
	}

	result := &Dashboard{
		Id:     types.Int64{Value: int64(*res.JSON200.Result.Id)},
		Title:  types.String{Value: *res.JSON200.Result.DashboardTitle},
		Slug:   types.String{Value: *res.JSON200.Result.Slug},
		Status: types.String{Value: status},
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDashboard) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var dashboard Dashboard
	diags := req.Plan.Get(ctx, &dashboard)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state Dashboard
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isManagedExternally := true
	isPublished := dashboard.Status.Value == "published"
	res, err := r.p.client.PutApiV1DashboardPkWithResponse(ctx, int(state.Id.Value), client.PutApiV1DashboardPkJSONRequestBody{
		DashboardTitle:      &dashboard.Title.Value,
		Slug:                &dashboard.Slug.Value,
		IsManagedExternally: &isManagedExternally,
		Published:           &isPublished,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating dashboard",
			"Could not update dashboard, unexpected error: "+err.Error(),
		)

		return
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating dashboard",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}

	var status string

	if *res.JSON200.Result.Published {
		status = "published"
	} else {
		status = "draft"
	}

	result := &Dashboard{
		Id:     types.Int64{Value: int64(*res.JSON200.Id)},
		Title:  types.String{Value: *res.JSON200.Result.DashboardTitle},
		Slug:   types.String{Value: *res.JSON200.Result.Slug},
		Status: types.String{Value: status},
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDashboard) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Dashboard
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.p.client.DeleteApiV1DashboardPkWithResponse(ctx, int(state.Id.Value))

	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting dashboard",
			"Could not delete dashboard, unexpected error: "+err.Error(),
		)
	}

	if res.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error deleting dashboard",
			fmt.Sprintf("%v response returned: %v", res.StatusCode(), string(res.Body)),
		)

		return
	}
}
