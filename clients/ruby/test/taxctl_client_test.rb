require_relative "test_helper"

module TaxConformanceKit
  class TaxctlClientTest < ClientTestCase
    def test_exposes_version
      refute_nil TaxConformanceKit::VERSION
    end

    def test_rejects_empty_command
      error = assert_raises(ArgumentError) do
        TaxctlClient.new(command: [])
      end

      assert_equal "command cannot be empty", error.message
    end

    def test_rejects_non_string_non_array_command
      error = assert_raises(ArgumentError) do
        TaxctlClient.new(command: :taxctl)
      end

      assert_equal "command must be a String or Array", error.message
    end

    def test_validate_real_ruleset
      ruleset = read_ruleset("regulation", "nl", "gemeentelijke_verordening", "amsterdam", "2026-01-01.json")

      response = client.validate(ruleset: ruleset)

      assert_equal true, response["ok"]
      assert_equal "v1", response["api_version"]
      assert_equal 1, response["rule_count"]
    end

    def test_validate_with_inline_registry
      ruleset = read_ruleset("regulation", "nl", "gemeentelijke_verordening", "amsterdam", "2026-01-01.json")
      inline_client = TaxctlClient.new(
        command: ["go", "run", "./cmd/taxctl"],
        registry_path: nil,
        chdir: go_dir.to_s,
        env: {
          "GOCACHE" => "/tmp/tck-gocache",
          "CCACHE_DISABLE" => "1"
        }
      )

      response = inline_client.validate(ruleset: ruleset, kind_registry: registry)

      assert_equal true, response["ok"]
      assert_equal 1, response["rule_count"]
    end

    def test_evaluate_booking_matches_breda_conformance_case
      conformance = read_json("core", "fixtures", "regulation", "nl", "gemeentelijke_verordening", "breda", "conformance", "camping_arrangement_guest.json")
      ruleset = read_ruleset("regulation", "nl", "gemeentelijke_verordening", "breda", "2026-01-01.json")

      response = client.evaluate(
        ruleset: ruleset,
        booking_input: conformance.fetch("booking_input")
      )

      assert_equal true, response["ok"]
      assert_equal conformance.dig("expected", "total_tax"), response.dig("result", "total_tax")
      assert_equal conformance.dig("expected", "matched_rule_ids"), response.dig("result", "matched_rule_ids")
      assert_equal 1, response["result"]["matched_rule_ids"].length
    end

    def test_evaluate_assessment_matches_amsterdam_conformance_case
      conformance = read_json("core", "fixtures", "regulation", "nl", "gemeentelijke_verordening", "amsterdam", "conformance", "quarterly_threshold_met.assessment.json")
      ruleset = read_ruleset("regulation", "nl", "gemeentelijke_verordening", "amsterdam", "2026-01-01.json")

      response = client.evaluate_assessment(
        ruleset: ruleset,
        assessment_input: conformance.fetch("assessment_input")
      )

      assert_equal true, response["ok"]
      assert_equal conformance.dig("expected", "total_booking_tax"), response.dig("result", "total_booking_tax")
      assert_equal conformance.dig("expected", "total_assessment_tax"), response.dig("result", "total_assessment_tax")
      assert_equal conformance.dig("expected", "booking_results"), response.dig("result", "booking_results")
      assert_equal 2, response["result"]["booking_results"].length
    end

    def test_evaluate_assessment_below_threshold_matches_conformance_case
      conformance = read_json("core", "fixtures", "regulation", "nl", "gemeentelijke_verordening", "amsterdam", "conformance", "quarterly_threshold_below_minimum.assessment.json")
      ruleset = read_ruleset("regulation", "nl", "gemeentelijke_verordening", "amsterdam", "2026-01-01.json")

      response = client.evaluate_assessment(
        ruleset: ruleset,
        assessment_input: conformance.fetch("assessment_input")
      )

      assert_equal true, response["ok"]
      assert_equal 20.0, response.dig("result", "total_booking_tax")
      assert_equal 0.0, response.dig("result", "total_assessment_tax")
    end

    def test_raises_on_runtime_validation_error
      ruleset = read_ruleset("regulation", "nl", "gemeentelijke_verordening", "amsterdam", "2026-01-01.json")
      broken_ruleset = Marshal.load(Marshal.dump(ruleset))
      broken_ruleset.fetch("rules").first.fetch("calculation")["kind"] = "bogus.unknown"

      error = assert_raises(Error) do
        client.validate(ruleset: broken_ruleset)
      end

      assert_includes error.message, "unknown calculation kind"
    end

    def test_raises_on_invalid_json_response
      invalid_client = TaxctlClient.new(command: ["ruby", "-e", 'STDOUT.write("not json")'])

      error = assert_raises(Error) do
        invalid_client.validate(ruleset: {})
      end

      assert_includes error.message, "invalid taxctl response"
    end

    def test_prefers_stderr_when_subprocess_fails_before_json
      failing_client = TaxctlClient.new(command: ["ruby", "-e", 'STDERR.write("boom"); exit 1'])

      error = assert_raises(Error) do
        failing_client.validate(ruleset: {})
      end

      assert_equal "boom", error.message
    end
  end
end
