set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

go_dir := "engines/go"
amsterdam_rules := "../../core/fixtures/regulation/nl/gemeentelijke_verordening/amsterdam/2026-01-01.json"
amsterdam_assessment_case := "../../core/fixtures/regulation/nl/gemeentelijke_verordening/amsterdam/conformance/quarterly_threshold_met.assessment.json"

default:
  just --list

test:
  cd {{go_dir}} && go test ./...

test-ruby-client:
  ruby -I clients/ruby/lib -I clients/ruby/test -e 'Dir["clients/ruby/test/*_test.rb"].sort.each { |path| require_relative path }'

test-ruby-gem:
  tmpdir="$(mktemp -d /tmp/tck-ruby-gem-XXXXXX)" && printf 'source "https://rubygems.org"\ngem "tax_conformance_kit", path: "%s/clients/ruby"\n' "{{justfile_directory()}}" > "$tmpdir/Gemfile" && cd "$tmpdir" && BUNDLE_PATH="$tmpdir/vendor/bundle" bundle install --local >/dev/null && BUNDLE_GEMFILE="$tmpdir/Gemfile" bundle exec ruby -e 'require "tax_conformance_kit"; puts TaxConformanceKit::VERSION'

validate rules=amsterdam_rules:
  cd {{go_dir}} && go run ./cmd/taxctl validate -rules {{rules}}

evaluate-assessment case=amsterdam_assessment_case:
  cd {{go_dir}} && go run ./cmd/taxctl evaluate-assessment -case {{case}}

runtime-validate input="-":
  cd {{go_dir}} && go run ./cmd/taxctl runtime-validate -input {{input}}

runtime-evaluate input="-":
  cd {{go_dir}} && go run ./cmd/taxctl runtime-evaluate -input {{input}}

runtime-evaluate-assessment input="-":
  cd {{go_dir}} && go run ./cmd/taxctl runtime-evaluate-assessment -input {{input}}

harvest output="/tmp/tck-cvdr-index":
  cd {{go_dir}} && go run ./cmd/taxctl harvest-cvdr -output-dir {{output}}

select index="/tmp/tck-cvdr-index" as_of="2026-04-17" output="/tmp/tck-cvdr-selection":
  cd {{go_dir}} && go run ./cmd/taxctl select-cvdr-candidates -index-dir {{index}} -as-of-date {{as_of}} -output-dir {{output}}

extract selection="/tmp/tck-cvdr-selection" output="/tmp/tck-cvdr-extract":
  cd {{go_dir}} && go run ./cmd/taxctl extract-cvdr-stubs -selection-dir {{selection}} -output-dir {{output}}

analyze extraction="/tmp/tck-cvdr-extract":
  cd {{go_dir}} && go run ./cmd/taxctl analyze-cvdr-stubs -extraction-dir {{extraction}}

generate extraction="/tmp/tck-cvdr-extract":
  cd {{go_dir}} && go run ./cmd/taxctl generate-draft-fixtures -extraction-dir {{extraction}} -overwrite

import-municipality-codes archive:
  cd {{go_dir}} && go run ./cmd/taxctl import-cbs-municipality-codes -archive {{archive}}

backfill-municipality-codes:
  cd {{go_dir}} && go run ./cmd/taxctl backfill-municipality-codes
