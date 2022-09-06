# `terraform-provider-preset` **Beta**

A Terraform provider for [Preset.io](https://preset.io).

### Development

#### Building

To build the client, run `make build`. This will output a binary in the root directory called `terraform-provider-superset`.

#### Running

Once the client is build, you can use the provider by updating your `~/.terraformrc` configuration:

```tf
provider_installation {
  dev_overrides {
    "vercel/preset" = "<path>/terraform-provider-preset"
  }
  direct {}
}%
```

**Note: The full path targets this repository folder, not the built binary!**

Now your provider will be configured to use the local binary in your Terraform code.

#### Building the Superset API Client (`client/client.gen.go`)

`terraform-provider-preset` uses [deepmap/oapi-codegen](https://github.com/deepmap/oapi-codegen) to generate a Go API Client from Superset's Open API specification. The Superset Open API specification lives in `client/superset_openapi.json` and is _manually_ modified to adapt it to `oapi-codegen` and fix places where it is incorrect. If you make changes to the spec, you can regenerate the API client by installing `oapi-codegen` and running `make client`. **Do not modify `client.gen.go` directly because it will be overwritten at some point in the future.** If you need to extend it with custom logic, add it to `client.go`.

Notes:

- The Superset API does not return an RFC3339/ISO8601 timestamps as advertised and Go will complain when it attempts to parse them. To avoid this, we've added a `SupersetTime` type that correctly parses the timestamp which can be set in the Superset API specififcation using the `"x-go-type": "SupersetTime"` attribute. For example:

```
{
  "components": {
    "schemas": {
      "DashboardGetResponseSchema": {
        "properties": {
          "changed_on": {
            "format": "date-time",
            "type": "string",
            "x-go-type": "SupersetTime"
          },
        }
      }
    }
  }
}
```

- Setting `"nullable": true` on a schema in the Open API specification means that the API Client will send the value as `null` if it does not exist. This is bad for `PUT` requests which intend to update on some data on a model because instead of just omitting the property, we will unset it. Remove the `"nullable": true` from the specification to omit the property from the request.
