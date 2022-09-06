package preset

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sync"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vercel/terraform-provider-preset/client"
)

type DashboardFilter struct {
	Id          types.String `tfsdk:"id"`
	DashboardId types.Int64  `tfsdk:"dashboard_id"`
	Name        types.String `tfsdk:"name"`
	Config      types.String `tfsdk:"config"`
}

// we can only create a single dashboard filter at any one time because filters may
// update the same dashboard at any one time which can cause a race condition
var resourceDashboardFilterMutex sync.Mutex

type resourceDashboardFilterType struct{}

func (r resourceDashboardFilterType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:      true,
				Type:          types.StringType,
				PlanModifiers: tfsdk.AttributePlanModifiers{resource.UseStateForUnknown()},
			},
			"dashboard_id": {
				Required: true,
				Type:     types.Int64Type,
			},
			"name": {
				Required: true,
				Type:     types.StringType,
			},
			"config": {
				Required: true,
				Type:     types.StringType,
			},
		},
	}, nil
}

func (r resourceDashboardFilterType) NewResource(_ context.Context, p provider.Provider) (resource.Resource, diag.Diagnostics) {
	return resourceDashboardFilter{
		p: *p.(*presetProvider),
	}, nil
}

type resourceDashboardFilter struct {
	p presetProvider
}

func (r resourceDashboardFilter) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var dashboardfilter DashboardFilter
	diags := req.Plan.Get(ctx, &dashboardfilter)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := fmt.Sprint(rand.Intn(1000000) + 100000)
	err := upsertDashboardFilter(ctx, r.p.client, dashboardfilter.DashboardId.Value, id, &dashboardfilter)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating dashboard filter",
			"Could not create dashboard filter, unexpected error: "+err.Error(),
		)

		return
	}

	diags = resp.State.Set(ctx, dashboardfilter)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDashboardFilter) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardFilter
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	filter, err := getDashboardFilterById(ctx, r.p.client, state.DashboardId.Value, state.Id.Value)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading dashboardfilter",
			"Could not read dashboardfilter, unexpected error: "+err.Error(),
		)

		return
	}

	if filter == nil {
		resp.Diagnostics.AddError(
			"Error reading dashboard filter",
			"Could not find dashboard filter with id: "+state.Id.Value,
		)

		return
	}

	eq, err := isConfigEqual(state.Config.Value, filter.Config.Value)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading dashboard filter",
			"Could not find dashboard filter with id: "+err.Error(),
		)

		return
	}

	if eq {
		filter.Config = state.Config
	}

	diags = resp.State.Set(ctx, filter)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceDashboardFilter) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var dashboardfilter DashboardFilter
	diags := req.Plan.Get(ctx, &dashboardfilter)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DashboardFilter
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := upsertDashboardFilter(ctx, r.p.client, dashboardfilter.DashboardId.Value, state.Id.Value, &dashboardfilter)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating dashboardfilter",
			"Could not update dashboardfilter, unexpected error: "+err.Error(),
		)

		return
	}

	diags = resp.State.Set(ctx, dashboardfilter)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes an Alias.
func (r resourceDashboardFilter) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DashboardFilter
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := upsertDashboardFilter(ctx, r.p.client, state.DashboardId.Value, state.Id.Value, nil)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating dashboardfilter",
			"Could not update dashboardfilter, unexpected error: "+err.Error(),
		)

		return
	}
}

type jsonMetadata map[string]interface{}

func upsertDashboardFilter(ctx context.Context, c *client.ClientWithResponses, dashboardId int64, filterId string, filter *DashboardFilter) error {
	resourceDashboardFilterMutex.Lock()
	defer resourceDashboardFilterMutex.Unlock()

	filters, jm, err := getDashboardFilters(ctx, c, dashboardId)

	if err != nil {
		return err
	}

	existingFilter := findDashboardFilterById(filters, filterId)

	if existingFilter != nil {
		filters = remove(filters, existingFilter)
	}

	if filter != nil {
		filter.Id = types.String{Value: filterId}

		filters = append(filters, filter)
	}

	jsonMetadata := gabs.New()

	if jm != nil {
		err = jsonMetadata.Merge(jm)

		if err != nil {
			return err
		}
	}

	jsonMetadata.ArrayOfSize(len(filters), "native_filter_configuration")

	for i, v := range filters {
		config, err := gabs.ParseJSON([]byte(v.Config.Value))

		if err != nil {
			return err
		}

		config.Set(v.Id.Value, "id")
		jsonMetadata.S("native_filter_configuration").SetIndex(config, i)
	}

	serializedJsonMetadataString := jsonMetadata.String()

	res, err := c.PutApiV1DashboardPkWithResponse(ctx, int(dashboardId), client.PutApiV1DashboardPkJSONRequestBody{
		JsonMetadata: &serializedJsonMetadataString,
	})

	if err != nil {
		return err
	}

	if res.StatusCode() != 200 {
		return fmt.Errorf("Unable to upsert filter: %v response returned: %v", res.StatusCode(), string(res.Body))
	}

	return nil
}

func getDashboardJsonMetadata(ctx context.Context, client *client.ClientWithResponses, dashboardId int64) (*gabs.Container, error) {
	res, err := client.GetApiV1DashboardIdOrSlugWithResponse(ctx, fmt.Sprint(dashboardId))

	if err != nil {
		return nil, err
	}

	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("%v response returned: %v", res.StatusCode(), string(res.Body))
	}

	if res.JSON200.Result.JsonMetadata == nil {
		return nil, nil
	}

	return gabs.ParseJSON([]byte(*res.JSON200.Result.JsonMetadata))
}

func getDashboardFilters(ctx context.Context, client *client.ClientWithResponses, dashboardId int64) ([]*DashboardFilter, *gabs.Container, error) {
	jm, err := getDashboardJsonMetadata(ctx, client, dashboardId)

	if err != nil {
		return nil, nil, err
	}

	if jm == nil {
		return []*DashboardFilter{}, nil, nil
	}

	var filters []*DashboardFilter

	for _, v := range jm.Path("native_filter_configuration").Children() {
		id, ok := v.Path("id").Data().(string)

		if !ok {
			return nil, nil, fmt.Errorf("Unable to parse id")
		}

		name, ok := v.Path("name").Data().(string)

		if !ok {
			return nil, nil, fmt.Errorf("Unable to parse name")
		}

		err = v.Delete("id")

		if err != nil {
			return nil, nil, err
		}

		filters = append(filters, &DashboardFilter{
			Id:          types.String{Value: id},
			DashboardId: types.Int64{Value: dashboardId},
			Name:        types.String{Value: name},
			Config:      types.String{Value: string(v.String())},
		})
	}

	return filters, jm, nil
}

func getDashboardFilterById(ctx context.Context, client *client.ClientWithResponses, dashboardId int64, filterId string) (*DashboardFilter, error) {
	filters, _, err := getDashboardFilters(ctx, client, dashboardId)

	if err != nil {
		return nil, err
	}

	return findDashboardFilterById(filters, filterId), nil
}

func findDashboardFilterById(filters []*DashboardFilter, filterId string) *DashboardFilter {
	for _, v := range filters {
		if v.Id.Value == filterId {
			return v
		}
	}

	return nil
}

func remove(l []*DashboardFilter, item *DashboardFilter) []*DashboardFilter {
	for i, other := range l {
		if other.Id.Value == item.Id.Value {
			return append(l[:i], l[i+1:]...)
		}
	}

	return l
}

func mergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func isConfigEqual(configA string, configB string) (bool, error) {
	var ca map[string]interface{}
	err := json.Unmarshal([]byte(configA), &ca)

	if err != nil {
		return false, err
	}

	var cb map[string]interface{}
	err = json.Unmarshal([]byte(configB), &cb)

	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(ca, cb), nil
}
