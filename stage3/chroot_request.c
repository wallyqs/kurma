// Copyright 2013-2015 Apcera Inc. All rights reserved.

#ifndef INITD_SERVER_CHROOT_REQUEST_C
#define INITD_SERVER_CHROOT_REQUEST_C

#include <errno.h>
#include <string.h>

#include "cinitd.h"

// Documented in cinitd.h
void chroot_request(struct request *r)
{
	// The expected protocol for a chroot statement looks like this:
	// {
	//   { "CHROOT", "DIRECTORY" },
	// }

	INFO("[%d] CHROOT request.\n", r->fd);

	// Protocol error conditions.
	if (
		(r->outer_len != 1) ||
		// CHROOT
		(r->data[0][1] == NULL) ||
		(r->data[0][2] != NULL) ||
		// END
		(r->data[1] != NULL))
	{
		INFO("[%d] Protocol error.\n", r->fd);
		initd_response_protocol_error(r);
		return;
	}

	// Attempt the actual chroot.
	if (moveroot(r->data[0][1]) != 0) {
		ERROR("[%d] Failed to chroot('%s'): %s\n", r->fd, r->data[0][1], strerror(errno));
		initd_response_internal_error(r);
		return;
	}

	// Success. Inform the caller.
	INFO("[%d] Successful chroot('%s') and chdir('/'), responding OK.\n", r->fd, r->data[0][1]);
	initd_response_request_ok(r);
}

#endif
