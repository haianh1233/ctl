package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/conduktor/ctl/cmd"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"Conduktor MCP Demo 🚀",
		"1.0.0",
	)

	getAllTool := mcp.NewTool("get_all", mcp.WithDescription("Get all resources from Conduktor"))
	getTopicTool := mcp.NewTool("get_topic", mcp.WithDescription("Get all topics from Conduktor"),
		mcp.WithString("cluster",
			mcp.Required(),
			mcp.Description("Name of the cluster to get topics from")),
	)

	s.AddTool(getAllTool, getAllHandler)
	s.AddTool(getTopicTool, getTopicHandler)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func getAllHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := cmd.ExecuteCapture("get", "all")
	if err != nil {
		return nil, errors.New("Command failed with error: " + err.Error())
	}

	return mcp.NewToolResultText(output), nil
}

func getTopicHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cluster, ok := request.Params.Arguments["cluster"].(string)

	if !ok {
		return nil, errors.New("cluster must be a string")
	}

	output, err := cmd.ExecuteCapture("get", "topic", "--cluster", cluster)
	if err != nil {
		return nil, errors.New("Command failed with error: " + err.Error())
	}

	return mcp.NewToolResultText(output), nil
}

//func main() {
//	os.Setenv("CDK_USER", "admin@demo.com")
//	os.Setenv("CDK_PASSWORD", "admin")
//	os.Setenv("CDK_BASE_URL", "http://localhost:8080")
//
//	fmt.Println("CDK_USER =", os.Getenv("CDK_USER"))
//	fmt.Println("CDK_PASSWORD =", os.Getenv("CDK_PASSWORD"))
//	fmt.Println("CDK_BASE_URL =", os.Getenv("CDK_BASE_URL"))
//
//	output, err := cmd.ExecuteCapture("get", "all")
//	if err != nil {
//		fmt.Println("Command failed with error:", err.Error())
//		return
//	}
//
//	fmt.Println("Result:", output)
//}
