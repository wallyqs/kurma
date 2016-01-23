// Copyright 2014-2015 Apcera Inc. All rights reserved.
//
// Portions of this file are based on code from:
//   https://github.com/arachsys/containers
//
// Code is licensed under MIT.
// Copyright 2013 Chris Webb <chris@arachsys.com>

#ifndef INITD_SPAWNER_MOUNT_C
#define INITD_SPAWNER_MOUNT_C

#define _GNU_SOURCE

#include <errno.h>
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/types.h>

#include "spawner.h"

static char *root;

char *tmpdir(void) {
	char *dir;

	if (!(dir = strdup("/tmp/XXXXXX")))
		error(1, errno, "strdup");
	else if (!mkdtemp(dir))
		error(1, errno, "Failed to create temporary directory");
	return dir;
}

void bindnode(char *src, char *dst) {
	int fd;

	if ((fd = open(dst, O_WRONLY | O_CREAT, 0600)) >= 0)
		close(fd);
	if (mount(src, dst, NULL, MS_BIND, NULL) < 0)
		error(1, errno, "Failed to bind %s into new %s filesystem", src, dst);
}

void createroot(char *src, char *dst, bool privileged) {
	mode_t mask;
	pid_t child;
	int res;
	int console;

	mask = umask(0);

	// Convert the root mount into a private mount. This will ensures going
	// forward, all changes are private.
	if (mount("none", "/", NULL, MS_REC | MS_PRIVATE, NULL) < 0)
		error(1, errno, "Failed to convert new mount namespace to be private");

	// Create /tmp since this is typically where the container's bind location
	// will be, and helps with making SSH work for Continuum capsules.
	mkdir("/tmp", 0755);
	if (mount("tmpfs", "/tmp", "tmpfs", 0, "mode=0755") < 0)
	  error(1, errno, "Failed to mount /tmp tmpfs in parent filesystem");

	// Typically the dst is passed in, however fall back on handling to create a
	// tmpdir and clean it up. This is primarily for localized testing of the
	// spawner itself.
	if (dst) {
		mkdir(dst, 0755);
		root = dst;
	} else {
		root = tmpdir();
	}

	// Mount the source to the root temp directory.
	if (mount(src, root, NULL, MS_BIND | MS_REC, NULL) < 0)
		error(1, errno, "Failed to bind new root filesystem");
	else if (chdir(root) < 0)
		error(1, errno, "Failed to enter new root filesystem");

	// Setup /dev as tmpfs mounts within the container
	mkdir("dev" , 0755);
	if (privileged) {
		if (mount("devtmpfs", "dev", "devtmpfs", 0, "") < 0)
			error(1, errno, "Failed to mount /dev devtmpfs in new root filesystem");

		// ptmx is still symlinked regardless of privileged mode. This is because
		// the devpts within the container is a new instance, so /dev/ptmx can't be
		// inherited from devtmpfs or the host.
		unlink("dev/ptmx");
		res = symlink("pts/ptmx", "dev/ptmx");
	} else {
		if (mount("tmpfs", "dev", "tmpfs", MS_NOEXEC | MS_STRICTATIME, "mode=0755") < 0)
			error(1, errno, "Failed to mount /dev tmpfs in new root filesystem");

		// Populate /dev within the container
		bindnode("/dev/full", "dev/full");
		bindnode("/dev/null", "dev/null");
		bindnode("/dev/random", "dev/random");
		bindnode("/dev/tty", "dev/tty");
		bindnode("/dev/urandom", "dev/urandom");
		bindnode("/dev/zero", "dev/zero");

		// Check for the presence of some devices that might not always be there.
		if(access("/dev/fuse", F_OK) == 0)
			bindnode("/dev/fuse", "dev/fuse");

		res = symlink("pts/ptmx", "dev/ptmx");
		res = symlink("/proc/kcore", "dev/core");
		res = symlink("/proc/self/fd", "dev/fd");
		res = symlink("console", "dev/kmsg");

		res = symlink("fd/0", "dev/stdin");
		res = symlink("fd/1", "dev/stdout");
		res = symlink("fd/2", "dev/stderr");
	}

	// setup /dev/mqueue, /dev/pts and /dev/shm
	mkdir("dev/mqueue", 0755);
	if (mount("mqueue", "dev/mqueue", "mqueue", MS_NOEXEC | MS_NOSUID | MS_NODEV, NULL) < 0)
		error(1, errno, "Failed to mount /dev/mqueue in new root filesystem");
	mkdir("dev/pts", 0755);
	if (mount("devpts", "dev/pts", "devpts", MS_NOEXEC | MS_NOSUID, "newinstance,ptmxmode=0666") < 0)
		error(1, errno, "Failed to mount /dev/pts in new root filesystem");
	mkdir("dev/shm", 0755);
	if (mount("tmpfs", "dev/shm", "tmpfs", MS_NOEXEC | MS_NOSUID | MS_NODEV, "mode=1777,size=65536k") < 0)
		error(1, errno, "Failed to mount /dev/shm in new root filesystem");

	// Setup /tmp within the container
	mkdir("tmp", 0777);
	umask(mask);
}

void enterroot() {
	if (chdir(root) < 0)
		error(1, errno, "Failed to chdir into the new root");
	if (mount (root, root, NULL, MS_BIND | MS_REC, NULL) < 0)
		error(1, errno, "Failed to bind mount the root");
	if (mount (root, "/", NULL, MS_MOVE, NULL) < 0)
		error(1, errno, "Failed to move the root");
	if (chroot (".") < 0)
		error(1, errno, "Failed to chroot");
	if (chdir("/") < 0 )
		error(1, errno, "Failed to chdir into /");
}

void mountproc(void) {
	mode_t mask;

	mask = umask(0);
	mkdir("proc" , 0755);
	mkdir("sys", 0755);
	umask(mask);

	if (mount("proc", "proc", "proc", MS_NOSUID | MS_NOEXEC | MS_NODEV, NULL) < 0)
		error(1, errno, "Failed to mount /proc in new root filesystem: %s", strerror(errno));
	if (mount("sysfs", "sys", "sysfs", MS_NOEXEC | MS_NOSUID | MS_NODEV | MS_RDONLY, NULL) < 0)
		error(1, errno, "Failed to mount /sys in new root filesystem");
}

#endif
