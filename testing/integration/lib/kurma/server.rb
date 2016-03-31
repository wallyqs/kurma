# encoding: utf-8

require 'tmpdir'

class Kurma::Server
  attr_reader :pid
  attr_reader :tmpdir

  def initialize
  end

  def kurma_binary
    ENV["KURMAD_BINARY"] || File.join(File.dirname(__FILE__), "..", "..", "..", "..", "bin", "kurma-server")
  end

  def kurma_logfile
    ENV["KURMAD_LOGFILE"] || File.join(Dir.pwd, "log", "kurma-server.log")
  end

  def write_configuration(tmppath, builddir)
    cfgfile = File.join(tmppath, "kurmad.yml")
    File.open(cfgfile, "w") do |f|
      f.puts %Q{---
socketPath: #{tmppath}/kurma.sock
socketPermissions: 0666

parentCgroupName: kurma
podsDirectory: #{tmppath}/pods
imagesDirectory: #{tmppath}/images
volumesDirectory: #{tmppath}/volumes
defaultStagerImage: file:///#{builddir}/bin/stager-container.aci

prefetchImages:
- file:///#{builddir}/bin/busybox.aci

podNetworks:
- name: bridge
  aci: "file:///#{builddir}/bin/cni-netplugin.aci"
  default: true
  containerInterface: "veth+{{shortuuid}}"
  type: bridge
  bridge: bridge0
  isGateway: true
  ipMasq: true
  ipam:
    type: host-local
    subnet: 10.220.0.0/16
    routes: [ { dst: 0.0.0.0/0 } ]
      }
    end
    cfgfile
  end

  def start
    @tmpdir = Dir.mktmpdir
    builddir = File.join(File.dirname(__FILE__), "..", "..", "..", "..")
    cfgfile = self.write_configuration(@tmpdir, builddir)

    @pid = fork do
      logfile = File.open(self.kurma_logfile, "a")
      Dir.chdir(@tmpdir) do
        $stdin.reopen("/dev/null")
        $stdout.reopen(logfile)
        $stderr.reopen(logfile)
        exec "sudo #{self.kurma_binary} -configFile #{cfgfile}"
      end
    end
    Process.detach(@pid)

    ENV["KURMA_HOST"] = File.join(@tmpdir, "kurma.sock")
    wait_for_socket
  end

  def stop
    if File.exists?("/proc/#{@pid}")
      %x{sudo kill -TERM $(sudo cat /proc/#{@pid}/task/#{@pid}/children)}
      begin
        wait_for_exit
      rescue Kurma::Error
        %x{sudo kill -9 $(sudo cat /proc/#{@pid}/task/#{@pid}/children)}
        raise
      end
    end

    # FIXME may need to also check for mounts if the server failed, or has
    # children
    return if @tmpdir.nil? || @tmpdir == "" || @tmpdir == "/" || @tmpdir !~ /\A#{Dir.tmpdir}/
    %x{sudo rm -rf #{@tmpdir}}
  end

  private

  def wait_for_socket
    deadline = Time.now.to_i + 10
    loop do
      break if File.exists?(ENV["KURMA_HOST"])
      if Time.now.to_i > deadline
        raise Kurma::Error, "Failed to find socket within 10 seconds"
      end
      sleep 0.5
    end
  end

  def wait_for_exit
    deadline = Time.now.to_i + 10
    loop do
      break if !File.exists?("/proc/#{@pid}")
      if Time.now.to_i > deadline
        raise Kurma::Error, "server took too long to exit"
      end
      sleep 0.5
    end
  end
end
