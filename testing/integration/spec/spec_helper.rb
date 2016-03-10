# encoding: utf-8

# Set ruby internal and external encoding.
Encoding.default_internal = Encoding::UTF_8
Encoding.default_external = Encoding::UTF_8

require 'kurma'
require 'time'
require 'pathname'
require 'socket'
require 'set'
require 'httparty'

# See http://rubydoc.info/gems/rspec-core/RSpec/Core/Configuration
RSpec.configure do |config|
  config.run_all_when_everything_filtered = false
  config.disable_monkey_patching!

  Dir["./spec/support/**/*.rb"].sort.each {|f| require f}

  # Do tests in random order, if you need to replay then keep your seed!
  config.order = 'random'

  config.include TestHelpers

  config.before(:all) do
    @server = Kurma::Server.new
    @server.start
  end

  config.after(:all) do
    @server.stop
  end

  config.before(:each) do
    log.info "=== RUN #{RSpec.current_example.full_description}"

    # Array of items we want to clean up.
    @cleanup = []
  end

  config.after(:each) do
    # Start cleaning up.
    begin
      @cleanup.reverse.each do |cmd|
        cli.run!(cmd)
      end
    ensure
      if RSpec.current_example.exception
        log.error "--- FAIL: #{RSpec.current_example.full_description}"
        log.error RSpec.current_example.location
      else
        log.error "--- PASS: #{RSpec.current_example.full_description}"
      end
    end
  end
end
