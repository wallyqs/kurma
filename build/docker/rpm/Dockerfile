FROM centos

RUN yum update -y && yum install -y ruby-devel gcc make rpm-build

RUN gem install fpm

# the entrypoint script will be mounted as a volume for flexibility
ENTRYPOINT /entrypoint.sh