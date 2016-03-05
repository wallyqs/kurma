# -*- encoding: utf-8 -*-
# stub: rspec-prof 0.0.6 ruby lib

Gem::Specification.new do |s|
  s.name = "rspec-prof"
  s.version = "0.0.6"

  s.required_rubygems_version = Gem::Requirement.new(">= 0") if s.respond_to? :required_rubygems_version=
  s.require_paths = ["lib"]
  s.authors = ["Colin MacKenzie IV"]
  s.date = "2011-08-27"
  s.description = "Integrates ruby-prof with RSpec, allowing you to easily profile your RSpec examples."
  s.email = "sinisterchipmunk@gmail.com"
  s.extra_rdoc_files = ["LICENSE", "README.rdoc"]
  s.files = [".document", ".gitignore", ".travis.yml", "Gemfile", "Gemfile.lock", "LICENSE", "README.rdoc", "Rakefile", "VERSION", "features/profile_with_env_vars.feature", "features/step_definitions/environment_variable_steps.rb", "features/step_definitions/pass_fail_steps.rb", "features/support/env.rb", "features/support/reset_env.rb", "features/support/spec_helper.rb", "lib/rspec-prof.rb", "lib/rspec-prof/filename_helpers.rb", "rspec-prof.gemspec"]
  s.homepage = "http://www.thoughtsincomputation.com/"
  s.licenses = ["MIT"]
  s.rubygems_version = "2.4.5.1"
  s.summary = "Integrates ruby-prof with RSpec, allowing you to easily profile your RSpec examples."
  s.test_files = ["features/profile_with_env_vars.feature", "features/step_definitions/environment_variable_steps.rb", "features/step_definitions/pass_fail_steps.rb", "features/support/env.rb", "features/support/reset_env.rb", "features/support/spec_helper.rb"]

  if s.respond_to? :specification_version then
    s.specification_version = 4

    if Gem::Version.new(Gem::VERSION) >= Gem::Version.new('1.2.0') then
      s.add_runtime_dependency(%q<rspec>, ["~> 3.0"])
      s.add_runtime_dependency(%q<ruby-prof>, [">= 0"])
      s.add_development_dependency(%q<cucumber>, [">= 0"])
      s.add_development_dependency(%q<aruba>, [">= 0"])
      s.add_development_dependency(%q<simplecov>, [">= 0"])
      s.add_development_dependency(%q<coveralls>, [">= 0"])
      s.add_development_dependency(%q<rake>, [">= 0"])
    else
      s.add_dependency(%q<rspec>, ["~> 3.0"])
      s.add_dependency(%q<ruby-prof>, [">= 0"])
      s.add_dependency(%q<cucumber>, [">= 0"])
      s.add_dependency(%q<aruba>, [">= 0"])
      s.add_dependency(%q<simplecov>, [">= 0"])
      s.add_dependency(%q<coveralls>, [">= 0"])
      s.add_dependency(%q<rake>, [">= 0"])
    end
  else
    s.add_dependency(%q<rspec>, ["~> 3.0"])
    s.add_dependency(%q<ruby-prof>, [">= 0"])
    s.add_dependency(%q<cucumber>, [">= 0"])
    s.add_dependency(%q<aruba>, [">= 0"])
    s.add_dependency(%q<simplecov>, [">= 0"])
    s.add_dependency(%q<coveralls>, [">= 0"])
    s.add_dependency(%q<rake>, [">= 0"])
  end
end
