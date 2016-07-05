FROM debian

RUN apt-get update -y --fix-missing && apt-get install -y ruby-dev gcc make rubygems

RUN gem install fpm

# This will be mounted in for flexibility in the fpm command for systemd and upstart.
ENTRYPOINT /entrypoint.sh
