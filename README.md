# Terraform Provider Scaffolding (Terraform Plugin Framework)

_This template repository is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework). The template repository built on the [Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) can be found at [terraform-provider-scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding). See [Which SDK Should I Use?](https://developer.hashicorp.com/terraform/plugin/framework-benefits) in the Terraform documentation for additional information._

This repository is a *template* for a [Terraform](https://www.terraform.io) provider. It is intended as a starting point for creating Terraform providers, containing:

- A resource and a data source (`internal/provider/`),
- Examples (`examples/`) and generated documentation (`docs/`),
- Miscellaneous meta files.

These files contain boilerplate code that you will need to edit to create your own Terraform provider. Tutorials for creating Terraform providers can be found on the [HashiCorp Developer](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework) platform. _Terraform Plugin Framework specific guides are titled accordingly._

Please see the [GitHub template repository documentation](https://help.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-from-a-template) for how to create a new repository from this template on GitHub.

Once you've written your provider, you'll want to [publish it on the Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing) so that others can use it.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

Fill this in for each provider

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

### Running Integration Tests

This provider includes integration tests that test against a real Flintlock server. The tests are organized into two categories:

1. **Unit Tests** - Test the provider logic without requiring Flintlock
2. **Integration Tests** - Test against a real Flintlock server

#### Running Tests with nektos/act

The GitHub Actions workflows can be tested locally using [nektos/act](https://github.com/nektos/act).

**Prerequisites for full integration tests:**
- Docker installed and running
- `containerd` installed and running
- Network bridge `br0` created

```bash
# Create the br0 bridge (required for Flintlock)
sudo ip link add br0 type bridge
sudo ip addr add 10.0.0.1/24 dev br0
sudo ip link set br0 up

# Start containerd (required for Flintlock VM creation)
sudo containerd &

# Run unit tests (no special requirements)
act -j unit-tests

# Run integration tests (requires br0 bridge and containerd)
act -j integration-tests

# Run all jobs
act
```

#### Running Tests Manually

```bash
# Run unit tests only
TF_ACC=1 go test -v -timeout 5m ./internal/provider/ -run TestAccVMsDataSource -tags='!integration'

# Run integration tests (connectivity only, no containerd required)
TF_ACC=1 go test -v -timeout 5m ./internal/provider/ -run TestIntegration_FlintlockConnectivity -tags=integration

# Run full integration tests (requires containerd)
TF_ACC=1 CONTAINERD_REQUIRED=1 go test -v -timeout 10m ./internal/provider/ -run TestIntegration_VMsDataSource_WithVMs -tags=integration

# Skip all integration tests
TF_ACC_SKIP_INTEGRATION=1 go test -v ./internal/provider/
```

#### Test Descriptions

| Test | Description | Requirements |
|------|-------------|--------------|
| `TestAccVMsDataSource` | Unit test for VMs data source | None |
| `TestIntegration_FlintlockConnectivity` | Tests provider can connect to Flintlock | `br0` bridge |
| `TestIntegration_VMsDataSource_WithVMs` | Full test with VM creation | `br0` bridge, `containerd` |
