// Copyright 2013-2015 Apcera Inc. All rights reserved.

#ifndef INITD_SERVER_MOUNT_REQUEST_C
#define INITD_SERVER_MOUNT_REQUEST_C

#include <errno.h>
#include <stdlib.h>
#include <string.h>

#include <sys/stat.h>
#include <sys/mount.h>

#include "cinitd.h"

// Documented in cinitd.h
void mount_request(struct request *r)
{
	unsigned long flags;

	// The expected protocol for a mount statement looks like this:
	// {
	//   { "MOUNT", "HOST DIRECTORY", "CONTAINER PATH" },
	//   { "FSTYPE", "FLAGS", "DATA" }
	// }

	INFO("[%d] MOUNT request.\n", r->fd);

	// Protocol error conditions.
	if (
		(r->outer_len != 2) ||
		// MOUNT
		(r->data[0][1] == NULL) ||
		(r->data[0][2] == NULL) ||
		(r->data[0][3] != NULL) ||
		// FSTYPE (nullable), FLAGS, DATA (nullable)
		(r->data[1][1] == NULL) ||
		(r->data[1][3] != NULL) ||
		// END
		(r->data[2] != NULL))
	{
		INFO("[%d] Protocol error.\n", r->fd);
		initd_response_protocol_error(r);
		return;
	}

	// Convert the flags
	flags = atol(r->data[1][1]);

	// Attempt the mount.
	if (mount(r->data[0][1], r->data[0][2], r->data[1][0], flags, r->data[1][2]) != 0) {
		ERROR("[%d] Failed to mount('%s', '%s'): %s\n", r->fd, r->data[0][1], r->data[0][2], strerror(errno));
		initd_response_internal_error(r);
		return;
	}

	// Success. Inform the caller.
	INFO("[%d] Successful mount('%s', '%s'), responding OK.\n", r->fd, r->data[0][1], r->data[0][2]);
	initd_response_request_ok(r);
}

#endif
