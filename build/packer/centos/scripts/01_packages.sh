set -x

# Update packages
yum -y check-update

# Abort on error
set -e

# Upgrade to the latest
yum -y upgrade

# Some needed apps/libraries
yum -y install git libcap wget rsync
