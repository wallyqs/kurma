# encoding: utf-8

require 'faraday'
require 'faraday/adapter/net_http'
require 'jsonrpc-client'
require 'net_http_unix'

Faraday::Adapter::NetHttp.class_eval do
  def net_http_connection(env)
    NetX::HTTPUnix.new("unix://#{ENV["KURMA_HOST"]}")
  end
end

class Kurma::ApiClient
  attr_reader :client

  def initialize
    @client = JSONRPC::Client.new('http://example.com/rpc')
  end

  def list_containers
    @client.invoke("Containers.List", [])
  end
end
