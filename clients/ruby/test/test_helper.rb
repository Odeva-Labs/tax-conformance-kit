require "json"
require "pathname"
require "minitest/autorun"

require_relative "../lib/tax_conformance_kit"

module TaxConformanceKit
  class ClientTestCase < Minitest::Test
    def repo_root
      @repo_root ||= Pathname(__dir__).join("..", "..", "..").realpath
    end

    def go_dir
      repo_root.join("engine")
    end

    def registry_path
      repo_root.join("core/schemas/kind-registry.v1.json")
    end

    def registry
      @registry ||= begin
        raw = read_json("core", "schemas", "kind-registry.v1.json")
        {
          "registry_version" => raw.fetch("registry_version"),
          "calculations" => raw.fetch("calculations"),
          "predicates" => raw.fetch("predicates")
        }
      end
    end

    def client
      @client ||= TaxctlClient.new(
        command: ["go", "run", "./cmd/taxctl"],
        registry_path: registry_path.to_s,
        chdir: go_dir.to_s,
        env: {
          "GOCACHE" => "/tmp/tck-gocache",
          "CCACHE_DISABLE" => "1"
        }
      )
    end

    def read_json(*parts)
      JSON.parse(repo_root.join(*parts).read)
    end

    def read_ruleset(*parts)
      read_json("core", "fixtures", *parts)
    end
  end
end
