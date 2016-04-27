set -x

# Update packages
dnf -y check-update

# Abort on error
set -e

# Upgrade to the latest
dnf -y upgrade

# Some needed apps/libraries
dnf -y install git libcap wget rsync
