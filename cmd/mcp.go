package cmd

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server",
	Long:  `Start the Conduktor MCP server for integration with AI assistants.`,
	Run: func(cmd *cobra.Command, args []string) {
		startMCPServer()
	},
}

func startMCPServer() {
	s := server.NewMCPServer(
		"Conduktor MCP Server",
		"1.0.0",
	)

	getAllTool := mcp.NewTool("get_all", mcp.WithDescription("Get all resources from Conduktor"))
	getTopicTool := mcp.NewTool("get_topic", mcp.WithDescription("Get all topics from Conduktor"),
		mcp.WithString("name", mcp.Description("Name of the topic to get")),
		mcp.WithString("cluster",
			mcp.Required(),
			mcp.Description("Name of the cluster to get topics from")),
	)
	getUserTool := mcp.NewTool("get_user", mcp.WithDescription("Get all users from Conduktor"),
		mcp.WithString("name", mcp.Description("Name of the user to get")),
	)
	getInterceptorTool := mcp.NewTool("get_interceptor", mcp.WithDescription("Get all interceptors from Conduktor"),
		mcp.WithString("name", mcp.Description("Name of the interceptor to get")),
	)
	getVirtualClusterTool := mcp.NewTool("get_virtual_cluster", mcp.WithDescription("Get all virtual clusters from Conduktor"),
		mcp.WithString("name", mcp.Description("Name of the virtual cluster to get")),
	)
	getGroupTool := mcp.NewTool("get_group", mcp.WithDescription("Get all groups from Conduktor"),
		mcp.WithString("name", mcp.Description("Name of the group to get")),
	)

	s.AddTool(getAllTool, getAllHandler)
	s.AddTool(getTopicTool, getTopicHandler)
	s.AddTool(getUserTool, getUserHandler)
	s.AddTool(getInterceptorTool, getInterceptorHandler)
	s.AddTool(getVirtualClusterTool, getVirtualClusterHandler)
	s.AddTool(getGroupTool, getGroupHandler)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func getAllHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonResult, err := get("all", "json", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(jsonResult), nil
}

func getTopicHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cluster, ok := request.Params.Arguments["cluster"].(string)

	if !ok {
		return nil, fmt.Errorf("cluster must be a string")
	}

	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	jsonResult, err := get("Topic", "json", map[string]string{"cluster": cluster}, []string{name})
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(jsonResult), nil
}

func getUserHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	jsonResult, err := get("User", "json", nil, []string{name})
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(jsonResult), nil
}

func getInterceptorHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	jsonResult, err := get("Interceptor", "json", map[string]string{"name": name}, nil)
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(jsonResult), nil
}

func getVirtualClusterHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	jsonResult, err := get("VirtualCluster", "json", nil, []string{name})
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(jsonResult), nil
}

func getGroupHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	jsonResult, err := get("Group", "json", nil, []string{name})
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(jsonResult), nil
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
