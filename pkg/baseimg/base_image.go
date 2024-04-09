// Copyright 2022, 2023 Chainguard, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package baseimg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"

	"github.com/chainguard-dev/go-apk/pkg/apk"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"

	"chainguard.dev/apko/pkg/build/types"
)

type BaseImage struct {
	img      v1.Image
	apkIndex []byte
	tmpDir   string
	arch     types.Architecture
}

func getImageForArch(imgPath string, arch types.Architecture) (v1.Image, error) {
	index, err := layout.ImageIndexFromPath(imgPath)
	if err != nil {
		return nil, err
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}

	for _, m := range indexManifest.Manifests {
		if m.Platform.Architecture == arch.ToOCIPlatform().Architecture {
			img, err := index.Image(m.Digest)
			if err != nil {
				return nil, err
			}
			return img, nil
		}
	}
	return nil, fmt.Errorf("image for arch not found")
}

func New(imgPath string, apkIndexPath string, arch types.Architecture, tmpDir string) (*BaseImage, error) {
	img, err := getImageForArch(imgPath, arch)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(apkIndexPath)
	if err != nil {
		return nil, err
	}
	baseImg := BaseImage{
		img:      img,
		apkIndex: contents,
		tmpDir:   tmpDir,
		arch:     arch,
	}
	err = baseImg.createAPKIndexArchive()
	if err != nil {
		return nil, err
	}
	return &baseImg, nil
}

func (baseImg *BaseImage) Image() v1.Image {
	return baseImg.img
}

func (baseImg *BaseImage) InstalledPackages() ([]*apk.InstalledPackage, error) {
	reader := bytes.NewReader(baseImg.apkIndex)
	return apk.ParseInstalled(reader)
}

func (baseImg *BaseImage) APKIndexPath() string {
	return baseImg.tmpDir + "/base_image_apkindex"
}

func (baseImg *BaseImage) createAPKIndexArchive() error {
	archDir := baseImg.APKIndexPath() + "/" + baseImg.arch.ToAPK()
	if err := os.MkdirAll(archDir, 0777); err != nil {
		return err
	}
	tarFile, err := os.OpenFile(archDir+"/APKINDEX.tar.gz", os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer tarFile.Close()
	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
	header := tar.Header{Name: "APKINDEX", Size: int64(len(baseImg.apkIndex)), Mode: 0777}
	if err := tarWriter.WriteHeader(&header); err != nil {
		return err
	}
	if _, err := tarWriter.Write(baseImg.apkIndex); err != nil {
		return err
	}
	return nil
}
