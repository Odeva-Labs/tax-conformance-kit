# tax_conformance_kit

Thin Ruby client for the `tax-conformance-kit` Go runtime.

## Install

Path dependency:

```ruby
gem "tax_conformance_kit", path: "../tax-conformance-kit/clients/ruby"
```

Git dependency:

```ruby
gem "tax_conformance_kit", git: "https://github.com/ramones/tax-conformance-kit.git", glob: "clients/ruby/*.gemspec"
```

## Usage

```ruby
require "tax_conformance_kit"

client = TaxConformanceKit::TaxctlClient.new(
  command: ["/path/to/taxctl"],
  registry_path: "/path/to/kind-registry.v1.json"
)

response = client.evaluate(
  ruleset: ruleset_hash,
  booking_input: booking_input_hash
)
```

`command:` may be either:

- a binary path string, for example `"/usr/local/bin/taxctl"`
- a full argv array, for example `["go", "run", "./cmd/taxctl"]`

The client does not implement the evaluator. It shells out to the canonical Go runtime and exchanges JSON over stdin/stdout.
