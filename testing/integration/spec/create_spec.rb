# encoding: utf-8
require 'spec_helper'

RSpec.describe "CLI create" do
  it "should launch an AppC conatiner" do
    output = cli.run!("create coreos.com/etcd:v2.2.5")
    expect(output).to include("Launched container")

    resp = api.list_containers
    expect(resp["containers"].size).to eq(1)

    output = cli.run!("stop #{resp["containers"].first["uuid"]}")
    expect(output).to include("Destroyed container")

    resp = api.list_containers
    expect(resp["containers"]).to be_empty
  end

  it "should launch a Docker conatiner" do
    output = cli.run!("create docker://nats")
    expect(output).to include("Launched container")

    resp = api.list_containers
    expect(resp["containers"].size).to eq(1)

    output = cli.run!("stop #{resp["containers"].first["uuid"]}")
    expect(output).to include("Destroyed container")

    resp = api.list_containers
    expect(resp["containers"]).to be_empty
  end
end
