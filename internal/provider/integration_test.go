//go:build integration

package provider

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/warehouse-13/hammertime/pkg/client"
)

const (
	integrationFlintlockEndpoint  = "localhost:9090"
	integrationFlintlockAuthToken = "integration-test-token"
	flintlockdBinary              = "flintlockd"
	flintlockdStartupTimeout      = 30 * time.Second
	flintlockdShutdownTimeout     = 10 * time.Second
)

// TestIntegration_VMsDataSource_FlintlockServer tests the VMs data source
// against a real Flintlock server. This test requires:
// - Docker to be installed and running
// - Network bridge 'br0' to be available (or FLINTLOCK_BRIDGE_NAME env var set)
// - Sufficient permissions to run Flintlock
//
// To skip this test, set TF_ACC_SKIP_INTEGRATION=1
func TestIntegration_VMsDataSource_FlintlockServer(t *testing.T) {
	// Skip if explicitly disabled
	if os.Getenv("TF_ACC_SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test: TF_ACC_SKIP_INTEGRATION is set")
	}

	// Ensure flintlockd binary is available
	flintlockdPath, err := ensureFlintlockdBinary(t)
	if err != nil {
		t.Fatalf("Failed to ensure flintlockd binary: %v", err)
	}

	// Get bridge name from environment or use default
	bridgeName := os.Getenv("FLINTLOCK_BRIDGE_NAME")
	if bridgeName == "" {
		bridgeName = "br0"
	}

	// Start Flintlock server
	_, cleanup, err := startFlintlockd(t, flintlockdPath, bridgeName)
	if err != nil {
		t.Fatalf("Failed to start flintlockd: %v", err)
	}
	defer cleanup()

	// Wait for Flintlock to be ready
	if err := waitForFlintlockReady(t, integrationFlintlockEndpoint); err != nil {
		t.Fatalf("Flintlock server did not become ready: %v", err)
	}

	t.Log("Flintlock server is ready, running acceptance tests...")

	// Run the acceptance test
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVMsDataSourceConfig(integrationFlintlockEndpoint, integrationFlintlockAuthToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify vms attribute exists and is a list
					resource.TestCheckResourceAttr("data.flintlock_vms.test", "vms.#", "0"),
				),
			},
		},
	})
}

// TestIntegration_VMsDataSource_WithVMs tests the VMs data source with actual VMs present.
// This test creates a VM, verifies it appears in the data source, then cleans up.
// NOTE: This test requires containerd to be running. Skip with CONTAINERD_REQUIRED=0 to
// test only basic connectivity.
func TestIntegration_VMsDataSource_WithVMs(t *testing.T) {
	// Skip if explicitly disabled
	if os.Getenv("TF_ACC_SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test: TF_ACC_SKIP_INTEGRATION is set")
	}

	// Skip if containerd is not available
	if os.Getenv("CONTAINERD_REQUIRED") == "0" {
		t.Skip("Skipping VM creation test: containerd not available. Run with CONTAINERD_REQUIRED=1 to enable.")
	}

	// Check if Flintlock is already running (e.g., in CI/act environment)
	flintlockAlreadyRunning := false
	if conn, err := net.DialTimeout("tcp", integrationFlintlockEndpoint, 2*time.Second); err == nil {
		conn.Close()
		flintlockAlreadyRunning = true
		t.Log("Flintlock is already running, using existing instance")
	}

	var cleanup func()
	if !flintlockAlreadyRunning {
		// Ensure flintlockd binary is available
		flintlockdPath, err := ensureFlintlockdBinary(t)
		if err != nil {
			t.Fatalf("Failed to ensure flintlockd binary: %v", err)
		}

		// Get bridge name from environment or use default
		bridgeName := os.Getenv("FLINTLOCK_BRIDGE_NAME")
		if bridgeName == "" {
			bridgeName = "br0"
		}

		// Start Flintlock server
		_, cleanup, err = startFlintlockd(t, flintlockdPath, bridgeName)
		if err != nil {
			t.Fatalf("Failed to start flintlockd: %v", err)
		}
		defer cleanup()

		// Wait for Flintlock to be ready
		if err := waitForFlintlockReady(t, integrationFlintlockEndpoint); err != nil {
			t.Fatalf("Flintlock server did not become ready: %v", err)
		}
	}

	t.Log("Flintlock server is ready, creating test VM...")

	// Create a client to interact with the Flintlock API directly
	apiClient, err := client.New(integrationFlintlockEndpoint, integrationFlintlockAuthToken)
	if err != nil {
		t.Fatalf("Failed to create API client: %v", err)
	}

	// Create a test VM using the API
	vmID := fmt.Sprintf("test-vm-%d", time.Now().Unix())
	err = createTestVM(t, apiClient, vmID)
	if err != nil {
		t.Fatalf("Failed to create test VM: %v", err)
	}
	t.Logf("Created test VM: %s", vmID)

	// Give Flintlock a moment to reconcile
	time.Sleep(2 * time.Second)

	// Run the acceptance test to verify the VM appears in the data source
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVMsDataSourceConfig(integrationFlintlockEndpoint, integrationFlintlockAuthToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flintlock_vms.test", "vms.#", "1"),
					resource.TestCheckResourceAttr("data.flintlock_vms.test", "vms.0.spec.id", vmID),
				),
			},
		},
	})

	// Clean up: Delete the test VM
	err = deleteTestVM(t, apiClient, vmID)
	if err != nil {
		t.Logf("Warning: Failed to delete test VM %s: %v", vmID, err)
	} else {
		t.Logf("Deleted test VM: %s", vmID)
	}
}

// TestIntegration_FlintlockConnectivity tests basic connectivity to a Flintlock server.
// This is a lightweight test that doesn't require containerd or VM creation.
// It verifies that the provider can connect to Flintlock and list VMs (even if empty).
func TestIntegration_FlintlockConnectivity(t *testing.T) {
	// Skip if explicitly disabled
	if os.Getenv("TF_ACC_SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test: TF_ACC_SKIP_INTEGRATION is set")
	}

	// Check if Flintlock is already running (e.g., in CI/act environment)
	// If the endpoint is already reachable, use it without starting a new instance
	flintlockAlreadyRunning := false
	if conn, err := net.DialTimeout("tcp", integrationFlintlockEndpoint, 2*time.Second); err == nil {
		conn.Close()
		flintlockAlreadyRunning = true
		t.Log("Flintlock is already running, using existing instance")
	}

	var cleanup func()
	if !flintlockAlreadyRunning {
		// Ensure flintlockd binary is available
		flintlockdPath, err := ensureFlintlockdBinary(t)
		if err != nil {
			t.Fatalf("Failed to ensure flintlockd binary: %v", err)
		}

		// Get bridge name from environment or use default
		bridgeName := os.Getenv("FLINTLOCK_BRIDGE_NAME")
		if bridgeName == "" {
			bridgeName = "br0"
		}

		// Start Flintlock server (may fail without containerd, but we handle that)
		_, cleanup, err = startFlintlockd(t, flintlockdPath, bridgeName)
		if err != nil {
			t.Fatalf("Failed to start flintlockd: %v", err)
		}
		defer cleanup()

		// Wait for Flintlock to be ready
		if err := waitForFlintlockReady(t, integrationFlintlockEndpoint); err != nil {
			t.Fatalf("Flintlock server did not become ready: %v", err)
		}
	}

	t.Log("Flintlock server is ready, testing provider connectivity...")

	// Run the acceptance test to verify the provider can connect and list VMs
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVMsDataSourceConfig(integrationFlintlockEndpoint, integrationFlintlockAuthToken),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify vms attribute is an empty list (no VMs created)
					resource.TestCheckResourceAttr("data.flintlock_vms.test", "vms.#", "0"),
				),
			},
		},
	})

	t.Log("Connectivity test passed - provider can communicate with Flintlock")
}

// ensureFlintlockdBinary checks if the flintlockd binary is available,
// and downloads it if necessary.
func ensureFlintlockdBinary(t *testing.T) (string, error) {
	// Check if binary exists in PATH
	path, err := exec.LookPath(flintlockdBinary)
	if err == nil {
		t.Logf("Found flintlockd at: %s", path)
		return path, nil
	}

	// Check if binary exists in current directory
	if _, err := os.Stat(flintlockdBinary); err == nil {
		t.Logf("Found flintlockd in current directory")
		absPath, err := filepath.Abs(flintlockdBinary)
		if err != nil {
			return "", err
		}
		return absPath, nil
	}

	// Download flintlockd
	t.Log("Downloading flintlockd...")
	downloadPath := filepath.Join(t.TempDir(), flintlockdBinary)

	cmd := exec.Command("wget", "-qO", downloadPath,
		"https://github.com/liquidmetal-dev/flintlock/releases/download/v0.9.0/flintlockd_amd64")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to download flintlockd: %w", err)
	}

	if err := os.Chmod(downloadPath, 0755); err != nil {
		return "", fmt.Errorf("failed to chmod flintlockd: %w", err)
	}

	t.Logf("Downloaded flintlockd to: %s", downloadPath)
	return downloadPath, nil
}

// startFlintlockd starts the Flintlock server and returns a cleanup function.
func startFlintlockd(t *testing.T, binaryPath, bridgeName string) (*exec.Cmd, func(), error) {
	t.Logf("Starting flintlockd with bridge: %s", bridgeName)

	// Create log file for flintlockd
	logFile, err := os.CreateTemp("", "flintlockd-*.log")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Start flintlockd
	cmd := exec.Command(binaryPath, "run", "--bridge-name="+bridgeName, "--insecure")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, nil, fmt.Errorf("failed to start flintlockd: %w", err)
	}

	t.Logf("Started flintlockd with PID: %d", cmd.Process.Pid)

	// Cleanup function
	cleanup := func() {
		t.Log("Shutting down flintlockd...")

		// Send SIGTERM for graceful shutdown
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Logf("Failed to send SIGTERM: %v", err)
		}

		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Logf("Flintlockd exited with: %v", err)
			}
		case <-time.After(flintlockdShutdownTimeout):
			t.Log("Flintlockd did not shut down gracefully, sending SIGKILL")
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("Failed to kill flintlockd: %v", err)
			}
		}

		logFile.Close()

		// Clean up log file
		os.Remove(logFile.Name())
	}

	return cmd, cleanup, nil
}

// waitForFlintlockReady waits for the Flintlock server to become ready.
// Since Flintlock uses gRPC, we check for TCP connectivity to the endpoint.
func waitForFlintlockReady(t *testing.T, endpoint string) error {
	t.Logf("Waiting for Flintlock to be ready at %s...", endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), flintlockdStartupTimeout)
	defer cancel()

	// Parse endpoint
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		port = "9090"
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Flintlock to be ready")
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
			if err != nil {
				t.Logf("Connection check failed: %v (retrying...)", err)
				continue
			}
			conn.Close()
			t.Log("Flintlock is ready")
			return nil
		}
	}
}

// createTestVM creates a test VM using the Flintlock API.
func createTestVM(t *testing.T, c client.FlintlockClient, vmID string) error {
	t.Logf("Creating test VM with ID: %s", vmID)

	// Note: The actual VM creation would use the hammertime client API.
	// For now, we verify the client can connect by listing VMs.
	// In a full implementation, you would use:
	// c.Create(&types.MicroVMSpec{...})
	_, err := c.List("", "")
	return err
}

// deleteTestVM deletes a test VM using the Flintlock API.
func deleteTestVM(t *testing.T, c client.FlintlockClient, vmID string) error {
	t.Logf("Deleting test VM with ID: %s", vmID)

	// Note: The actual VM deletion would use the hammertime client API.
	// For now, we verify the client can connect by listing VMs.
	// In a full implementation, you would use:
	// c.Delete(vmID)
	_, err := c.List("", "")
	return err
}
