// Copyright 2015 Apcera Inc. All rights reserved.

#ifndef STORAGE_OVERLAY_RUNNER_C
#define STORAGE_OVERLAY_RUNNER_C

#include <errno.h>
#include <error.h>
#include <getopt.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/types.h>

char* join_strings(char* strings[], char* seperator, int count) {
    char* str = NULL;             /* Pointer to the joined strings  */
    size_t total_length = 0;      /* Total length of joined strings */
    int i = 0;                    /* Loop counter                   */

    /* Find total length of joined strings */
    for (i = 0; i < count; i++) total_length += strlen(strings[i]);
    total_length++;     /* For joined string terminator */
    total_length += strlen(seperator) * (count - 1); // for seperators

    str = (char*) malloc(total_length);  /* Allocate memory for joined strings */
    str[0] = '\0';                      /* Empty string we can append to      */

    /* Append all the strings */
    for (i = 0; i < count; i++) {
        strcat(str, strings[i]);
        if (i < (count - 1)) strcat(str, seperator);
    }

    return str;
}

// With this constructor we can ensure that we run our overlay_runner() function prior
// to golang startup for cgroups_initd.
void overlay_runner(int argc, char **argv) __attribute__ ((constructor));

// This function is executed prior to golang's startup logic.
void overlay_runner(int argc, char **argv)
{
	char **lowerdir;
	char *upperdir;
	char *workdir;
	char *destdir;
	char *finishdir;
	char *options;
	int c;

	// Ensure we're intended to run
	if (getenv("STORAGE_OVERLAY_INTERCEPT") == NULL) {
		return;
	}

	lowerdir = NULL;
	size_t dir_len = 0;

	// loop and process the arguments
	while(1) {
		static struct option long_options[] =
			{
				{"lowerdir", required_argument, 0, 'a'},
				{"upperdir",  required_argument, 0, 'b'},
				{"workdir", required_argument, 0, 'c'},
				{"destdir", required_argument, 0, 'd'},
				{"finishdir", required_argument, 0, 'e'},
				{0, 0, 0, 0}
			};
		/* getopt_long stores the option index here. */
		int option_index = 0;

		c = getopt_long(argc, argv, "abcde", long_options, &option_index);

		/* Detect the end of the options. */
		if (c == -1)
			break;

		switch (c) {
		case 0:
			/* If this option set a flag, do nothing else now. */
			if (long_options[option_index].flag != 0)
			break;
			printf ("option %s", long_options[option_index].name);
			if (optarg)
			printf (" with arg %s", optarg);
			printf ("\n");
			break;

			// lowerdir
		case 'a':
			lowerdir = realloc(lowerdir, sizeof(char*) * (dir_len+1));
			if (!lowerdir) { error(1, 0, "lowerdir was null"); }
			lowerdir[dir_len] = optarg;
			dir_len++;
			break;

			// others
		case 'b':
			upperdir = optarg;
			break;
		case 'c':
			workdir = optarg;
			break;
		case 'd':
			destdir = optarg;
			break;
		case 'e':
			finishdir = optarg;
			break;

		case '?':
			/* getopt_long already printed an error message. */
			break;

		default:
			abort();
		}
	}

	// ensure the final element in arrays is null
	lowerdir = realloc(lowerdir, sizeof(char*) * (dir_len+1));
	lowerdir[dir_len] = NULL;

	// Create the upperdir and workdir
	mkdir(upperdir, 0755);
	mkdir(workdir, 0755);
	mkdir(destdir, 0755);

	// Mount
	char *lower = join_strings(lowerdir, ":", dir_len);
	options = malloc(strlen(lower)+strlen(upperdir)+strlen(workdir)+28+1);
	sprintf(options, "lowerdir=%s,upperdir=%s,workdir=%s", lower, upperdir, workdir);
	if (mount("overlay", destdir, "overlay", 0, options) < 0)
		error(1, errno, "Failed to mount overlay filesystem");

	mkdir(finishdir, 0755);

	// Wait to be killed
	pause();

	// Ensure that we never ever fall back into the go world.
	exit(0);
}

#endif
