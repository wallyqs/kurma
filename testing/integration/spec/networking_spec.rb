# encoding: utf-8
require 'spec_helper'

RSpec.describe "Container networking" do
  context "with host networking" do
    it "should create a resolv.conf for containers" do
      output = cli.run!("create docker://busybox --name busybox --net=host /bin/sleep 60")
      uuid = output.scan(/Launched pod ([\w-]+)/).flatten.first
      expect(uuid).not_to be_nil
      @cleanup << "stop #{uuid}"

      # validate it has a resolv.conf with a nameserver entry and a valid IP
      output = cli.run!("enter #{uuid} busybox", "cat /etc/resolv.conf", "exit")
      output.gsub!("\r", "") # trim carriage returns
      expect(output).to match(/^nameserver \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/)
    end
  end

  context "with isolated networking" do
    context "bridge" do
      it "should create a resolv.conf for containers" do
        output = cli.run!("create docker://busybox --name busybox --net=bridge /bin/sleep 60")
        uuid = output.scan(/Launched pod ([\w-]+)/).flatten.first
        expect(uuid).not_to be_nil
        @cleanup << "stop #{uuid}"

        # retrieve the pod object and validate it has a network entry and
        # matching an IP in the bridge network's range
        pod = api.get_pod(uuid)
        expect(pod).to be_kind_of(Hash)
        puts pod["pod"]["networks"].inspect
        expect(pod["pod"]["networks"].size).to be(1)
        expect(pod["pod"]["networks"][0]["ip4"]["ip"]).to match(/^10\.220\.0\.\d{1,3}\/16/)

        # validate it has a resolv.conf with a nameserver entry and a valid IP
        output = cli.run!("enter #{uuid} busybox", "cat /etc/resolv.conf", "exit")
        output.gsub!("\r", "") # trim carriage returns
        expect(output).to match(/^nameserver \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/)
      end
    end
  end
end
