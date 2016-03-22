// Copyright 2015-2016 Apcera Inc. All rights reserved.

package apiproxy

import (
	"fmt"

	kschema "github.com/apcera/kurma/schema"
	"github.com/appc/spec/schema"
)

func validatePodManifest(manifest *schema.PodManifest) error {
	if len(manifest.Apps) == 0 {
		return fmt.Errorf("the imageManifest must specify an App")
	}

	for _, iso := range manifest.Isolators {
		// Reject any containers that request host privilege. This can only be started
		// with the local API, not remote API.
		if iso.Name.String() == kschema.HostPrivilegedName {
			return fmt.Errorf("host privileged containers cannot be launched remotely")
		}

		// Reject any containers that request host privilege. This can only be started
		// with the local API, not remote API.
		if iso.Name.String() == kschema.HostApiAccessName {
			return fmt.Errorf("host API access containers cannot be launched remotely")
		}
	}

	// FIXME once network isolation is in, this should force adding the container
	// namespaces isolator to ensure any remotely sourced images are network
	// namespaced.

	return nil
}
