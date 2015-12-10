// Copyright 2013-2015 Apcera Inc. All rights reserved.

// +build linux,cgo

package capability

// #cgo linux LDFLAGS: -lcap
// #include <sys/capability.h>
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"unsafe"
)

type CapValue int

// FromName will parse the given text and return the corresponding CapValue, or
// error if it fails to parse it.
func FromName(name string) (CapValue, error) {
	return cap_from_name(name)
}

const (
	// In a system with the [_POSIX_CHOWN_RESTRICTED] option defined, this
	// overrides the restriction of changing file ownership and group
	// ownership.
	CAP_CHOWN = CapValue(int(iota))

	// Override all DAC access, including ACL execute access if
	// [_POSIX_ACL] is defined. Excluding DAC access covered by
	// CAP_LINUX_IMMUTABLE.
	CAP_DAC_OVERRIDE

	// Overrides all DAC restrictions regarding read and search on files
	// and directories, including ACL restrictions if [_POSIX_ACL] is
	// defined. Excluding DAC access covered by CAP_LINUX_IMMUTABLE.
	CAP_DAC_READ_SEARCH

	// Overrides all restrictions about allowed operations on files, where
	// file owner ID must be equal to the user ID, except where CAP_FSETID
	// is applicable. It doesn't override MAC and DAC restrictions.
	CAP_FOWNER

	// Overrides the following restrictions that the effective user ID
	// shall match the file owner ID when setting the S_ISUID and S_ISGID
	// bits on that file; that the effective group ID (or one of the
	// supplementary group IDs) shall match the file owner ID when setting
	// the S_ISGID bit on that file; that the S_ISUID and S_ISGID bits are
	// cleared on successful return from chown(2) (not implemented).
	CAP_FSETID

	// Overrides the restriction that the real or effective user ID of a
	// process sending a signal must match the real or effective user ID
	// of the process receiving the signal.
	CAP_KILL

	// * Allows setgid(2) manipulation
	// * Allows setgroups(2)
	// * Allows forged gids on socket credentials passing.
	CAP_SETGID

	// * Allows set*uid(2) manipulation (including fsuid).
	// * Allows forged pids on socket credentials passing.
	CAP_SETUID

	// Linux-specific capabilities

	// Without VFS support for capabilities:
	// * Transfer any capability in your permitted set to any pid,
	// * remove any capability in your permitted set from any pid
	//
	// With VFS support for capabilities (neither of above, but)
	// *   Add any capability from current's capability bounding set
	// *       to the current process' inheritable set
	// *   Allow taking bits out of capability bounding set
	// *   Allow modification of the securebits for a process
	CAP_SETPCAP

	// * Allow modification of S_IMMUTABLE and S_APPEND file attributes
	CAP_LINUX_IMMUTABLE

	// * Allows binding to TCP/UDP sockets below 1024
	// * Allows binding to ATM VCIs below 32
	CAP_NET_BIND_SERVICE

	// * Allow broadcasting, listen to multicast
	CAP_NET_BROADCAST

	// * Allow interface configuration
	// * Allow administration of IP firewall, masquerading and accounting
	// * Allow setting debug option on sockets
	// * Allow modification of routing tables
	// * Allow setting arbitrary process / process group ownership on sockets
	// * Allow binding to any address for transparent proxying
	//   (also via NET_RAW)
	// * Allow setting TOS (type of service)
	// * Allow setting promiscuous mode
	// * Allow clearing driver statistics
	// * Allow multicasting
	// * Allow read/write of device-specific registers
	// * Allow activation of ATM control sockets
	CAP_NET_ADMIN

	// * Allow use of RAW sockets
	// * Allow use of PACKET sockets
	// * Allow binding to any address for transparent proxying
	//   (also via NET_ADMIN)
	CAP_NET_RAW

	// * Allow blocking of shared memory segments
	// * Allow mlock and mlockall (which doesn't really have anything to do
	//   with IPC)
	CAP_IPC_LOCK

	// * Override IPC ownership checks
	CAP_IPC_OWNER

	// * Insert and remove kernel modules - modify kernel without limit
	CAP_SYS_MODULE

	// * Allow ioperm/iopl access
	// * Allow sending USB messages to any device via /proc/bus/usb
	CAP_SYS_RAWIO

	// * Allow use of chroot()
	CAP_SYS_CHROOT

	// * Allow ptrace() of any process
	CAP_SYS_PTRACE

	// * Allow configuration of process accounting
	CAP_SYS_PACCT

	// * Allow configuration of the secure attention key
	// * Allow administration of the random device
	// * Allow examination and configuration of disk quotas
	// * Allow setting the domainname
	// * Allow setting the hostname
	// * Allow calling bdflush()
	// * Allow mount() and umount(), setting up new smb connection
	// * Allow some autofs root ioctls
	// * Allow nfsservctl
	// * Allow VM86_REQUEST_IRQ
	// * Allow to read/write pci config on alpha
	// * Allow irix_prctl on mips (setstacksize)
	// * Allow flushing all cache on m68k (sys_cacheflush)
	// * Allow removing semaphores
	// * Used instead of CAP_CHOWN to "chown" IPC message queues, semaphores
	//   and shared memory
	// * Allow locking/unlocking of shared memory segment
	// * Allow turning swap on/off
	// * Allow forged pids on socket credentials passing
	// * Allow setting readahead and flushing buffers on block devices
	// * Allow setting geometry in floppy driver
	// * Allow turning DMA on/off in xd driver
	// * Allow administration of md devices (mostly the above, but some
	//   extra ioctls)
	// * Allow tuning the ide driver
	// * Allow access to the nvram device
	// * Allow administration of apm_bios, serial and bttv (TV) device
	// * Allow manufacturer commands in isdn CAPI support driver
	// * Allow reading non-standardized portions of pci configuration space
	// * Allow DDI debug ioctl on sbpcd driver
	// * Allow setting up serial ports
	// * Allow sending raw qic-117 commands
	// * Allow enabling/disabling tagged queuing on SCSI controllers and sending
	//   arbitrary SCSI commands
	// * Allow setting encryption key on loopback filesystem
	// * Allow setting zone reclaim policy
	CAP_SYS_ADMIN

	// * Allow use of reboot()
	CAP_SYS_BOOT

	// * Allow raising priority and setting priority on other (different
	//   UID) processes
	// * Allow use of FIFO and round-robin (realtime) scheduling on own
	//   processes and setting the scheduling algorithm used by another
	//   process.
	// * Allow setting cpu affinity on other processes

	CAP_SYS_NICE

	// * Override resource limits. Set resource limits.
	// * Override quota limits.
	// * Override reserved space on ext2 filesystem
	// * Modify data journaling mode on ext3 filesystem (uses journaling
	// * resources)
	// * NOTE: ext2 honors fsuid when checking for resource overrides, so
	// * you can override using fsuid too
	// * Override size restrictions on IPC message queues
	// * Allow more than 64hz interrupts from the real-time clock
	// * Override max number of consoles on console allocation
	// * Override max number of keymaps
	CAP_SYS_RESOURCE

	// * Allow manipulation of system clock
	// * Allow irix_stime on mips
	// * Allow setting the real-time clock
	CAP_SYS_TIME

	// * Allow configuration of tty devices
	// * Allow vhangup() of tty
	CAP_SYS_TTY_CONFIG

	// * Allow the privileged aspects of mknod()
	CAP_MKNOD

	// * Allow taking of leases on files
	CAP_LEASE

	CAP_AUDIT_WRITE

	CAP_AUDIT_CONTROL

	CAP_SETFCAP

	// Override MAC access.
	// The base kernel enforces no MAC policy.
	// An LSM may enforce a MAC policy, and if it does and it chooses
	// to implement capability based overrides of that policy, this is
	// the capability it should use to do so.
	CAP_MAC_OVERRIDE

	// Allow MAC configuration or state changes.
	// The base kernel requires no MAC configuration.
	// An LSM may enforce a MAC policy, and if it does and it chooses
	// to implement capability based checks on modifications to that
	// policy or the data required to maintain it, this is the
	// capability it should use to do so.
	CAP_MAC_ADMIN

	// * Allow configuring the kernel's syslog (printk behaviour)
	CAP_SYSLOG

	// This is the marker for the last capability in the list
	CAP_LAST
)

// Returns the cap_t type for the given flag.
func (f CapValue) capT() C.cap_value_t {
	switch f {
	case CAP_CHOWN:
		return C.CAP_CHOWN
	case CAP_DAC_OVERRIDE:
		return C.CAP_DAC_OVERRIDE
	case CAP_DAC_READ_SEARCH:
		return C.CAP_DAC_READ_SEARCH
	case CAP_FOWNER:
		return C.CAP_FOWNER
	case CAP_FSETID:
		return C.CAP_FSETID
	case CAP_KILL:
		return C.CAP_KILL
	case CAP_SETGID:
		return C.CAP_SETGID
	case CAP_SETUID:
		return C.CAP_SETUID
	case CAP_SETPCAP:
		return C.CAP_SETPCAP
	case CAP_LINUX_IMMUTABLE:
		return C.CAP_LINUX_IMMUTABLE
	case CAP_NET_BIND_SERVICE:
		return C.CAP_NET_BIND_SERVICE
	case CAP_NET_BROADCAST:
		return C.CAP_NET_BROADCAST
	case CAP_NET_ADMIN:
		return C.CAP_NET_ADMIN
	case CAP_NET_RAW:
		return C.CAP_NET_RAW
	case CAP_IPC_LOCK:
		return C.CAP_IPC_LOCK
	case CAP_IPC_OWNER:
		return C.CAP_IPC_OWNER
	case CAP_SYS_MODULE:
		return C.CAP_SYS_MODULE
	case CAP_SYS_RAWIO:
		return C.CAP_SYS_RAWIO
	case CAP_SYS_CHROOT:
		return C.CAP_SYS_CHROOT
	case CAP_SYS_PTRACE:
		return C.CAP_SYS_PTRACE
	case CAP_SYS_PACCT:
		return C.CAP_SYS_PACCT
	case CAP_SYS_ADMIN:
		return C.CAP_SYS_ADMIN
	case CAP_SYS_BOOT:
		return C.CAP_SYS_BOOT
	case CAP_SYS_NICE:
		return C.CAP_SYS_NICE
	case CAP_SYS_RESOURCE:
		return C.CAP_SYS_RESOURCE
	case CAP_SYS_TIME:
		return C.CAP_SYS_TIME
	case CAP_SYS_TTY_CONFIG:
		return C.CAP_SYS_TTY_CONFIG
	case CAP_MKNOD:
		return C.CAP_MKNOD
	case CAP_LEASE:
		return C.CAP_LEASE
	case CAP_AUDIT_WRITE:
		return C.CAP_AUDIT_WRITE
	case CAP_AUDIT_CONTROL:
		return C.CAP_AUDIT_CONTROL
	case CAP_SETFCAP:
		return C.CAP_SETFCAP
	case CAP_MAC_OVERRIDE:
		return C.CAP_MAC_OVERRIDE
	case CAP_MAC_ADMIN:
		return C.CAP_MAC_ADMIN
	case CAP_SYSLOG:
		return C.CAP_SYSLOG
		//	case CAP_WAKE_ALARM:
		//		return C.CAP_WAKE_ALARM
		//	case CAP_BLOCK_SUSPEND:
		//		return C.CAP_BLOCK_SUSPEND
	}
	panic("UNKNOWN CAPABILITY TYPE")
}

type Cap struct {
	data C.cap_t
}

// isset returns whether the specified capabilities is in the given capability
// set.
func (cap *Cap) isset(value CapValue, flag C.cap_flag_t) (bool, error) {
	var dest C.cap_flag_value_t
	n, err := C.cap_get_flag(cap.data, value.capT(), flag, &dest)
	if err != nil {
		return false, err
	}

	if n != 0 {
		return false, fmt.Errorf("Unknown error returned from cap_get_flag: %d", n)
	}

	if dest == C.CAP_SET {
		return true, nil
	}
	return false, nil
}

func (cap *Cap) set(value CapValue, flag C.cap_flag_t, set C.cap_flag_value_t) error {
	cap_value := value.capT()
	ret := C.cap_set_flag(cap.data, flag, 1, &cap_value, set)
	if ret != 0 {
		return fmt.Errorf("failed to set the desired value")
	}
	return nil
}

// EffectiveCapability returns true if the given capability (value) in its
// effective set of capabilities. This means that the process is actively being
// allowed the permissions that the capability grants.
func (cap *Cap) EffectiveCapability(flag CapValue) (bool, error) {
	return cap.isset(flag, C.CAP_EFFECTIVE)
}

// PermittedCapabilityr eturns true if the given capability (value) in its
// permitted set of capabilities.
func (cap *Cap) PermittedCapability(flag CapValue) (bool, error) {
	return cap.isset(flag, C.CAP_PERMITTED)
}

// InheritableCapability returns true if the given capability (value) in its
// permitted set of capabilities.
func (cap *Cap) InheritableCapability(flag CapValue) (bool, error) {
	return cap.isset(flag, C.CAP_INHERITABLE)
}

// SetEffectiveCapability will enable the specified capability in the effective
// set.
func (cap *Cap) SetEffectiveCapability(flag CapValue, set bool) error {
	if set {
		return cap.set(flag, C.CAP_EFFECTIVE, C.CAP_SET)
	}
	return cap.set(flag, C.CAP_EFFECTIVE, C.CAP_CLEAR)
}

// SetPermittedCapability will enable the specified capability in the permitted
// set.
func (cap *Cap) SetPermittedCapability(flag CapValue, set bool) error {
	if set {
		return cap.set(flag, C.CAP_PERMITTED, C.CAP_SET)
	}
	return cap.set(flag, C.CAP_PERMITTED, C.CAP_CLEAR)
}

// SetInheritableCapability will enable the specified capability in the
// inheritable set.
func (cap *Cap) SetInheritableCapability(flag CapValue, set bool) error {
	if set {
		return cap.set(flag, C.CAP_INHERITABLE, C.CAP_SET)
	}
	return cap.set(flag, C.CAP_INHERITABLE, C.CAP_CLEAR)
}

// Free will release the current capability object, releasing the memory.
func (cap *Cap) Free() {
	C.cap_free(unsafe.Pointer(cap.data))
}

// Clear will reset all capabilities.
func (cap *Cap) Clear() {
	C.cap_clear(cap.data)
}

// String will convert the current capabilities set into a string representation
// using cap_to_text().
func (cap *Cap) String() string {
	s, _ := cap_to_text(cap)
	return s
}

// NewFromPid will return a new capabilities set for the process specified by
// pid.
func NewFromPid(pid int) (*Cap, error) {
	return cap_get_pid(pid)
}

func cap_get_pid(pid int) (*Cap, error) {
	cap_t, err := C.cap_get_pid(C.pid_t(pid))
	if err != nil {
		return nil, err
	}
	return &Cap{data: cap_t}, nil
}

func cap_to_text(cap *Cap) (string, error) {
	cString := C.cap_to_text(cap.data, nil)
	if cString == nil {
		return "", fmt.Errorf("failed to convert capabilities to text")
	}
	defer C.cap_free(unsafe.Pointer(cString))
	return C.GoString(cString), nil
}

func cap_from_name(name string) (CapValue, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var capv C.cap_value_t
	ret := C.cap_from_name(cname, &capv)
	if ret != 0 {
		return CapValue(0), fmt.Errorf("failed to parse the capability name")
	}
	return CapValue(int(capv)), nil
}
