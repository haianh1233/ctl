package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/conduktor/ctl/resource"
	"github.com/conduktor/ctl/schema"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag/v2"
)

type OutputFormat enumflag.Flag

var savedKindCatalog schema.KindCatalog

const (
	JSON OutputFormat = iota
	YAML
	NAME
)

var OutputFormatIds = map[OutputFormat][]string{
	JSON: {"json"},
	YAML: {"yaml"},
	NAME: {"name"},
}

func (o OutputFormat) String() string {
	return OutputFormatIds[o][0]
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get resource of a given kind",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Root command does nothing
		cmd.Help()
		os.Exit(1)
	},
}

func removeTrailingSIfAny(name string) string {
	return strings.TrimSuffix(name, "s")
}

func buildAlias(name string) []string {
	aliases := []string{strings.ToLower(name)}
	// This doesn't seem to be needed since none of the kinds ends with an S
	// However I'm leaving it here as a conditional so it won't affect the usage
	if strings.HasSuffix(name, "s") {
		aliases = append(aliases, removeTrailingSIfAny(name), removeTrailingSIfAny(strings.ToLower(name)))
	}
	return aliases
}

func printResource(result interface{}, format OutputFormat) error {
	switch format {
	case JSON:
		jsonOutput, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshalling JSON: %s\n%s", err, result)
		}
		fmt.Println(string(jsonOutput))
	case NAME:
		// show Kind/Name
		switch res := result.(type) {
		case []resource.Resource:
			for _, r := range res {
				fmt.Println(r.Kind + "/" + r.Name)
			}
		case resource.Resource:
			fmt.Println(res.Kind + "/" + res.Name)
		default:
			return fmt.Errorf("unexpected resource type")
		}
	case YAML:
		switch res := result.(type) {
		case []resource.Resource:
			for _, r := range res {
				fmt.Println("---")
				r.PrintPreservingOriginalFieldOrder()
			}
		case resource.Resource:
			res.PrintPreservingOriginalFieldOrder()
		default:
			return fmt.Errorf("unexpected resource type")
		}
	default:
		return fmt.Errorf("invalid output format %s", format.String())
	}
	return nil
}

func isGateway(kind schema.Kind) bool {
	_, isGatewayKind := kind.GetLatestKindVersion().(*schema.GatewayKindVersion)
	return isGatewayKind
}

func isConsole(kind schema.Kind) bool {
	_, isConsoleKind := kind.GetLatestKindVersion().(*schema.ConsoleKindVersion)
	return isConsoleKind
}

func initGet(kinds schema.KindCatalog) {
	rootCmd.AddCommand(getCmd)
	savedKindCatalog = kinds
	var format OutputFormat = YAML

	var onlyGateway *bool
	var onlyConsole *bool
	var allCmd = &cobra.Command{
		Use:   "all",
		Short: "Get all global resources",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			var allResources []resource.Resource

			kindsByName := sortedKeys(kinds)
			if gatewayApiClientError != nil {
				if *debug || *onlyGateway {
					fmt.Fprintf(os.Stderr, "Cannot create Gateway client: %s\n", gatewayApiClientError)
				}
			}
			if consoleApiClientError != nil {
				if *debug || *onlyConsole {
					fmt.Fprintf(os.Stderr, "Cannot create Console client: %s\n", consoleApiClientError)
				}
			}
			for _, key := range kindsByName {
				kind := kinds[key]
				// keep only the Kinds where listing is provided TODO fix if config is provided
				if !kind.IsRootKind() {
					continue
				}
				var resources []resource.Resource
				var err error
				if isGateway(kind) && !*onlyConsole && gatewayApiClientError == nil {
					resources, err = gatewayApiClient().Get(&kind, []string{}, []string{}, map[string]string{})
				} else if isConsole(kind) && !*onlyGateway && consoleApiClientError == nil {
					resources, err = consoleApiClient().Get(&kind, []string{}, []string{}, map[string]string{})
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error fetching resource %s: %s\n", kind.GetName(), err)
					continue
				}

				allResources = append(allResources, resources...)
			}
			err := printResource(allResources, format)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				os.Exit(1)
			}
		},
	}
	onlyGateway = allCmd.Flags().BoolP("gateway", "g", false, "Only show gateway resources")
	onlyConsole = allCmd.Flags().BoolP("console", "c", false, "Only show console resources")
	allCmd.MarkFlagsMutuallyExclusive("gateway", "console")
	allCmd.Flags().VarP(enumflag.New(&format, "output", OutputFormatIds, enumflag.EnumCaseInsensitive), "output", "o", "Output format. One of: json|yaml|name")
	getCmd.AddCommand(allCmd)

	// Add all kinds to the 'get' command
	for name, kind := range kinds {
		gatewayKind, isGatewayKind := kind.GetLatestKindVersion().(*schema.GatewayKindVersion)
		args := cobra.MaximumNArgs(1)
		use := fmt.Sprintf("%s [name]", name)
		if isGatewayKind && !gatewayKind.GetAvailable {
			args = cobra.NoArgs
			use = fmt.Sprintf("%s", name)
		}
		parentFlags := kind.GetParentFlag()
		parentQueryFlags := kind.GetParentQueryFlag()
		parentFlagValue := make([]*string, len(parentFlags))
		parentQueryFlagValue := make([]*string, len(parentQueryFlags))
		var multipleFlags *MultipleFlags
		kindCmd := &cobra.Command{
			Use:     use,
			Short:   "Get resource of kind " + name,
			Args:    args,
			Long:    `If name not provided it will list all resource`,
			Aliases: buildAlias(name),
			Run: func(cmd *cobra.Command, args []string) {
				parentValue := make([]string, len(parentFlagValue))
				parentQueryValue := make([]string, len(parentQueryFlagValue))
				queryParams := multipleFlags.ExtractFlagValueForQueryParam()
				for i, v := range parentFlagValue {
					parentValue[i] = *v
				}
				for i, v := range parentQueryFlagValue {
					parentQueryValue[i] = *v
				}

				var err error

				if len(args) == 0 {
					var result []resource.Resource
					if isGatewayKind {
						result, err = gatewayApiClient().Get(&kind, parentValue, parentQueryValue, queryParams)
					} else {
						result, err = consoleApiClient().Get(&kind, parentValue, parentQueryValue, queryParams)
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error fetching resources: %s\n", err)
						return
					}
					err = printResource(result, format)
				} else if len(args) == 1 {
					var result resource.Resource
					if isGatewayKind {
						result, err = gatewayApiClient().Describe(&kind, parentValue, parentQueryValue, args[0])
					} else {
						result, err = consoleApiClient().Describe(&kind, parentValue, parentQueryValue, args[0])
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error describing resource: %s\n", err)
						return
					}
					err = printResource(result, format)
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					os.Exit(1)
				}
			},
		}
		for i, flag := range parentFlags {
			parentFlagValue[i] = kindCmd.Flags().String(flag, "", "Parent "+flag)
			kindCmd.MarkFlagRequired(flag)
		}
		for i, flag := range parentQueryFlags {
			parentQueryFlagValue[i] = kindCmd.Flags().String(flag, "", "Parent "+flag)
		}
		multipleFlags = NewMultipleFlags(kindCmd, kind.GetListFlag())
		kindCmd.Flags().VarP(enumflag.New(&format, "output", OutputFormatIds, enumflag.EnumCaseInsensitive), "output", "o", "Output format. One of: json|yaml|name")
		getCmd.AddCommand(kindCmd)
	}
}

func get(kindName string, outputFormat string, allFlags map[string]string, args []string) (string, error) {
	var format OutputFormat
	switch strings.ToLower(outputFormat) {
	case "json":
		format = JSON
	case "yaml":
		format = YAML
	case "name":
		format = NAME
	default:
		return "", fmt.Errorf("invalid output format: %s. Supported formats: json, yaml, name", outputFormat)
	}

	if kindName == "all" {
		return getAllResources(format)
	}

	kind, exists := savedKindCatalog[kindName]
	if !exists {
		return "", fmt.Errorf("kind %s not found in configuration", kindName)
	}

	isGatewayKind := isGateway(kind)

	// Extract parent flag values in the order expected by the API client
	parentFlagValues := make([]string, 0, len(kind.GetParentFlag()))
	parentQueryFlagValues := make([]string, 0, len(kind.GetParentQueryFlag()))
	queryParams := make(map[string]string)

	// Handle parent flags (path parameters)
	for _, flagName := range kind.GetParentFlag() {
		value, exists := allFlags[flagName]
		if !exists {
			return "", fmt.Errorf("required parent flag --%s not provided for kind %s", flagName, kindName)
		}
		parentFlagValues = append(parentFlagValues, value)
		// Remove from allFlags to avoid duplication in queryParams
		delete(allFlags, flagName)
	}

	// Handle parent query flags
	for _, flagName := range kind.GetParentQueryFlag() {
		value, exists := allFlags[flagName]
		if exists {
			parentQueryFlagValues = append(parentQueryFlagValues, value)
			// Remove from allFlags to avoid duplication in queryParams
			delete(allFlags, flagName)
		} else {
			parentQueryFlagValues = append(parentQueryFlagValues, "")
		}
	}

	for k, v := range allFlags {
		queryParams[k] = v
	}

	var result []resource.Resource
	var err error

	if isGatewayKind {
		if gatewayApiClientError != nil {
			return "", fmt.Errorf("cannot create Gateway client: %s", gatewayApiClientError)
		}

		switch len(args) {
		case 0:
			result, err = gatewayApiClient().Get(&kind, parentFlagValues, parentQueryFlagValues, queryParams)
		case 1:
			singleResource, descErr := gatewayApiClient().Describe(&kind, parentFlagValues, parentQueryFlagValues, args[0])
			if descErr != nil {
				err = descErr
			} else {
				result = []resource.Resource{singleResource}
			}
		default:
			return "", fmt.Errorf("invalid number of arguments for kind %s", kindName)
		}
	} else {
		if consoleApiClientError != nil {
			return "", fmt.Errorf("cannot create Console client: %s", consoleApiClientError)
		}

		switch len(args) {
		case 0:
			result, err = consoleApiClient().Get(&kind, parentFlagValues, parentQueryFlagValues, queryParams)
		case 1:
			singleResource, err := consoleApiClient().Describe(&kind, parentFlagValues, parentQueryFlagValues, args[0])
			if err != nil {
				return "", fmt.Errorf("error fetching/describing resources of kind %s: %s", kindName, err)
			} else {
				result = []resource.Resource{singleResource}
			}
		default:
			return "", fmt.Errorf("invalid number of arguments for kind %s", kindName)
		}
	}

	if err != nil {
		return "", fmt.Errorf("error fetching/describing resources of kind %s: %s", kindName, err)
	}

	return formatResources(result, format)
}

func getAllResources(format OutputFormat) (string, error) {
	var allResources []resource.Resource
	var errors []error

	kindsByName := sortedKeys(savedKindCatalog)

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

	return formatResources(allResources, format)
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

func sortedKeys(kinds schema.KindCatalog) []string {
	keys := make([]string, 0, len(kinds))
	for key := range kinds {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
