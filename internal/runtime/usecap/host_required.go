package usecap

import "reasonix/internal/contract/tool"

// HostRequiredBy says why the tool cannot be removed by name: it is the only way
// the model reaches a connected MCP server's deferred tools.
func (*UseCapabilityTool) HostRequiredBy() string { return tool.FeatureDeferredMCP }
