package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	spiceSchema = `definition user {
		relation self: user
	}`
)

func TestAccRelationshipResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + testAccSchemaResourceConfig(spiceSchema) + testAccRelationship("user:user-1#self@user:user-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("spicedb_relationship.test", "relationship", "user:user-1#self@user:user-1"),
				),
			},
			// Update and Read testing
			{
				Config: providerConfig + testAccSchemaResourceConfig(spiceSchema) + testAccRelationship("user:user-2#self@user:user-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("spicedb_relationship.test", "relationship", "user:user-2#self@user:user-2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccRelationship(relationship string) string {
	return fmt.Sprintf(`
resource "spicedb_relationship" "test" {
  relationship = %[1]q
  depends_on = [spicedb_schema.test]
}`, relationship)
}
