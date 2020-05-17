package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/apparentlymart/go-hcl-overlay/hcloverlay"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	flag "github.com/spf13/pflag"
)

func main() {
	var config Config
	rootSchema, _ := gohcl.ImpliedBodySchema(&config)

	// First we extract the arguments that correspond with items in the
	// configuration schema, like --io_mode=foo or
	// --service.foo.bar.listen_addr=blah .
	overlays, args, diags := hcloverlay.ExtractCLIOptions(os.Args[1:], rootSchema)

	// "args" is what's left of the arguments after removing the CLI argument
	// options. We can now use pflag to extract other options that are not
	// related to the configuration schema.
	flags := flag.NewFlagSet("cliargs", flag.ExitOnError)
	flags.Usage = showUsage
	printJSON := flags.BoolP("json", "j", false, "Produce JSON instead of a human-oriented format")
	flags.Parse(args)
	args = flags.Args()

	// Now "args" contains only the non-option arguments. There should be one
	// left, which is the configuration file.
	if len(args) != 1 {
		flags.Usage()
		os.Exit(1)
	}

	filename := args[0]
	parser := hclparse.NewParser()
	file, moreDiags := parser.ParseHCLFile(filename)
	diags = append(diags, moreDiags...)
	body := hcloverlay.ApplyOverlays(file.Body, overlays...)

	moreDiags = gohcl.DecodeBody(body, nil, &config)
	diags = append(diags, moreDiags...)

	pr := hcl.NewDiagnosticTextWriter(os.Stderr, parser.Files(), 80, true)
	pr.WriteDiagnostics(diags)

	if *printJSON {
		printConfigAsJSON(&config)
	} else {
		printConfig(&config)
	}

	if diags.HasErrors() {
		os.Exit(1)
	}
}

func printConfigAsJSON(config *Config) {
	result, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		// We control the input types here, so this should never fail
		panic(err)
	}
	fmt.Println(string(result))
}

func printConfig(config *Config) {
	fmt.Printf("The IO mode is %q\n\n", config.IOMode)

	for _, svc := range config.Services {
		fmt.Printf("- Service %q %q:\n", svc.Type, svc.Name)
		fmt.Printf("  The listen address is %s\n\n", svc.ListenAddr)
	}
}

func showUsage() {
	fmt.Fprintln(os.Stderr, strings.TrimSpace(`
go run . [options] <config-file>

Options:
  --json, -j
      Produce JSON output instead of human-oriented output.

  --io_mode=MODE
      Override the io_mode configuration argument.

  --service.TYPE.NAME.listen_addr=ADDR
	  Override the listen address for the service with the given TYPE and NAME.
`))
}

type ServiceConfig struct {
	Type       string `hcl:"type,label" json:"type"`
	Name       string `hcl:"name,label" json:"name"`
	ListenAddr string `hcl:"listen_addr" json:"listen_addr"`
}
type Config struct {
	IOMode   string          `hcl:"io_mode" json:"io_mode"`
	Services []ServiceConfig `hcl:"service,block" json:"services"`
}
