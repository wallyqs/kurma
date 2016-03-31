# encoding: utf-8
require 'spec_helper'

RSpec.describe "CLI create" do
  it "should launch an AppC conatiner" do
    initial_pods_count = api.list_pods["pods"].size

    output = cli.run!("create coreos.com/etcd:v2.2.5")
    uuid = output.scan(/Launched pod ([\w-]+)/).flatten.first
    expect(uuid).not_to be_nil

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count+1)

    output = cli.run!("stop #{uuid}")
    expect(output).to include("Destroyed pod")

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count)
  end

  it "should launch a Docker conatiner" do
    initial_pods_count = api.list_pods["pods"].size

    output = cli.run!("create docker://nats")
    uuid = output.scan(/Launched pod ([\w-]+)/).flatten.first
    expect(uuid).not_to be_nil

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count+1)

    output = cli.run!("stop #{uuid}")
    expect(output).to include("Destroyed pod")

    resp = api.list_pods
    expect(resp["pods"].size).to eq(initial_pods_count)
  end

  it "should enter a container and run a command" do
    output = cli.run!("create docker://busybox --name busybox --net=host /bin/sleep 60")
    uuid = output.scan(/Launched pod ([\w-]+)/).flatten.first
    expect(uuid).not_to be_nil

    output = cli.run!("enter #{uuid} busybox", "whoami", "exit")
    output.gsub!("\r", "") # trim carriage returns
    expect(output).to match("whoami\nroot")

    output = cli.run!("stop #{uuid}")
    expect(output).to include("Destroyed pod")
  end
end
