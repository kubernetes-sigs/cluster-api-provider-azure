/*
Copyright 2023 The Kubernetes Authors.

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

package filewatcher

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

var watchCertificateFileOnce sync.Once

// WatchFileForChanges watches the file, fileToWatch, for changes. If the file contents have changed, the pod this
// function is running on will be restarted.
func WatchFileForChanges(fileToWatch string) error {
	var err error

	// This starts only one occurrence of the file watcher, which watches the file, fileToWatch.
	watchCertificateFileOnce.Do(func() {
		klog.V(2).Infof("Starting the file change watcher on file, %s", fileToWatch)

		// Update the file path to watch in case this is a symlink
		fileToWatch, err = filepath.EvalSymlinks(fileToWatch)
		if err != nil {
			return
		}
		klog.V(2).Infof("Watching file, %s", fileToWatch)

		// Start the file watcher to monitor file changes
		go func() {
			err := checkForFileChanges(fileToWatch)
			klog.Errorf("Error checking for file changes: %v", err)
		}()
	})
	return err
}

// checkForFileChanges starts a new file watcher. If the file is changed, the pod running this function will exit.
func checkForFileChanges(path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if ok && (event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod) || event.Has(fsnotify.Remove)) {
					klog.Infof("file, %s, was modified, exiting...", event.Name)
					os.Exit(0)
				}
			case err, ok := <-watcher.Errors:
				if ok {
					klog.Errorf("file watcher error: %v", err)
				}
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		return err
	}

	return nil
}
