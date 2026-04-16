require_relative "test_helper"

module TaxConformanceKit
  class ConformanceCasesTest < ClientTestCase
    ROOT = File.expand_path("../../..", __dir__)

    Dir.glob(File.join(ROOT, "core/fixtures/**/conformance/*.json")).sort.each do |case_path|
      filename = File.basename(case_path, ".json")
      test_name = "test_conformance_#{filename.gsub(/[^a-z0-9]+/i, '_')}"

      define_method(test_name) do
        conformance = JSON.parse(File.read(case_path))
        ruleset_path = File.expand_path(conformance.fetch("rule_set_path"), File.dirname(case_path))
        ruleset = JSON.parse(File.read(ruleset_path))

        if case_path.end_with?(".assessment.json")
          response = client.evaluate_assessment(
            ruleset: ruleset,
            assessment_input: conformance.fetch("assessment_input")
          )

          assert_equal true, response["ok"], case_path
          assert_equal conformance.dig("expected", "total_booking_tax"), response.dig("result", "total_booking_tax"), case_path
          assert_equal conformance.dig("expected", "total_assessment_tax"), response.dig("result", "total_assessment_tax"), case_path
          assert_equal conformance.dig("expected", "booking_results"), response.dig("result", "booking_results"), case_path
        else
          response = client.evaluate(
            ruleset: ruleset,
            booking_input: conformance.fetch("booking_input")
          )

          assert_equal true, response["ok"], case_path
          assert_equal conformance.dig("expected", "total_tax"), response.dig("result", "total_tax"), case_path
          assert_equal conformance.dig("expected", "matched_rule_ids"), response.dig("result", "matched_rule_ids"), case_path
        end
      end
    end
  end
end
