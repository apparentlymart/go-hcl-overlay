module github.com/apparentlymart/go-hcl-overlay/examples/cliargs

go 1.14

require (
	github.com/apparentlymart/go-hcl-overlay v0.0.0-20200515064104-0460c2d0038f
	github.com/hashicorp/hcl/v2 v2.5.1
	github.com/spf13/pflag v1.0.2
)

replace github.com/apparentlymart/go-hcl-overlay => ../..
