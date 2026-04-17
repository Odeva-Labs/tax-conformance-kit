require_relative "tax_conformance_kit/version"
require_relative "tax_conformance_kit/taxctl_client"

module TaxConformanceKit
  GEM_ROOT = File.expand_path("..", __dir__)

  def self.taxctl_path
    File.join(GEM_ROOT, "bin", "taxctl")
  end

  def self.ruleset_root
    File.join(GEM_ROOT, "data", "regulation")
  end

  def self.kind_registry_path
    File.join(GEM_ROOT, "data", "kind-registry.v1.json")
  end
end
