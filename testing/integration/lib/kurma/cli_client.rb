# encoding: utf-8

require 'tempfile'
require 'expect'
require 'open3'

class Kurma::CliClient
  attr_reader :user

  def initialize
  end

  def logger
    Kurma::LOG
  end

  def cli_binary
    @cli_binary ||= File.join(File.dirname(__FILE__), "..", "..", "..", "..", "bin", "kurma-cli")
  end

  # @param [String] cmd
  # @return [String, String, Process::Status]
  def run(cmd, *args)
    env = { }
    cmd_line = "#{self.cli_binary} #{cmd}"
    if cmd_line.include? " -- "
      # Command contains extended parameters, so put --show-panics before the extended parameters
      #cmd_line.sub! " -- ", " --show-panics -- "
    else
      # Append --show-panics to the end of the command
      #cmd_line.concat " --show-panics"
    end
    puts cmd_line if ENV['KURMA_DEBUG']
    logger.info { "==> #{cmd_line}" }

    stdin_data = args.empty? ? nil : args.join("\n") + "\n"

    # :binmode prevents Open3 from altering line endings based on OS.  For
    # testing on Windows you need to prevent line ending changes if sending
    # commands to capsules.  Prevents errors like `exit\r command not found`
    stdout, stderr, status = Open3.capture3(env, cmd_line, :stdin_data => stdin_data, :binmode => true)

    logger.info { "STDOUT: #{stdout}" } unless stdout.empty?
    logger.info { "STDERR: #{stderr}" } unless stderr.empty?

    [stdout, stderr, status]
  end

  # @param [String] cmd, [Map] prompts
  # @return [Int] exit_status
  def run_with_prompts!(cmd, prompts = {})
    env = { }
    cmd_line = "#{self.cli_binary} #{cmd}"
    if cmd_line.include? " -- "
      # Command contains extended parameters, so put --show-panics before the extended parameters
      #cmd_line.sub! " -- ", " --show-panics -- "
    else
      # Append --show-panics to the end of the command
      #cmd_line.concat " --show-panics"
    end
    puts cmd_line if ENV['KURMA_DEBUG']
    logger.info { "==> #{cmd_line}" }

    exit_status = 0

    Open3.popen3(env, cmd_line) do |input, output, error, wait_thr|
      pid = wait_thr.pid
      input.sync = true
      output.sync = true

      prompts.each do |k,v|
        output.expect(k.to_s, 5)
        input.puts v
      end

      exit_status = wait_thr.value
    end

    return exit_status
  end

  # @param [String] cmd
  # @return [String, String, Process::Status]
  def exec(cmd, *args)
    env = { }
    cmd_line = "#{cmd}"
    puts cmd_line if ENV['KURMA_DEBUG']
    logger.info { "==> #{cmd_line}" }

    stdin_data = args.empty? ? nil : args.join("\n") + "\n"
    stdout, stderr, status = Open3.capture3(env, cmd_line, :stdin_data => stdin_data)

    logger.info { "STDOUT: #{stdout}" } unless stdout.empty?
    logger.info { "STDERR: #{stderr}" } unless stderr.empty?

    [stdout, stderr, status]
  end

  # @param [String] cmd
  # @return [String]
  def run!(cmd, *args)
    stdout, stderr, status = run(cmd, *args)
    unless status.success?
      raise Kurma::Error, "Failed to run '#{cmd}': #{stderr}\n#{stdout}"
    end
    stdout
  end

  # Start the supplied command and return immediately with
  # stdin, stdout and stderr pipes.
  # @param [String] cmd
  def start(cmd, *args)
    env = { }
    cmd_line = "#{self.cli_binary} #{cmd}"
    logger.info { "==> #{cmd_line}" }

    stdin_data = args.empty? ? nil : args.join("\n") + "\n"
    stdin, stdout, stderr, wait_thr = Open3.popen3(env, cmd_line)

    [stdin, stdout, stderr, wait_thr]
  end
end
