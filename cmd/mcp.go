package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/conduktor/ctl/client"
	"github.com/conduktor/ctl/resource"
	"github.com/conduktor/ctl/schema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"os"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server",
	Long:  `Start the Conduktor MCP server for integration with AI assistants.`,
	Run: func(cmd *cobra.Command, args []string) {
		startMCPServer()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func composeToolOptions(optionSlice []mcp.ToolOption) mcp.ToolOption {
	return func(tool *mcp.Tool) {
		for _, opt := range optionSlice {
			opt(tool)
		}
	}
}

func initTools(kinds schema.KindCatalog, server *server.MCPServer) {
	var format OutputFormat = JSON

	initAllResourcesTool(kinds, server, format)
	initOtherResourcesTool(kinds, server, format)
}

func initAllResourcesTool(kinds schema.KindCatalog, server *server.MCPServer, format OutputFormat) {
	server.AddTool(mcp.NewTool("GetAll", mcp.WithDescription("Get all global resources from Conduktor Console and Conduktor Gateway")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var allResources []resource.Resource
		var errors []error
		kindsByName := sortedKeys(kinds)

		if gatewayApiClientError != nil && *debug {
			errors = append(errors, fmt.Errorf("cannot create Gateway client: %s", gatewayApiClientError))
		}

		if consoleApiClientError != nil && *debug {
			errors = append(errors, fmt.Errorf("cannot create Console client: %s", consoleApiClientError))
		}

		for _, key := range kindsByName {
			kind := savedKindCatalog[key]
			if !kind.IsRootKind() {
				continue
			}

			var resources []resource.Resource
			var err error

			if isGateway(kind) && gatewayApiClientError == nil {
				resources, err = gatewayApiClient().Get(&kind, []string{}, []string{}, map[string]string{})
			} else if isConsole(kind) && consoleApiClientError == nil {
				resources, err = consoleApiClient().Get(&kind, []string{}, []string{}, map[string]string{})
			}

			if err != nil {
				errors = append(errors, fmt.Errorf("error fetching resource %s: %s", kind.GetName(), err))
				continue
			}

			allResources = append(allResources, resources...)
		}

		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}

		result, err := formatResources(allResources, format)

		if err != nil {
			return nil, fmt.Errorf("command failed with error: %w", err)
		}

		return mcp.NewToolResultText(result), nil
	})

}

func initOtherResourcesTool(kinds schema.KindCatalog, server *server.MCPServer, format OutputFormat) {
	for name, kind := range kinds {
		_, isGatewayKind := kind.GetLatestKindVersion().(*schema.GatewayKindVersion)

		var toolOptions []mcp.ToolOption
		if !isGatewayKind {
			toolOptions = append(toolOptions, mcp.WithString("resource_name", mcp.Description("Name of the resource to get")))
			toolOptions = append(toolOptions, mcp.WithDescription(fmt.Sprintf("Get resouce of kind %s from Conduktor Console", name)))
		} else {
			mcp.WithDescription(fmt.Sprintf("Get resouce of kind %s from Conduktor Gateway", name))
		}

		for _, param := range kind.GetParentFlag() {
			toolOptions = append(toolOptions, mcp.WithString(param,
				mcp.Required(),
				mcp.Description(fmt.Sprintf("Parent path parameter %s", param))))
		}

		for _, param := range kind.GetParentQueryFlag() {
			toolOptions = append(toolOptions, mcp.WithString(param,
				mcp.Description(fmt.Sprintf("Parent query parameter %s", param))))
		}

		handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var resourceNames []string
			if resourceName, ok := request.Params.Arguments["resource_name"].(string); ok && resourceName != "" {
				resourceNames = append(resourceNames, resourceName)
			}

			parentFlags := kind.GetParentFlag()
			parentValues := make([]string, len(parentFlags))

			for i, param := range parentFlags {
				if value, ok := request.Params.Arguments[param].(string); ok {
					parentValues[i] = value
				} else {
					return nil, fmt.Errorf("required parameter %s is missing", param)
				}
			}

			parentQueryFlags := kind.GetParentQueryFlag()
			parentQueryValues := make([]string, len(parentQueryFlags))

			for i, param := range parentQueryFlags {
				if value, ok := request.Params.Arguments[param].(string); ok {
					parentQueryValues[i] = value
				} else {
					parentQueryValues[i] = ""
				}
			}

			queryParams := make(map[string]string)
			for key, value := range request.Params.Arguments {
				if key == "resource_name" {
					continue
				}
				if contains(parentFlags, key) || contains(parentQueryFlags, key) {
					continue
				}
				if strValue, ok := value.(string); ok {
					queryParams[key] = strValue
				}
			}

			var jsonResult string
			var err error

			if isGateway(kind) && gatewayApiClientError == nil {
				var results []resource.Resource

				if len(resourceNames) == 0 {
					results, err = gatewayApiClient().Get(&kind, parentValues, parentQueryValues, queryParams)
				} else {
					var result resource.Resource
					result, err = gatewayApiClient().Describe(&kind, parentValues, parentQueryValues, resourceNames[0])
					if err == nil {
						results = append(results, result)
					}
				}

				if err != nil {
					return nil, fmt.Errorf("error fetching resource: %w", err)
				}

				jsonResult, err = formatResources(results, JSON)
			} else if isConsole(kind) && consoleApiClientError == nil {
				var results []resource.Resource

				if len(resourceNames) == 0 {
					results, err = consoleApiClient().Get(&kind, parentValues, parentQueryValues, queryParams)
				} else {
					var result resource.Resource
					result, err = consoleApiClient().Describe(&kind, parentValues, parentQueryValues, resourceNames[0])
					if err == nil {
						results = append(results, result)
					}
				}

				if err != nil {
					return nil, fmt.Errorf("error fetching resource: %w", err)
				}

				jsonResult, err = formatResources(results, JSON)
			} else {
				return nil, fmt.Errorf("no client available for kind %s", kind.GetName())
			}

			if err != nil {
				return nil, fmt.Errorf("command failed with error: %w", err)
			}

			return mcp.NewToolResultText(jsonResult), nil
		}

		server.AddTool(mcp.NewTool(name, composeToolOptions(toolOptions)), handler)
	}

}

func startMCPServer() {
	var consoleKinds *schema.Catalog
	if consoleApiClientError == nil {
		consoleKinds = apiClient_.GetCatalog()
	} else {
		consoleKinds = schema.ConsoleDefaultCatalog()
	}
	gatewayApiClient_, gatewayApiClientError = client.MakeGatewayClientFromEnv()
	var gatewayKinds *schema.Catalog
	if gatewayApiClientError == nil {
		gatewayKinds = gatewayApiClient().GetCatalog()
	} else {
		gatewayKinds = schema.GatewayDefaultCatalog()
	}
	catalog := consoleKinds.Merge(gatewayKinds)

	s := server.NewMCPServer(
		"Conduktor MCP Server",
		"1.0.0",
	)

	initTools(catalog.Kind, s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func formatResources(resources []resource.Resource, format OutputFormat) (string, error) {
	var buffer bytes.Buffer
	err := printResourceToWriter(resources, format, &buffer)
	return buffer.String(), err
}

func printResourceToWriter(resources interface{}, format OutputFormat, writer io.Writer) error {
	switch format {
	case JSON:
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(resources)
	case YAML:
		yamlData, err := yaml.Marshal(resources)
		if err != nil {
			return err
		}
		_, err = writer.Write(yamlData)
		return err
	case NAME:
		resourceList, ok := resources.([]resource.Resource)
		if !ok {
			singleResource, ok := resources.(resource.Resource)
			if !ok {
				return fmt.Errorf("cannot convert resource to expected type")
			}
			resourceList = []resource.Resource{singleResource}
		}

		for _, res := range resourceList {
			fmt.Fprintf(writer, "%s\n", res.Name)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format")
	}
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
