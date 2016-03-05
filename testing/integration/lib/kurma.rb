# encoding: utf-8

require "resolv-replace"
require 'logger'
require 'json'
require 'open3'
require 'securerandom'
require 'uri'
require 'fileutils'
require 'tempfile'
require 'yarjuf'

module Kurma
  ROOT_PATH = File.expand_path('..', File.dirname(__FILE__))
  LOG = Logger.new(File.join(ROOT_PATH, 'log/test.log'))
  LOG.level = Logger::INFO
  LOG.formatter = proc do |severity, datetime, progname, msg|
    "#{severity[0]}#{datetime.strftime("%m%d %H:%M:%S.%6N")} ##{Process.pid}] #{msg}\n"
  end
end

require 'kurma/errors'
require 'kurma/api_client'
require 'kurma/cli_client'
require 'kurma/server'
require 'rspec/core/formatters/progress_formatter'
require 'kurma/rspec_formatter'
