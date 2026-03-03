// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	testAccProviderEndpointEnv  = "FLINTLOCK_ENDPOINT"
	testAccProviderAuthtokenEnv = "FLINTLOCK_AUTHTOKEN"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"flintlock": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// Verify that the Flintlock endpoint is reachable
	endpoint := os.Getenv(testAccProviderEndpointEnv)
	if endpoint == "" {
		endpoint = "localhost:9090"
	}

	// Parse endpoint to get host and port
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		// If endpoint doesn't have port, assume default
		host = endpoint
		port = "9090"
	}

	// Check if the endpoint is reachable
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 5*time.Second)
	if err != nil {
		t.Logf("Warning: Flintlock endpoint at %s is not reachable: %v", endpoint, err)
		t.Logf("This is expected when running tests without a Flintlock server")
	} else {
		t.Logf("Flintlock endpoint at %s is reachable", endpoint)
		conn.Close()
	}

	// Log authentication token status
	authToken := os.Getenv(testAccProviderAuthtokenEnv)
	if authToken == "" {
		t.Log("FLINTLOCK_AUTHTOKEN not set, using default test token")
	} else {
		t.Log("FLINTLOCK_AUTHTOKEN is set")
	}
}
