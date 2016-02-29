// Copyright 2015-2016 Apcera Inc. All rights reserved.

package image

import (
	"testing"

	"github.com/apcera/logray"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	tt "github.com/apcera/util/testtool"
)

var (
	hash1  = types.NewHashSHA512([]byte(`1`))
	hash2  = types.NewHashSHA512([]byte(`2`))
	hash3  = types.NewHashSHA512([]byte(`3`))
	hash4  = types.NewHashSHA512([]byte(`4`))
	hash5  = types.NewHashSHA512([]byte(`5`))
	hash6  = types.NewHashSHA512([]byte(`6`))
	hash7  = types.NewHashSHA512([]byte(`7`))
	hash8  = types.NewHashSHA512([]byte(`8`))
	hash9  = types.NewHashSHA512([]byte(`9`))
	hash10 = types.NewHashSHA512([]byte(`10`))
)

func TestProcessLayers(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	manager := &Manager{
		Log: logray.New(),
		Options: &Options{
			Directory: "foo",
		},
		images: map[string]*schema.ImageManifest{
			hash1.String(): &schema.ImageManifest{
				Dependencies: []types.Dependency{
					{ImageID: hash2},
				},
			},
			hash2.String(): &schema.ImageManifest{
				Dependencies: []types.Dependency{
					{ImageID: hash3},
					{ImageID: hash4},
				},
			},
			hash3.String(): &schema.ImageManifest{
				Dependencies: []types.Dependency{
					{ImageID: hash5},
				},
			},
			hash4.String(): &schema.ImageManifest{
				Dependencies: []types.Dependency{
					{ImageID: hash5},
				},
			},
			hash5.String(): &schema.ImageManifest{},
		},
	}

	layers, err := manager.processLayers(hash1.String())
	tt.TestExpectSuccess(t, err)

	// order should be: 1, 2, 3, 5, 4
	expectedOrder := []string{hash1.String(), hash2.String(), hash3.String(), hash5.String(), hash4.String()}
	tt.TestEqual(t, layers, expectedOrder)
}
