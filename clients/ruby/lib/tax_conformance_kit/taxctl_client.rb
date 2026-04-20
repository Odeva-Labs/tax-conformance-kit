require "json"
require "open3"

module TaxConformanceKit
  class Error < StandardError; end

  class TaxctlClient
    def initialize(command: TaxConformanceKit.taxctl_path, registry_path: TaxConformanceKit.kind_registry_path, chdir: nil, env: {})
      @command = normalize_command(command)
      @registry_path = registry_path
      @chdir = chdir
      @env = env
    end

    def validate(ruleset:, kind_registry: nil)
      run_json("runtime-validate", {
        ruleset: ruleset,
        kind_registry: kind_registry
      })
    end

    def evaluate(ruleset:, booking_input:, kind_registry: nil)
      run_json("runtime-evaluate", {
        ruleset: ruleset,
        booking_input: booking_input,
        kind_registry: kind_registry
      })
    end

    def evaluate_resolved(booking_input:, fixture_root: nil, domain: nil, kind_registry: nil)
      run_json("runtime-resolve-evaluate", {
        fixture_root: fixture_root,
        domain: domain,
        booking_input: booking_input,
        kind_registry: kind_registry
      })
    end

    def evaluate_assessment(ruleset:, assessment_input:, kind_registry: nil)
      run_json("runtime-evaluate-assessment", {
        ruleset: ruleset,
        assessment_input: assessment_input,
        kind_registry: kind_registry
      })
    end

    def evaluate_resolved_assessment(assessment_input:, fixture_root: nil, domain: nil, kind_registry: nil)
      run_json("runtime-resolve-evaluate-assessment", {
        fixture_root: fixture_root,
        domain: domain,
        assessment_input: assessment_input,
        kind_registry: kind_registry
      })
    end

    private

    def normalize_command(command)
      case command
      when String
        [command]
      when Array
        raise ArgumentError, "command cannot be empty" if command.empty?

        command
      else
        raise ArgumentError, "command must be a String or Array"
      end
    end

    def run_json(subcommand, payload)
      argv = [*@command, subcommand]
      argv += ["-registry", @registry_path] if @registry_path

      options = { stdin_data: JSON.generate(payload) }
      options[:chdir] = @chdir unless @chdir.nil?

      stdout, stderr, status = Open3.capture3(@env, *argv, **options)
      response = parse_response(stdout)

      return response if status.success? && response["ok"]

      message = response.dig("error", "message")
      message = stderr.strip if message.nil? || message.empty?
      raise Error, message
    rescue Error
      raise
    rescue JSON::ParserError => e
      message = stderr.strip
      raise Error, message unless message.empty?

      raise Error, "invalid taxctl response: #{e.message}"
    end

    def parse_response(stdout)
      JSON.parse(stdout)
    end
  end
end
