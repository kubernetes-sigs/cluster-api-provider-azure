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

package wait

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/klog"
)

const (
	gceTimeout   = time.Minute * 10
	gceWaitSleep = time.Second * 5
)

func ForComputeOperation(client *compute.Service, project string, op *compute.Operation) error {
	start := time.Now()
	ctx, cf := context.WithTimeout(context.Background(), gceTimeout)
	defer cf()

	var err error
	for {
		if err = checkComputeOperation(op, err); err != nil || op.Status == "DONE" {
			return err
		}
		klog.V(1).Infof("Wait for %v %q: %v (%d%%): %v", op.OperationType, op.Name, op.Status, op.Progress, op.StatusMessage)
		select {
		case <-ctx.Done():
			return fmt.Errorf("gce operation %v %q timed out after %v", op.OperationType, op.Name, time.Since(start))
		case <-time.After(gceWaitSleep):
		}
		op, err = getComputeOperation(client, project, op)
	}
}

// getComputeOperation returns an updated operation.
func getComputeOperation(client *compute.Service, project string, op *compute.Operation) (*compute.Operation, error) {
	switch {
	case op.Zone != "":
		return client.ZoneOperations.Get(project, path.Base(op.Zone), op.Name).Do()
	case op.Region != "":
		return client.RegionOperations.Get(project, path.Base(op.Region), op.Name).Do()
	default:
		return client.GlobalOperations.Get(project, op.Name).Do()
	}
}

func checkComputeOperation(op *compute.Operation, err error) error {
	if err != nil || op.Error == nil || len(op.Error.Errors) == 0 {
		return err
	}
	var errs bytes.Buffer
	for _, v := range op.Error.Errors {
		errs.WriteString(v.Message)
		errs.WriteByte('\n')
	}
	return errors.New(errs.String())
}
