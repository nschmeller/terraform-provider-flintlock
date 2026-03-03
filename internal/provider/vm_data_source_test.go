package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVMsDataSource(t *testing.T) {
	endpoint := os.Getenv("FLINTLOCK_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9090"
	}

	authToken := os.Getenv("FLINTLOCK_AUTHTOKEN")
	if authToken == "" {
		authToken = "dummy"
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing - empty state
			{
				Config: testAccVMsDataSourceConfig(endpoint, authToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flintlock_vms.test", "vms.#", "0"),
				),
			},
		},
	})
}

func testAccVMsDataSourceConfig(endpoint, authToken string) string {
	return `
provider "flintlock" {
  endpoint  = "` + endpoint + `"
  authtoken = "` + authToken + `"
}

data "flintlock_vms" "test" {}
`
}
