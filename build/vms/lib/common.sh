#!/bin/bash
# Copyright (c) 2012 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# All scripts should die on error unless commands are specifically excepted
# by prefixing with '!' or surrounded by 'set +e' / 'set -e'.

# Determine and set up variables needed for fancy color output (if supported).
V_BOLD_RED=
V_BOLD_GREEN=
V_BOLD_YELLOW=
V_REVERSE=
V_VIDOFF=

if tput colors >&/dev/null; then
  # order matters: we want VIDOFF last so that when we trace with `set -x`,
  # our terminal doesn't bleed colors as bash dumps the values of vars.
  V_BOLD_RED=$(tput bold; tput setaf 1)
  V_BOLD_GREEN=$(tput bold; tput setaf 2)
  V_BOLD_YELLOW=$(tput bold; tput setaf 3)
  V_REVERSE=$(tput rev)
  V_VIDOFF=$(tput sgr0)
fi

# Turn on bash debug support if available for backtraces.
shopt -s extdebug 2>/dev/null

# Output a backtrace all the way back to the raw invocation, suppressing
# only the _dump_trace frame itself.
_dump_trace() {
  local j n p func src line args
  p=${#BASH_ARGV[@]}
  for (( n = ${#FUNCNAME[@]}; n > 1; --n )); do
    func=${FUNCNAME[${n} - 1]}
    src=${BASH_SOURCE[${n}]##*/}
    line=${BASH_LINENO[${n} - 1]}
    args=
    if [[ -z ${BASH_ARGC[${n} -1]} ]]; then
      args='(args unknown, no debug available)'
    else
      for (( j = 0; j < ${BASH_ARGC[${n} -1]}; ++j )); do
        args="${args:+${args} }'${BASH_ARGV[$(( p - j - 1 ))]}'"
      done
      ! (( p -= ${BASH_ARGC[${n} - 1]} ))
    fi
    if [[ ${n} == ${#FUNCNAME[@]} ]]; then
      error "script called: ${0##*/} ${args}"
      error "Backtrace:  (most recent call is last)"
    else
      error "$(printf '  file %s, line %s, called: %s %s' \
               "${src}" "${line}" "${func}" "${args}")"
    fi
  done
}

# Declare these asap so that code below can safely assume they exist.
_message() {
  local prefix="$1${CROS_LOG_PREFIX:-${SCRIPT_NAME}}"
  shift
  if [[ $# -eq 0 ]]; then
    echo -e "${prefix}:${V_VIDOFF}" >&2
    return
  fi
  (
    # Handle newlines in the message, prefixing each chunk correctly.
    # Do this in a subshell to avoid having to track IFS/set -f state.
    IFS="
"
    set +f
    set -- $*
    IFS=' '
    if [[ $# -eq 0 ]]; then
      # Empty line was requested.
      set -- ''
    fi
    for line in "$@"; do
      echo -e "${prefix}: ${line}${V_VIDOFF}" >&2
    done
  )
}

info() {
  _message "${V_BOLD_GREEN}INFO    " "$*"
}

warn() {
  _message "${V_BOLD_YELLOW}WARNING " "$*"
}

error() {
  _message "${V_BOLD_RED}ERROR   " "$*"
}

function cleanup_chroot() {
    echo "Cleaning up"
    set +e
    mount | grep "${SCRIPT_ROOT}/chroot" | awk '{print $3}' | sort -r | xargs -n1 sudo umount
}

function setup_chroot() {
    if [ ! -d "${SCRIPT_ROOT}/chroot" ]; then
        echo "Setting up the chroot"
        mkdir ${SCRIPT_ROOT}/chroot
        unzip -p "${SCRIPT_ROOT}/../output/kurmaos-stage3.cntmp" PACKAGE_RESOURCE_0001.tar.gz | sudo tar xj -C "${SCRIPT_ROOT}/chroot"
        unzip -p "${SCRIPT_ROOT}/../output/kurmaos-gentoo-stage4.cntmp" PACKAGE_RESOURCE_0001.tar.gz | sudo tar xz -C "${SCRIPT_ROOT}/chroot"
    fi

    echo "Bind mounting..."
    sudo mkdir -p "${SCRIPT_ROOT}/chroot/kurmaos" \
         "${SCRIPT_ROOT}/chroot/proc" \
         "${SCRIPT_ROOT}/chroot/sys" \
         "${SCRIPT_ROOT}/chroot/dev"
    sudo mount -t proc proc "${SCRIPT_ROOT}/chroot/proc"
    sudo mount --rbind /sys "${SCRIPT_ROOT}/chroot/sys"
    sudo mount --make-rslave "${SCRIPT_ROOT}/chroot/sys"
    sudo mount --rbind /dev "${SCRIPT_ROOT}/chroot/dev"
    sudo mount --make-rslave "${SCRIPT_ROOT}/chroot/dev"
    sudo mount --bind "${SCRIPT_ROOT}/../" "${SCRIPT_ROOT}/chroot/kurmaos"
    sudo mount -t tmpfs none "${SCRIPT_ROOT}/chroot/tmp"
    fix_mtab "${SCRIPT_ROOT}/chroot"

    trap cleanup_chroot EXIT
}

# For all die functions, they must explicitly force set +eu;
# no reason to have them cause their own crash if we're inthe middle
# of reporting an error condition then exiting.
die_err_trap() {
  local command=$1 result=$2
  set +e +u

  # Per the message, bash misreports 127 as 1 during err trap sometimes.
  # Note this fact to ensure users don't place too much faith in the
  # exit code in that case.
  set -- "Command '${command}' exited with nonzero code: ${result}"
  if [[ ${result} -eq 1 ]] && [[ -z $(type -t ${command}) ]]; then
    set -- "$@" \
       '(Note bash sometimes misreports "command not found" as exit code 1 '\
'instead of 127)'
  fi
  _dump_trace
  error
  error "Command failed:"
  DIE_PREFIX='  '
  die_notrace "$@"
}

# Exit this script due to a failure, outputting a backtrace in the process.
die() {
  set +e +u
  _dump_trace
  error
  error "Error was:"
  DIE_PREFIX='  '
  die_notrace "$@"
}

# Exit this script w/out a backtrace.
die_notrace() {
  set +e +u
  if [[ $# -eq 0 ]]; then
    set -- '(no error message given)'
  fi
  local line
  for line in "$@"; do
    error "${DIE_PREFIX}${line}"
  done
  exit 1
}

# Writes stdin to the given file name as root using sudo in overwrite mode.
#
# $1 - The output file name.
sudo_clobber() {
  tee "$1" >/dev/null
}

# Writes stdin to the given file name as root using sudo in append mode.
#
# $1 - The output file name.
sudo_append() {
  tee -a "$1" >/dev/null
}

fix_mtab() {
    local root="$1" mounts="../proc/self/mounts"
    if [[ "$(readlink "${root}/etc/mtab")" != "${mounts}" ]]; then
        sudo ln -sf "${mounts}" "${root}/etc/mtab"
    fi
}

switch_to_strict_mode() {
  # Set up strict execution mode; note that the trap
  # must follow switch_to_strict_mode, else it will have no effect.
  set -e
  trap 'die_err_trap "${BASH_COMMAND:-command unknown}" "$?"' ERR
  if [[ $# -ne 0 ]]; then
    set "$@"
  fi
}
