version_path = File.expand_path("../clients/ruby/lib/tax_conformance_kit/version.rb", __dir__)
readme_path = File.expand_path("../clients/ruby/README.md", __dir__)

version_text = File.read(version_path)
current = version_text[/VERSION = "(\d+\.\d+\.\d+)"/, 1]
abort "Could not find Ruby client version in #{version_path}" unless current

major, minor, patch = current.split(".").map(&:to_i)
new_version = [major, minor, patch + 1].join(".")

File.write(version_path, version_text.sub(/VERSION = "\d+\.\d+\.\d+"/, %(VERSION = "#{new_version}")))

if File.exist?(readme_path)
  readme_text = File.read(readme_path)
  File.write(readme_path, readme_text.gsub("~> #{current}", "~> #{new_version}"))
end

puts new_version
