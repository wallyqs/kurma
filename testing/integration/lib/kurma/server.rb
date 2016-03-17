# encoding: utf-8

require 'tmpdir'

class Kurma::Server
  attr_reader :pid
  attr_reader :tmpdir

  def initialize
  end

  def kurma_binary
    ENV["KURMAD_BINARY"] || "kurma-server"
  end

  def kurma_logfile
    ENV["KURMAD_LOGFILE"] || File.join(Dir.pwd, "log", "kurma-server.log")
  end

  def start
    @tmpdir = Dir.mktmpdir
    @pid = fork do
      logfile = File.open(self.kurma_logfile, "w+")
      Dir.chdir(@tmpdir) do
        %w( pods images volumes ).each { |d| Dir.mkdir(d) }
        $stdin.reopen("/dev/null")
        $stdout.reopen(logfile)
        $stderr.reopen(logfile)
        exec "sudo #{self.kurma_binary}"
      end
    end
    Process.detach(@pid)
    ENV["KURMA_HOST"] = File.join(@tmpdir, "kurma.sock")

    wait_for_socket
  end

  def stop
    if File.exists?("/proc/#{@pid}")
      %x{sudo kill -QUIT $(sudo cat /proc/#{@pid}/task/#{@pid}/children)}
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
