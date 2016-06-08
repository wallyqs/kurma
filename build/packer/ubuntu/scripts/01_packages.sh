# Abort on error
set -e -x

# Sleep to ensure cloud-init has populated the apt sources and written them
# before we continue. Otherwise, can get odd behavior.
sleep 60

# Get the codename and update the apt sources. This is to add in the universe
# set of pacakges.
ubuntuCodename=$(lsb_release -s -c)
echo "deb http://us.archive.ubuntu.com/ubuntu/ $ubuntuCodename main restricted universe" > /etc/apt/sources.list
echo "deb http://us.archive.ubuntu.com/ubuntu/ $ubuntuCodename-security main restricted universe" >> /etc/apt/sources.list
echo "deb http://us.archive.ubuntu.com/ubuntu/ $ubuntuCodename-updates main restricted universe" >> /etc/apt/sources.list

# Update packages
apt-get --yes --force-yes update

# http://askubuntu.com/questions/146921/how-do-i-apt-get-y-dist-upgrade-without-a-grub-config-prompt
# Core problem: post-install scripts don't care that we told apt-get --yes/--force-yes
DEBIAN_FRONTEND=noninteractive
UCF_FORCE_CONFFNEW=yes
export DEBIAN_FRONTEND UCF_FORCE_CONFFNEW
ucf --purge /boot/grub/menu.lst
apt-get -o Dpkg::Options::="--force-confnew" --force-yes -fuy dist-upgrade

# Install the 3.13 kernel image for Ubuntu precise. This is required for minimum
# kernel version support.
if [[ "$ubuntuCodename" == "precise" ]]; then
  # Forcibly remove all existing kernel packages from the system.
  dpkg -l | grep linux-image | awk '{print $2}' | xargs apt-get purge -y --force-yes

  # Install the trusty backport kernel.
  apt-get install -y linux-image-generic-lts-trusty
else
  # Install the specific needed linux-image-extra to get aufs
  extraPkg=$(dpkg -l | grep linux-image | grep -v linux-image-virtual | grep -v lts | awk '{print $2}' | sed -e 's#linux-image#linux-image-extra#g')
  apt-get -y install $extraPkg
fi

# Some needed apps/libraries
apt-get -y install git libcap2 rsync

# Install cgroup-lite on trusty and precise to handle cgroups mounts on startup.
if [[ "$ubuntuCodename" == "precise" || "$ubuntuCodename" == "trusty" ]]; then
  apt-get -y install cgroup-lite
fi
