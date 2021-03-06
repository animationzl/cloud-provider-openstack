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

package keystone

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/golang/glog"
)

// By now only project syncing is supported
// TODO(mfedosin): Implement syncing of role assignments, system role assignments, and user groups
var allowedDataTypesToSync = []string{"projects"}

// syncConfig contains configuration data for synchronization between Keystone and Kubernetes
type syncConfig struct {
	// List containing possible data types to sync. Now only "projects" are supported.
	DataTypesToSync []string `yaml:"data_types_to_sync"`

	// Format of automatically created namespace name. Can contain wildcards %i and %n,
	// corresponding to project id and project name respectively.
	NamespaceFormat string `yaml:"namespace_format"`

	// List of project ids to exclude from syncing.
	ProjectBlackList []string `yaml:"projects_black_list"`
}

func (sc *syncConfig) validate() error {
	// Namespace name must contain keystone project id
	if !strings.Contains(sc.NamespaceFormat, "%i") {
		return fmt.Errorf("format string should comprise a %%i substring (keystone project id)")
	}

	// By convention, the names should be up to maximum length of 63 characters and consist of
	// lower and upper case alphanumeric characters, -, _ and .
	ts := strings.Replace(sc.NamespaceFormat, "%i", "aa", -1)
	ts = strings.Replace(ts, "%n", "aa", -1)
	ts = strings.Replace(ts, "%d", "aa", -1)

	re := regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_.-]*[a-zA-Z0-9]$")
	if !re.MatchString(ts) {
		return fmt.Errorf("namespace name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character")
	}

	// Check that only allowed data types are enabled for synchronization
	for _, dt := range sc.DataTypesToSync {
		var flag bool
		for _, a := range allowedDataTypesToSync {
			if a == dt {
				flag = true
				break
			}
		}
		if !flag {
			return fmt.Errorf(
				"Unsupported data type to sync: %v. Available values: %v",
				dt,
				strings.Join(allowedDataTypesToSync, ","),
			)
		}
	}

	return nil
}

// formatNamespaceName generates a namespace name, based on format string
func (sc *syncConfig) formatNamespaceName(id string, name string, domain string) string {
	res := strings.Replace(sc.NamespaceFormat, "%i", id, -1)
	res = strings.Replace(res, "%n", name, -1)
	res = strings.Replace(res, "%d", domain, -1)

	if len(res) > 63 {
		glog.Warningf("Generated namespace name '%v' exceeds the maximum possible length of 63 characters. Just Keystone project id '%v' will be used as the namespace name.", res, id)
		return id
	}

	return res
}

// newSyncConfig defines the default values for syncConfig
func newSyncConfig() syncConfig {
	return syncConfig{
		// by default namespace name is a string containing just keystone project id
		NamespaceFormat: "%i",
		// by default all possible data types are enabled
		DataTypesToSync: allowedDataTypesToSync,
	}
}

// newSyncConfigFromFile loads a sync config from a file
func newSyncConfigFromFile(path string) (*syncConfig, error) {
	sc := newSyncConfig()

	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Errorf("yamlFile get err   #%v ", err)
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &sc)
	if err != nil {
		glog.Errorf("Unmarshal: %v", err)
		return nil, err
	}

	return &sc, nil
}
