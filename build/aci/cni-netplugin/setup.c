// Copyright 2016 Apcera Inc. All rights reserved.

#include <unistd.h>
#include <stdlib.h>

int main(int argc, char **argv) {
	// Close stdin for the configuration.
	close(0);

	// Just wait...
	pause();
	exit(0);
}
