# Abort on error
set -e -x

# Sleep to ensure cloud-init has populated the apt sources and written them
# before we continue. Otherwise, can get odd behavior.
sleep 60

# Update packages
apt-get --yes --force-yes update

# http://askubuntu.com/questions/146921/how-do-i-apt-get-y-dist-upgrade-without-a-grub-config-prompt
# Core problem: post-install scripts don't care that we told apt-get --yes/--force-yes
DEBIAN_FRONTEND=noninteractive
UCF_FORCE_CONFFNEW=yes
export DEBIAN_FRONTEND UCF_FORCE_CONFFNEW
ucf --purge /boot/grub/menu.lst
apt-get -o Dpkg::Options::="--force-confnew" --force-yes -fuy dist-upgrade

# Some needed apps/libraries
apt-get -y install git libcap2 rsync
