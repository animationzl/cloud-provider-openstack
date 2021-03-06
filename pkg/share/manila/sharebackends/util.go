/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sharebackends

import (
	"fmt"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/shares"
	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func getSecretName(shareID string) string {
	return "manila-" + shareID
}

// Splits ExportLocation path "addr1:port,addr2:port,...:/location" into its address
// and location parts. The last occurrence of ':' is considered as the delimiter
// between those two parts.
func splitExportLocation(loc *shares.ExportLocation) (address, location string, err error) {
	delimPos := strings.LastIndexByte(loc.Path, ':')
	if delimPos <= 0 {
		err = fmt.Errorf("failed to parse address and location from export location '%s'", loc.Path)
		return
	}

	address = loc.Path[:delimPos]
	location = loc.Path[delimPos+1:]

	return
}

func createSecret(name, namespace string, cs clientset.Interface, data map[string][]byte) error {
	sec := v1.Secret{Data: data}
	sec.Name = name

	if _, err := cs.CoreV1().Secrets(namespace).Create(&sec); err != nil {
		return err
	}

	return nil
}

func deleteSecret(name, namespace string, cs clientset.Interface) error {
	return cs.CoreV1().Secrets(namespace).Delete(name, nil)
}

// Grants access to Ceph share. Since Ceph share keys are generated by Ceph backend,
// they're not contained in the response from shares.GrantAccess(), but have to be
// queried for separately by subsequent ListAccessRights call(s)
func grantAccessCephx(args *GrantAccessArgs) (*shares.AccessRight, error) {
	accessOpts := shares.GrantAccessOpts{
		AccessType:  "cephx",
		AccessTo:    args.Share.Name,
		AccessLevel: "rw",
	}

	if _, err := shares.GrantAccess(args.Client, args.Share.ID, accessOpts).Extract(); err != nil {
		return nil, err
	}

	var accessRight shares.AccessRight

	err := gophercloud.WaitFor(120, func() (bool, error) {
		accessRights, err := shares.ListAccessRights(args.Client, args.Share.ID).Extract()
		if err != nil {
			return false, err
		}

		if len(accessRights) > 1 {
			return false, fmt.Errorf("unexpected number of access rules: got %d, expected 1", len(accessRights))
		} else if len(accessRights) == 0 {
			return false, nil
		}

		if accessRights[0].AccessKey != "" {
			accessRight = accessRights[0]
			return true, nil
		}

		return false, nil
	})

	return &accessRight, err
}
