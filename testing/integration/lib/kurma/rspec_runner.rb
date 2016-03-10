# encoding: utf-8

# Make sure that we can load kurma.
$LOAD_PATH.unshift(File.join(File.dirname(__FILE__), '..'))
require 'kurma'

class RspecRunner
  attr_accessor :config

  RSPEC_FORMATTER = 'RSpec::Instafail'

  def initialize(opts = {})
    @config = opts

    setup_environment
    validate
  end

  # Run specs with regexp applied. These don't run threaded because
  # it is terribly inefficient to loop through every file.
  def regexp(t)
    RSpec::Core::RakeTask.new(:regexp, :actual_regexp) do |t, args|
      if t.rspec_opts.nil?
        t.rspec_opts = %^#{@config[:format]} -e "#{args[:actual_regexp]}"^
      end
    end
  end

  # Runs a single tag in non-threaded mode. There is no way to do unions in rspec,
  # so it is impossible to run a single tag in threaded tests without running
  # solo tests as well.
  def run(t, tag=nil, summary=true)
    RSpec::Core::RakeTask.new(tag.nil? ? :spec : tag.to_sym) do |t|
      if t.rspec_opts.nil?
        t.rspec_opts = "#{@config[:format_rspec_opts]}"
        t.rspec_opts += " --tag #{tag}" if tag
      end
    end
  end

  private

  def setup_environment
    setup_xml_path
    setup_color
    setup_rspec_opts
    setup_format
    setup_tags
    setup_debug

    # Setup environment variables needed to run tests.
    environment.each do |k, v|
      ENV[k] = v.to_s
    end
  end

  def setup_xml_path
    @config[:xml_path] = 'log/xml/test_results.xml'
    if ENV['XML_REPORT_PATH'] && ENV['XML_REPORT_PATH'] != ""
      @config[:xml_path] = ENV['XML_REPORT_PATH']
    end
  end

  def setup_color
    @config[:color] = '--color'
    if ENV['NO_COLOR'] && ENV['NO_COLOR'] != ""
      @config[:color] = '--no-color'
    end
  end

  def setup_rspec_opts
    @config[:rspec_opts] ||= ENV['RSPEC_OPTS']
  end

  def setup_format
    @config[:format] = "--format #{RSPEC_FORMATTER} --format JUnit -o #{@config[:xml_path]} #{@config[:color]}"
    @config[:format_bootstrap] = "--format #{RSPEC_FORMATTER} #{@config[:color]}"
    @config[:format_rspec_opts] = ["#{@config[:format]}", "#{@config[:rspec_opts]}"].join(' ').strip
  end

  def setup_tags
    @config[:tags] ||= default_tags
    @config[:excluded_tags] ||= default_excluded_tags

    if @config[:merge_default_tags]
      @config[:tags] = (@config[:tags] + default_tags).uniq
      @config[:excluded_tags] = (@config[:excluded_tags] + default_excluded_tags).uniq
    end
  end

  def setup_debug
    if @config[:debug]
      puts 'Enabling APC Debug Mode'
      ENV['APC_DEBUG'] = '1'
    end
  end

  def validate
  end

  def tags
    tag_list = @config[:tags].map do |t|
      "--tag #{t}"
    end.join(' ')

    excluded_tag_list = @config[:excluded_tags].map do |t|
      # Causes errors on Windows with ~, but had previously caused a problem on
      # Linux and tried to expand `~` as the home directory
      ENV["WINDOWS"] == "true" ? "--tag ~#{t}" : "--tag \\~#{t}"
    end.join(' ')

    return [tag_list, excluded_tag_list].join(' ')
  end

  def environment
    e = { }

    e
  end

  def default_tags
    []
  end

  def default_excluded_tags
    tag_set = ['stress', 'burn', 'bootstrap', 'cleanup']
    ENV.each do |k, _|
      if m = k.match(/^RSPEC_ENABLE_TAG_(.*)/)
        tag_set = tag_set - [m[1].downcase]
      end

      if m = k.match(/^RSPEC_DISABLE_TAG_(.*)/)
        tag_set = tag_set + [m[1].downcase]
      end
    end

    tag_set.uniq
  end
end
