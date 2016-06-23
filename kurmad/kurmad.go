// Copyright 2016 Apcera Inc. All rights reserved.

package kurmad

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apcera/kurma/pkg/backend"
	"github.com/apcera/kurma/pkg/image"
	"github.com/apcera/kurma/pkg/networkmanager/types"
	"github.com/apcera/logray"
	"github.com/appc/spec/schema"

	atypes "github.com/appc/spec/schema/types"
)

var (
	// The setup functions that should be run in order to handle setting up
	// kurmad.
	setupFunctions = []func(*runner) error{
		(*runner).setupSignalHandling,
		(*runner).loadConfigurationFile,
		(*runner).configureLogging,
		(*runner).createDirectories,
		(*runner).createImageManager,
		(*runner).prefetchImages,
		(*runner).createPodManager,
		(*runner).createNetworkManager,
		(*runner).startDaemon,
		(*runner).startInitialPods,
	}
)

// Config is the configuration structure of kurmad.
type Config struct {
	Debug              bool                  `json:"debug,omitempty"`
	SocketPath         string                `json:"socketPath,omitempty"`
	SocketPermissions  *int                  `json:"socketPermissions,omitempty"`
	ParentCgroupName   string                `json:"parentCgroupName,omitempty"`
	PodsDirectory      string                `json:"podsDirectory,omitempty"`
	ImagesDirectory    string                `json:"imagesDirectory,omitempty"`
	VolumesDirectory   string                `json:"volumesDirectory,omitempty"`
	DefaultStagerImage string                `json:"defaultStagerImage,omitempty"`
	PrefetchImages     []string              `json:"prefetchImages,omitempty"`
	InitialPods        []*InitialPodManifest `json:"initialPods,omitempty"`
	PodNetworks        []*types.NetConf      `json:"podNetworks"`
}

// InitialPodManifest is used to handle the inital pod configuration section,
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
		hash, imageManifest, err = image.FetchAndLoad(ip.image, nil, true, imageManager)
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

// runner is an object that is used to handle the startup of the full KurmaOS
// system. It will take of the running of the process once init.Run() is
// invoked.
type runner struct {
	config         *Config
	configFile     string
	log            *logray.Logger
	podManager     backend.PodManager
	imageManager   backend.ImageManager
	networkManager backend.NetworkManager
}

// Run takes over the process and launches kurmad.
func Run(configFile string) error {
	r := &runner{
		configFile: configFile,
		log:        logray.New(),
	}
	return r.Run()
}

// Run handles executing the bootstrap setup. It will return an error if any
// part of the setup fails.
func (r *runner) Run() error {
	for _, f := range setupFunctions {
		if err := f(r); err != nil {
			r.log.Errorf("ERROR: %v", err)
			return fmt.Errorf("failed in startup function: %v", err)
		}
	}
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
