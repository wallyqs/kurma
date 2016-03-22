# encoding: utf-8
require 'spec_helper'

RSpec.describe "CLI create" do
  it "should launch an AppC conatiner" do
    initial_pods_count = api.list_pods["pods"].size

    output = cli.run!("create coreos.com/etcd:v2.2.5")
    expect(output).to include("Launched pod")

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count+1)

    output = cli.run!("stop #{resp["pods"].first["uuid"]}")
    expect(output).to include("Destroyed pod")

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count)
  end

  it "should launch a Docker conatiner" do
    initial_pods_count = api.list_pods["pods"].size

    output = cli.run!("create docker://nats")
    expect(output).to include("Launched pod")

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count+1)

    output = cli.run!("stop #{resp["pods"].first["uuid"]}")
    expect(output).to include("Destroyed pod")

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count)
  end
end
