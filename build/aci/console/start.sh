#!/bin/sh

set -e

for i in rsa dsa ecdsa ed25519; do
  local keyfile=/etc/ssh/ssh_host_${i}_key
  if [ ! -e $keyfile ]; then
    ssh-keygen -f $keyfile -N '' -t $i
  fi
done
mkdir -p /var/run/sshd

if [ -n "$CONSOLE_PASSWORD" ]; then
  echo "root:$CONSOLE_PASSWORD" | chpasswd
fi

if [ -n "$CONSOLE_KEYS" ]; then
  mkdir -p /root/.ssh
  chmod 700 /root/.ssh
  echo "$CONSOLE_KEYS" > /root/.ssh/authorized_keys
  chmod 600 /root/.ssh/authorized_keys
else
  # FIXME only do this for now when keys aren't available, ideally should inform
  # the user on the console.
  echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
fi

rm /start.sh

exec /sbin/spawn -f /etc/spawn.conf
