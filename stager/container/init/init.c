// Copyright 2016 Apcera Inc. All rights reserved.

#include <errno.h>
#include <error.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// This function is called when a SIGCHLD signal is received.
static void signal_sigchld(int sig) {
	return;
}

// This function is called when a SIGTERM or SIGINT signal is received.
static void signal_sigterm(int sig) {
	exit(0);
}

// This function will install signal_sigchld (above) as the signal handler for
// SIGCHLD signals.
static void setup_signal_handler(void)
{
	struct sigaction sigchld_handler;
	struct sigaction sigterm_handler;

	// Zero out the structures.
	memset(&sigchld_handler, 0, sizeof(struct sigaction));
	memset(&sigterm_handler, 0, sizeof(struct sigaction));

	// Set up sigchld.
	sigchld_handler.sa_handler = signal_sigchld;
	sigchld_handler.sa_flags = 0;
	if (sigemptyset(&sigchld_handler.sa_mask)) {
		error(1, errno, "Error in sigemptyset() for sigchld");
	}
	if (sigaction(SIGCHLD, &sigchld_handler, NULL)) {
		error(1, errno, "Error in sigaction() for sigchld");
	}

	// Set up sigterm and sigint.
	sigterm_handler.sa_handler = signal_sigterm;
	sigterm_handler.sa_flags = 0;
	if (sigemptyset(&sigterm_handler.sa_mask)) {
		error(1, errno, "Error in sigemptyset() for sigterm");
	}
	if (sigaction(SIGTERM, &sigterm_handler, NULL)) {
		error(1, errno, "Error in sigaction() for sigterm");
	}
	if (sigaction(SIGINT, &sigterm_handler, NULL)) {
		error(1, errno, "Error in sigaction() for sigint");
	}
}

int main(int argc, char **argv) {
	// Configure the signal handler.
	setup_signal_handler();

	// Just wait...
	pause();
	exit(0);
}
