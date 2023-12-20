package provider

import (
	"context"
	"fmt"
	"io"

	authproto "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/spicedb/pkg/tuple"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RelationshipResource{}
var _ resource.ResourceWithImportState = &RelationshipResource{}

func NewRelationshipResource() resource.Resource {
	return &RelationshipResource{}
}

// RelationshipResource defines the resource implementation.
type RelationshipResource struct {
	client *authzed.Client
}

// RelationsipResourceModel describes the resource data model.
type RelationsipResourceModel struct {
	Relationship types.String `tfsdk:"relationship"`
}

func (r *RelationshipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_relationship"
}

func (r *RelationshipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example resource",

		Attributes: map[string]schema.Attribute{
			"relationship": schema.StringAttribute{
				MarkdownDescription: "A SpiceDB relationship in object#relation@subject format.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *RelationshipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*authzed.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *authzed.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func parseRelationship(s string) (*authproto.Relationship, error) {
	rel := tuple.ParseRel(s)
	if rel == nil {
		return nil, fmt.Errorf("invalid relationship string: %s", s)
	}

	return rel, nil
}

func (r *RelationshipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RelationsipResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rel, err := parseRelationship(data.Relationship.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Relationship", fmt.Sprintf("Unable to parse relationship: %s", err))
		return
	}

	_, err = r.client.WriteRelationships(ctx, &authproto.WriteRelationshipsRequest{
		Updates: []*authproto.RelationshipUpdate{
			{
				Operation:    authproto.RelationshipUpdate_OPERATION_TOUCH,
				Relationship: rel,
			},
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("SpiceDB Client Error", fmt.Sprintf("Unable to create relationships, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "Created SpiceDB relationships")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RelationshipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RelationsipResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rel, err := parseRelationship(data.Relationship.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Relationship", fmt.Sprintf("Unable to parse relationship: %s", err))
		return
	}

	found, err := r.hasRelationship(ctx, rel)
	if err != nil {
		resp.Diagnostics.AddError("Unable to get relationship information", fmt.Sprintf("Unable to get relationship: %s", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	tflog.Trace(ctx, "Read SpiceDB relationship")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RelationshipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r *RelationshipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RelationsipResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rel, err := parseRelationship(data.Relationship.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Relationship", fmt.Sprintf("Unable to parse relationship: %s", err))
		return
	}

	_, err = r.client.DeleteRelationships(ctx, &authproto.DeleteRelationshipsRequest{
		RelationshipFilter: r.relationshipFilter(rel),
	})

	if err != nil {
		resp.Diagnostics.AddError("SpiceDB Client Error", fmt.Sprintf("Unable to delete relationship, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "Deleted SpiceDB relationship")
}

func (r *RelationshipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("relationship"), req, resp)
}

func (r *RelationshipResource) relationshipFilter(rel *authproto.Relationship) *authproto.RelationshipFilter {
	filter := &authproto.RelationshipFilter{
		ResourceType:       rel.Resource.ObjectType,
		OptionalResourceId: rel.Resource.ObjectId,
		OptionalRelation:   rel.Relation,
		OptionalSubjectFilter: &authproto.SubjectFilter{
			SubjectType:       rel.Subject.Object.ObjectType,
			OptionalSubjectId: rel.Subject.Object.ObjectId,
		},
	}

	if rel.Subject.OptionalRelation != "" {
		filter.OptionalSubjectFilter.OptionalRelation = &authproto.SubjectFilter_RelationFilter{
			Relation: rel.Subject.OptionalRelation,
		}
	}

	return filter
}

func (r *RelationshipResource) hasRelationship(ctx context.Context, rel *authproto.Relationship) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := r.client.PermissionsServiceClient.ReadRelationships(ctx, &authproto.ReadRelationshipsRequest{
		Consistency: &authproto.Consistency{
			Requirement: &authproto.Consistency_FullyConsistent{
				FullyConsistent: true,
			},
		},
		RelationshipFilter: r.relationshipFilter(rel),
	})

	if err != nil {
		return false, err
	}

	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		if r != nil {
			return true, nil
		}
	}

	return false, nil
}
