module github.com/monetakit/contrib

go 1.26

require github.com/monetakit/monetakit v0.0.0

require (
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/hashicorp/hcl/v2 v2.24.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/tmccombs/hcl2json v0.6.9 // indirect
	github.com/zclconf/go-cty v1.18.0 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
)

// Core has no tagged release yet. Until it does, develop with a sibling
// checkout (git clone both repos into the same parent directory).
// Once core tags v0.x, drop this replace and pin the require above.
replace github.com/monetakit/monetakit => ../monetakit
