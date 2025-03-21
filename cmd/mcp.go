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
		mcp.WithString("cluster",
			mcp.Required(),
			mcp.Description("Name of the cluster to get topics from")),
	)
	getUserTool := mcp.NewTool("get_user", mcp.WithDescription("Get all users from Conduktor"))

	s.AddTool(getAllTool, getAllHandler)
	s.AddTool(getTopicTool, getTopicHandler)
	s.AddTool(getUserTool, getUserHandler)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func getAllHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := ExecuteCapture("get", "all")
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(output), nil
}

func getTopicHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cluster, ok := request.Params.Arguments["cluster"].(string)

	if !ok {
		return nil, fmt.Errorf("cluster must be a string")
	}

	output, err := ExecuteCapture("get", "topic", "--cluster", cluster)
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(output), nil
}

func getUserHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := ExecuteCapture("get", "user")
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %w", err)
	}

	return mcp.NewToolResultText(output), nil
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
