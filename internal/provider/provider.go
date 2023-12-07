package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
)

// Ensure SpiceDBProvider satisfies various provider interfaces.
var _ provider.Provider = &SpiceDBProvider{}

// SpiceDBProvider defines the provider implementation.
type SpiceDBProvider struct {
	version string
}

type SpiceDBProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

func (p *SpiceDBProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "spicedb"
	resp.Version = p.version
}

func (p *SpiceDBProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "SpiceDB gRPC API endpoint",
				Required:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "SpiceDB API token",
				Required:            true,
			},
			"insecure": schema.BoolAttribute{
				MarkdownDescription: "Connect over a plaintext connection",
				Optional:            true,
			},
		},
	}
}

func (p *SpiceDBProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring SpiceDB client")

	var data SpiceDBProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Endpoint.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Endpoint is not provided",
			"SpiceDB Endpoint is required so that the provider knows how to talk to SpiceDB. "+
				"It is expected to be a GRPC url like `grpc.authzed.com:443`")
	}

	if data.Token.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Token is not provided",
			"SpiceDB API token is required so that the provider knows how to talk to SpiceDB. "+
				"It is expected to be a bearer token like `t_your_token_here_1234567deadbeef`")
	}

	opts := []grpc.DialOption{}

	if data.Insecure.ValueBool() {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		opts = append(opts, grpcutil.WithInsecureBearerToken(data.Token.ValueString()))
	} else {
		systemCerts, err := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
		if err != nil {
			resp.Diagnostics.AddError("SpiceDB Provider Error", fmt.Sprintf("Unable to use system certs, got error: %s", err))
			return
		}
		opts = append(opts, grpcutil.WithBearerToken(data.Token.ValueString()))
		opts = append(opts, systemCerts)
	}

	client, err := authzed.NewClient(
		data.Endpoint.ValueString(),
		opts...,
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure SpiceDB client", fmt.Sprintf("Unable to configure SpiceDB client, got error: %s", err))
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured SpiceDB client", map[string]any{"success": true})
}

func (p *SpiceDBProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSchemaResource,
		NewRelationshipResource,
	}
}

func (p *SpiceDBProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSchemaDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SpiceDBProvider{
			version: version,
		}
	}
}
