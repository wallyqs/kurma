# encoding: utf-8
require 'spec_helper'

RSpec.describe "Container networking" do
  it "should create a resolv.conf for containers" do
    output = cli.run!("create docker://busybox --name busybox --net=host /bin/sleep 60")
    uuid = output.scan(/Launched pod ([\w-]+)/).flatten.first
    expect(uuid).not_to be_nil
    @cleanup << "stop #{uuid}"

    output = cli.run!("enter #{uuid} busybox", "cat /etc/resolv.conf", "exit")
    output.gsub!("\r", "") # trim carriage returns
    expect(output).to match(/^nameserver \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/)
  end
end
