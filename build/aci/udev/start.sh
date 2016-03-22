#!/bin/sh

set -e -x

udevd --daemon
udevadm trigger --action=add
udevadm settle
