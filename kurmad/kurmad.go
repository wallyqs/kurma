// Copyright 2016 Apcera Inc. All rights reserved.

package kurmad

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apcera/kurma/pkg/aciremote"
	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/networkmanager/types"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"

	atypes "github.com/appc/spec/schema/types"
)

// Config is the configuration structure of kurmad.
type Config struct {
	// Debug is a flag which activates debug tracing
	// done by the daemon.
	Debug bool `json:"debug,omitempty"`

	// SocketPath is the location of the socket to which
	// we can connect to make requests to the Kurma API.
	SocketPath string `json:"socketPath,omitempty"`

	// SocketPermissions are the file permissions to be set
	// on the Unix socket created by the daemon.
	SocketPermissions *int `json:"socketPermissions,omitempty"`

	// ParentCgroupName is the name of the cgroup to be used
	// by the pod manager when generating the config from a container.
	ParentCgroupName string `json:"parentCgroupName,omitempty"`

	// PodsDirectory is the directory which the pod manager
	// will use for the pods.
	PodsDirectory string `json:"podsDirectory,omitempty"`

	// ImagesDirectory is the directory which will be used
	// by the image manager.
	ImagesDirectory string `json:"imagesDirectory,omitempty"`

	// VolumesDirectory is the directory which the pod manager
	// will use for the volume mounts.
	VolumesDirectory string `json:"volumesDirectory,omitempty"`

	// DefaultStagerImage is the uri of the default ACI image
	// to be used for stager run when launching pods.
	DefaultStagerImage string `json:"defaultStagerImage,omitempty"`

	// PrefetchImages is the collection of image uris which
	// will be preloaded onto the image manager when kurmad boots.
	PrefetchImages []string `json:"prefetchImages,omitempty"`

	// InitialPods is the collection of manifests from pods which will
	// be launched when kurmad boots.
	InitialPods []*InitialPodManifest `json:"initialPods,omitempty"`

	// PodNetworks is the collection of networks which will be provisioned
	// by the network manager for pods.
	PodNetworks []*types.NetConf `json:"podNetworks"`
}

// InitialPodManifest is used to handle the initial pod configuration section,
// where either an image specification string can be given, or a partial pod
// manifest.
type InitialPodManifest struct {
	name  string
	image string
	pod   *schema.PodManifest
}

// process handles processing the initial configuration input and turning it
// into a ready to run pod manifest.
func (ip *InitialPodManifest) Process(imageManager backend.ImageManager) (string, *schema.PodManifest, error) {
	if ip.pod != nil {
		for i, app := range ip.pod.Apps {
			if app.Image.ID.Val != "" {
				continue
			}

			version, _ := app.Image.Labels.Get("version")
			hash, image := imageManager.FindImage(app.Image.Name.String(), version)
			if hash == "" {
				return "", nil, fmt.Errorf("unable to locate image for %s", app.Image.Name)
			}
			h, err := atypes.NewHash(hash)
			if err != nil {
				return "", nil, fmt.Errorf("failed to create hash for %s: %v", app.Image.Name, err)
			}
			app.Image.ID = *h
			app.Image.Name = &image.Name
			app.Image.Labels = image.Labels
			ip.pod.Apps[i] = app
		}

		return ip.name, ip.pod, nil
	}

	if ip.image == "" {
		return ip.name, nil, fmt.Errorf("failed to get a valid pod manifest")
	}

	hash, imageManifest := imageManager.FindImage(ip.image, "")
	if imageManifest == nil {
		var err error
		hash, imageManifest, err = aciremote.LoadImage(ip.image, true, imageManager)
		if err != nil {
			return ip.name, nil, fmt.Errorf("failed to get a retrieve image %q: %v", ip.image, err)
		}
	}

	appname, err := convertACIdentifierToACName(imageManifest.Name)
	if err != nil {
		return ip.name, nil, fmt.Errorf("failed to generate app name for %q: %v", ip.image, err)
	}

	imageID, err := atypes.NewHash(hash)
	if err != nil {
		return ip.name, nil, fmt.Errorf("failed to process image hash for %q: %v", ip.image, err)
	}

	ip.pod = schema.BlankPodManifest()
	ip.pod.Apps = []schema.RuntimeApp{
		schema.RuntimeApp{
			Name: *appname,
			Image: schema.RuntimeImage{
				ID: *imageID,
			},
		},
	}

	return ip.name, ip.pod, nil
}

// UnmarshalJSON handles deciding which avenue to parse the provided JSON input.
func (ip *InitialPodManifest) UnmarshalJSON(b []byte) error {
	switch b[0] {
	case '"':
		return ip.unmarshalImageUrl(b)
	case '{':
		return ip.unmarshalPodManifest(b)
	case 'n':
		return nil
	}
	return fmt.Errorf("failed to unmarshal initial pod manifest")
}

// unmarshalImageUrl handles unmarshaling just an image specification into a
// string that will be used later in process.
func (ip *InitialPodManifest) unmarshalImageUrl(b []byte) error {
	return json.Unmarshal(b, &ip.image)
}

// unmarshalPodManifest handles unmarshaling the input pod manifest.
func (ip *InitialPodManifest) unmarshalPodManifest(b []byte) error {
	// first unmarshal some extra fields
	extra := struct {
		Name string `json:"name"`
	}{}
	if err := json.Unmarshal(b, &extra); err != nil {
		return err
	}
	ip.name = extra.Name

	// then unmarshal to the pod object
	ip.pod = schema.BlankPodManifest()
	return json.Unmarshal(b, &ip.pod)
}

// Run takes over the process and launches kurmad. It will return an error if any
// part of the setup fails.
func Run(configFile string) error {
	r := &runner{
		configFile: configFile,
		log:        logray.New(),
	}
	if err := bootstrap(r); err != nil {
		r.log.Errorf("ERROR: %v", err)
		return err
	}

	return nil
}

// bootstrap handles executing the bootstrap setup for kurmad.
func bootstrap(r setupRunner) error {
	r.setupSignalHandling()

	err := r.loadConfigurationFile()
	if err != nil {
		return err
	}

	r.configureLogging()

	err = r.createDirectories()
	if err != nil {
		return err
	}

	err = r.createImageManager()
	if err != nil {
		return err
	}

	r.prefetchImages()

	err = r.createPodManager()
	if err != nil {
		return err
	}

	r.createNetworkManager()

	err = r.startDaemon()
	if err != nil {
		return err
	}

	r.startInitialPods()

	return nil
}

func convertACIdentifierToACName(name atypes.ACIdentifier) (*atypes.ACName, error) {
	parts := strings.Split(name.String(), "/")
	n, err := atypes.SanitizeACName(parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	return atypes.NewACName(n)
}
