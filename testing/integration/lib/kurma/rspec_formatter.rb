# encoding: utf-8

require 'rspec/core/formatters/documentation_formatter'

module RSpec
  class Instafail < RSpec::Core::Formatters::DocumentationFormatter
    def example_failed(example)
      super(example)
      # also do what dump_failures for all failures at the end right now for this one
      index = failed_examples.size - 1
      pending_fixed?(example) ? dump_pending_fixed(example, index) : dump_failure(example, index)
      dump_backtrace(example)
    end
  end

  class DocInstafail < Instafail; end
end
