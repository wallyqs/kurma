// Copyright 2015 Apcera Inc. All rights reserved.
//
// Simple executable to write to the sysreq-trigger handler to shutdown or
// reboot the host from a privileged container.

#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

void writesysreq(char c) {
	sync();

	int fd = open("/host/proc/sysrq-trigger", O_WRONLY);
	if (fd == -1) {
		perror("open");
		exit(1);
	}
	if (write(fd, &c, 1) < 0) {
		perror("write");
		exit(1);
	}
	if (close(fd) == -1) {
		perror("close");
		exit(1);
	}
}

void poweroff() {
	writesysreq('o');
}

void reboot() {
	writesysreq('b');
}

int main(int argc, char **argv) {
	switch (argv[0][0]) {
	case 'h':
		poweroff();
		break;
	case 'p':
		poweroff();
		break;
	case 'r':
		reboot();
		break;
	default:
		fprintf(stderr, "unrecognized command\n");
		exit(1);
	}
}
