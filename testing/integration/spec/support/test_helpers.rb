# encoding: utf-8

require 'rspec'

module TestHelpers
  extend RSpec::SharedContext

  def log
    Kurma::LOG
  end

  def cli
    Thread.current[:cli] ||= Kurma::CliClient.new
  end

  def api
    Thread.current[:api] ||= Kurma::ApiClient.new
  end

  def random(n = 10)
    SecureRandom.uuid[0..n]
  end

  # Call a block of rspec tests multiple times until either the block
  # succeeds, or the configured timeout (default 10 seconds) occurs.  Delay by the configured
  # delay (default 1 second) between passes.
  def wait_until(condition_msg, timeout_s = 10, delay = 1)
    unless block_given?
      raise ArgumentError, "'wait_until' should be called with block"
    end

    deadline = Time.now.to_i + timeout_s
    begin
      val = yield
      return val if val
    rescue Net::OpenTimeout, Net::ReadTimeout, RSpec::Expectations::ExpectationNotMetError, Kurma::Error, EOFError, Errno::ECONNRESET, Errno::ECONNREFUSED => e
      if Time.now.to_i >= deadline
        raise TimeoutError, "Timed out waiting for '#{condition_msg}' after #{timeout_s} seconds with error: #{e.message}"
      end

      sleep(delay)
      retry
    end
  end

  # Run a command locally, logging output and making it easy to send multiple
  # inputs to STDIN
  def exec(cmd, *args)
    Kurma::LOG.info { "Executing: #{cmd}" }
    Kurma::LOG.info { "ENV: #{ENV.inspect}" }

    stdin_data = args.empty? ? nil : args.join("\n") + "\n"
    stdout, stderr, status = Open3.capture3(ENV, cmd, :stdin_data => stdin_data)

    Kurma::LOG.info { "STDOUT: #{stdout}" }
    Kurma::LOG.info { "STDERR: #{stderr}" }

    [stdout, stderr, status]
  end

  # @param [String] cmd
  # @return [String]
  def exec!(cmd, *args)
    stdout, stderr, status = exec(cmd, *args)
    unless status.success?
      raise Kurma::Error, "Failed to run '#{cmd}': #{stderr}"
    end
    stdout
  end

  # APC presents and expects resources in MB/mbps, but server-side receives them
  # in B/bps.
  BYTE_PER_KB = 1024
  BIT_PER_KBIT = 1000
  KBIT_PER_MBIT = 1000

  def mb_from_bytes(bytes)
    bytes / (BYTE_PER_KB * BYTE_PER_KB)
  end

  def bytes_from_mb(mb)
    mb * BYTE_PER_KB * BYTE_PER_KB
  end

  def bps_from_kbps(kbps)
    kbps / BIT_PER_KBIT
  end

  def bps_from_mbps(mbps)
    mbps * BIT_PER_KBIT * KBIT_PER_MBIT
  end

  def run(cmd, *args)
    log.info { "==> #{cmd}" }
    stdin_data = args.empty? ? nil : args.join("\n") + "\n"
    stdout, stderr, status = Open3.capture3(cmd, :stdin_data => stdin_data)

    log.info { "STDOUT: #{stdout}" } unless stdout.empty?
    log.info { "STDERR: #{stderr}" } unless stderr.empty?

    [stdout, stderr, status]
  end

  def run!(cmd, *args)
    stdout, stderr, status = run(cmd, *args)
    unless status.success?
      raise Kurma::Error, "Failed to run '#{cmd}': #{stderr}"
    end
    stdout
  end
end
