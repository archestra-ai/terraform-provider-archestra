package provider

import (
	"testing"
)

func TestAccMCPServerToolDataSource(t *testing.T) {
	// Skip for now - MCP server tools are not discoverable until server is actually running
	// The data source schema expects tools to be available in the registry but filesystem
	// MCP server tools aren't registered until the server instance is started and discovers them.
	// This requires either:
	// 1. A pre-seeded MCP server with known tools
	// 2. Waiting for MCP server to start and discover its tools
	// 3. Using a different approach to test this data source
	t.Skip("Skipping MCP server tool data source tests - tools not available until server is running")
}
