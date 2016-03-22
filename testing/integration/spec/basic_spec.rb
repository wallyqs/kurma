# encoding: utf-8
require 'spec_helper'

RSpec.describe "CLI version" do
  it "should include client and server versions" do
    output = cli.run!("version")
    expect(output).to include("Client Version")
    expect(output).to include("Server Version")
  end
end

RSpec.describe "Empty system" do
  it "should list no containers" do
    resp = api.list_pods
    expect(resp).to be_kind_of(Hash)
    expect(resp).to have_key("pods")
  end
end
