# tax_conformance_kit

Thin Ruby client for the `tax-conformance-kit` Go runtime.

## Install

Bundler (RubyGems):

```ruby
gem "tax_conformance_kit", "~> 0.2.0"
```

Path dependency (for local development):

```ruby
gem "tax_conformance_kit", path: "../tax-conformance-kit/clients/ruby"
```

Git dependency:

```ruby
gem "tax_conformance_kit", git: "https://github.com/odeva-labs/tax-conformance-kit.git", glob: "clients/ruby/*.gemspec"
```

## Usage

Zero-config — the gem ships with the bundled `taxctl` binary and the kind registry:

```ruby
require "tax_conformance_kit"

client = TaxConformanceKit::TaxctlClient.new
response = client.evaluate(ruleset: ruleset_hash, booking_input: booking_input_hash)
resolved = client.evaluate_resolved(booking_input: booking_input_hash)
```

### Advanced: override `command:`

`command:` may be either:

- a binary path string, for example `"/usr/local/bin/taxctl"`
- a full argv array, for example `["go", "run", "./cmd/taxctl"]`

```ruby
client = TaxConformanceKit::TaxctlClient.new(
  command: ["/path/to/taxctl"],
  registry_path: "/path/to/kind-registry.v1.json"
)
```

The client does not implement the evaluator. It shells out to the canonical Go runtime and exchanges JSON over stdin/stdout.
