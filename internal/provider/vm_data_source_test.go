package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)



func TestAccVMsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
PreCheck:                 func() {},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: `
provider "flintlock" {
  endpoint  = "localhost:9090"
  authtoken = "dummy"
}

data "flintlock_vms" "test" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
// Initial check will probably return 0 items because the test server is empty.
resource.TestCheckResourceAttr("data.flintlock_vms.test", "vms.#", "0"),
),
			},
		},
	})
}
