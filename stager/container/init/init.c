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

// This function will install signal_sigchld (above) as the signal handler for
// SIGCHLD signals.
static void setup_signal_handler(void)
{
	struct sigaction sigchld_handler;

	// Zero out the structure.
	memset(&sigchld_handler, 0, sizeof(struct sigaction));

	// Set the required values.
	sigchld_handler.sa_handler = signal_sigchld;
	sigchld_handler.sa_flags = 0;
	if (sigemptyset(&sigchld_handler.sa_mask)) {
		error(1, errno, "Error in sigemptyset()");
	}
	if (sigaction(SIGCHLD, &sigchld_handler, NULL)) {
		error(1, errno, "Error in sigaction()");
	}
}

int main(int argc, char **argv) {
	// Configure the signal handler.
	setup_signal_handler();

	// Just wait...
	pause();
	exit(0);
}
