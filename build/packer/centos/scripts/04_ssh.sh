#!/bin/bash

set -e -x

echo 'Defaults:centos !requiretty' > /etc/sudoers.d/999-vagrant-cloud-init-requiretty
chmod 440 /etc/sudoers.d/999-vagrant-cloud-init-requiretty

sed -i'.bk' -e 's/^\(Defaults\s\+requiretty\)/# \1/' /etc/sudoers
