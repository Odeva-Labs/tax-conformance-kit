require_relative "lib/tax_conformance_kit/version"

Gem::Specification.new do |spec|
  spec.name = "tax_conformance_kit"
  spec.version = TaxConformanceKit::VERSION
  spec.authors = ["ramones"]
  spec.email = ["ramonvansprundel@gmail.com"]

  spec.summary = "Thin Ruby client for the tax-conformance-kit Go runtime"
  spec.description = "Ruby adapter for runtime-validate, runtime-evaluate, and runtime-evaluate-assessment."
  spec.homepage = "https://github.com/odeva-labs/tax-conformance-kit"
  spec.license = "MIT"
  spec.required_ruby_version = ">= 3.1"

  spec.files = Dir[
    "lib/**/*.rb",
    "bin/taxctl",
    "data/**/*.json",
    "README.md",
    "LICENSE"
  ]
  spec.require_paths = ["lib"]

  spec.metadata["homepage_uri"] = spec.homepage
  spec.metadata["source_code_uri"] = spec.homepage
  spec.metadata["allowed_push_host"] = "https://rubygems.org"
  spec.metadata["rubygems_mfa_required"] = "true"
end
