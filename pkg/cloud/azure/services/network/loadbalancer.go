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

package network

/*
import (
	"fmt"
	"time"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/azure/services/azureerrors"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/azure/services/wait"

	"github.com/azure/azure-sdk-go/azure"
	"github.com/azure/azure-sdk-go/azure/azureerr"
	"github.com/azure/azure-sdk-go/service/elb"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"k8s.io/klog"
)

// TODO
// ReconcileLoadBalancers reconciles the load balancers for the given cluster.
func (s *Service) ReconcileLoadBalancers() error {
	klog.V(2).Info("Reconciling load balancers")

	// Get default api server spec.
	spec := s.getAPIServerLoadBalancerSpec()

	// Describe or create.
	apiLB, err := s.describeLoadBalancer(spec.Name)
	if IsNotFound(err) {
		apiLB, err = s.createLoadBalancer(spec)
		if err != nil {
			return err
		}

		klog.V(2).Infof("Created new classic load balancer for apiserver: %v", apiLB)
	} else if err != nil {
		return err
	}

	// TODO(vincepri): check if anything has changed and reconcile as necessary.
	apiLB.DeepCopyInto(&s.scope.Network().APIServerLB)
	klog.V(2).Info("Reconcile load balancers completed successfully")
	return nil
}

// TODO
// GetAPIServerDNSName returns the DNS name endpoint for the API server
func (s *Service) GetAPIServerDNSName() (string, error) {
	apiLB, err := s.describeLoadBalancer(GenerateLBName(s.scope.Name(), TagValueAPIServerRole))

	if err != nil {
		return "", err
	}

	return apiLB.DNSName, nil
}

// TODO
// DeleteLoadBalancers deletes the load balancers for the given cluster.
func (s *Service) DeleteLoadBalancers() error {
	klog.V(2).Info("Deleting load balancers")

	// Get default api server spec.
	spec := s.getAPIServerLoadBalancerSpec()

	// Describe or create.
	apiLB, err := s.describeLoadBalancer(spec.Name)
	if IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := s.deleteLoadBalancerAndWait(apiLB.Name); err != nil {
		return err
	}

	klog.V(2).Info("Deleting load balancers completed successfully")
	return nil
}

// TODO
// RegisterInstanceWithLoadBalancer registers an instance with a classic LB
func (s *Service) RegisterInstanceWithLoadBalancer(vmId string, loadBalancer string) error {
	input := &elb.RegisterInstancesWithLoadBalancerInput{
		Instances:        []*elb.Instance{{InstanceId: azure.String(vmId)}},
		LoadBalancerName: azure.String(loadBalancer),
	}

	_, err := s.scope.LB.RegisterInstancesWithLoadBalancer(input)
	if err != nil {
		return err
	}

	return nil
}

// TODO
// RegisterInstanceWithAPIServerLB registers an instance with a classic LB
func (s *Service) RegisterInstanceWithAPIServerLB(vmId string) error {
	input := &elb.RegisterInstancesWithLoadBalancerInput{
		Instances:        []*elb.Instance{{InstanceId: azure.String(vmId)}},
		LoadBalancerName: azure.String(GenerateLBName(s.scope.Name(), TagValueAPIServerRole)),
	}

	_, err := s.scope.LB.RegisterInstancesWithLoadBalancer(input)
	if err != nil {
		return err
	}

	return nil
}

// GenerateLBName generates a formatted LB name
func GenerateLBName(clusterName string, lbName string) string {
	return fmt.Sprintf("%s-%s", clusterName, lbName)
}

// TODO
func (s *Service) getAPIServerLoadBalancerSpec() *v1alpha1.LoadBalancer {
	res := &v1alpha1.LoadBalancer{
		Name:   GenerateLBName(s.scope.Name(), TagValueAPIServerRole),
		Scheme: v1alpha1.LoadBalancerSchemeInternetFacing,
		Listeners: []*v1alpha1.LoadBalancerListener{
			{
				Protocol:         v1alpha1.LoadBalancerProtocolTCP,
				Port:             6443,
				InstanceProtocol: v1alpha1.LoadBalancerProtocolTCP,
				InstancePort:     6443,
			},
		},
		HealthCheck: &v1alpha1.LoadBalancerHealthCheck{
			Target:             fmt.Sprintf("%v:%d", v1alpha1.LoadBalancerProtocolTCP, 6443),
			Interval:           10 * time.Second,
			Timeout:            5 * time.Second,
			HealthyThreshold:   5,
			UnhealthyThreshold: 3,
		},
		SecurityGroupIDs: []string{s.scope.SecurityGroups()[v1alpha1.SecurityGroupControlPlane].ID},
		Tags:             s.buildTags(s.scope.Name(), ResourceLifecycleOwned, "", TagValueAPIServerRole, nil),
	}

	for _, sn := range s.scope.Subnets().FilterPublic() {
		res.SubnetIDs = append(res.SubnetIDs, sn.ID)
	}

	return res
}

// TODO
func (s *Service) createLoadBalancer(spec *v1alpha1.LoadBalancer) (*v1alpha1.LoadBalancer, error) {
	input := &elb.CreateLoadBalancerInput{
		LoadBalancerName: azure.String(spec.Name),
		Subnets:          azure.StringSlice(spec.SubnetIDs),
		SecurityGroups:   azure.StringSlice(spec.SecurityGroupIDs),
		Scheme:           azure.String(string(spec.Scheme)),
		Tags:             mapToTags(spec.Tags),
	}

	for _, ln := range spec.Listeners {
		input.Listeners = append(input.Listeners, &elb.Listener{
			Protocol:         azure.String(string(ln.Protocol)),
			LoadBalancerPort: azure.Int64(ln.Port),
			InstanceProtocol: azure.String(string(ln.InstanceProtocol)),
			InstancePort:     azure.Int64(ln.InstancePort),
		})
	}

	out, err := s.scope.LB.CreateLoadBalancer(input)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create classic load balancer: %v", spec)
	}

	if spec.HealthCheck != nil {
		hc := &elb.ConfigureHealthCheckInput{
			LoadBalancerName: azure.String(spec.Name),
			HealthCheck: &elb.HealthCheck{
				Target:             azure.String(spec.HealthCheck.Target),
				Interval:           azure.Int64(int64(spec.HealthCheck.Interval.Seconds())),
				Timeout:            azure.Int64(int64(spec.HealthCheck.Timeout.Seconds())),
				HealthyThreshold:   azure.Int64(spec.HealthCheck.HealthyThreshold),
				UnhealthyThreshold: azure.Int64(spec.HealthCheck.UnhealthyThreshold),
			},
		}

		if _, err := s.scope.LB.ConfigureHealthCheck(hc); err != nil {
			return nil, errors.Wrapf(err, "failed to configure health check for classic load balancer: %v", spec)
		}
	}

	klog.V(2).Infof("Created load balancer with dns name: %q", *out.DNSName)

	res := spec.DeepCopy()
	res.DNSName = *out.DNSName
	return res, nil
}

// TODO
func (s *Service) deleteLoadBalancer(name string) error {
	input := &elb.DeleteLoadBalancerInput{
		LoadBalancerName: azure.String(name),
	}

	if _, err := s.scope.LB.DeleteLoadBalancer(input); err != nil {
		return err
	}
	return nil
}

// TODO
func (s *Service) deleteLoadBalancerAndWait(name string) error {
	if err := s.deleteLoadBalancer(name); err != nil {
		return err
	}

	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: azure.StringSlice([]string{name}),
	}

	checkForLBDeletion := func() (done bool, err error) {
		out, err := s.scope.LB.DescribeLoadBalancers(input)

		// LB already deleted.
		if len(out.LoadBalancerDescriptions) == 0 {
			return true, nil
		}

		if code, _ := azureerrors.Code(err); code == "LoadBalancerNotFound" {
			return true, nil
		}

		if err != nil {
			return false, err
		}

		return false, nil

	}

	if err := wait.WaitForWithRetryable(wait.NewBackoff(), checkForLBDeletion, []string{}); err != nil {
		return errors.Wrapf(err, "failed to wait for LB deletion %q", name)
	}

	return nil
}

// TODO
func (s *Service) describeLoadBalancer(name string) (*v1alpha1.LoadBalancer, error) {
	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: azure.StringSlice([]string{name}),
	}

	out, err := s.scope.LB.DescribeLoadBalancers(input)
	if err != nil {
		if aerr, ok := err.(azureerr.Error); ok {
			switch aerr.Code() {
			case elb.ErrCodeAccessPointNotFoundException:
				return nil, errors.Wrapf(err, "no classic load balancer found with name: %q", name)
			case elb.ErrCodeDependencyThrottleException:
				return nil, errors.Wrap(err, "too many requests made to the LB service")
			default:
				return nil, errors.Wrap(err, "unexpected azure error")
			}
		} else {
			return nil, errors.Wrapf(err, "failed to describe classic load balancer: %s", name)
		}
	}

	if out == nil && len(out.LoadBalancerDescriptions) == 0 {
		return nil, NewNotFound(fmt.Errorf("no classic load balancer found with name %q", name))
	}

	return fromSDKTypeToLoadBalancer(out.LoadBalancerDescriptions[0]), nil
}

// TODO
func fromSDKTypeToLoadBalancer(v *elb.LoadBalancerDescription) *v1alpha1.LoadBalancer {
	return &v1alpha1.LoadBalancer{
		Name:             azure.StringValue(v.LoadBalancerName),
		Scheme:           v1alpha1.LoadBalancerScheme(*v.Scheme),
		SubnetIDs:        azure.StringValueSlice(v.Subnets),
		SecurityGroupIDs: azure.StringValueSlice(v.SecurityGroups),
		DNSName:          azure.StringValue(v.DNSName),
	}
}

// TODO
func fromSDKTypeToClassicListener(v *elb.Listener) *v1alpha1.LoadBalancerListener {
	return &v1alpha1.LoadBalancerListener{
		Protocol:         v1alpha1.LoadBalancerProtocol(*v.Protocol),
		Port:             *v.LoadBalancerPort,
		InstanceProtocol: v1alpha1.LoadBalancerProtocol(*v.InstanceProtocol),
		InstancePort:     *v.InstancePort,
	}
}

// TODO
func fromSDKTypeToClassicHealthCheck(v *elb.HealthCheck) *v1alpha1.LoadBalancerHealthCheck {
	return &v1alpha1.LoadBalancerHealthCheck{
		Target:             *v.Target,
		Interval:           time.Duration(*v.Interval) * time.Second,
		Timeout:            time.Duration(*v.Timeout) * time.Second,
		HealthyThreshold:   *v.HealthyThreshold,
		UnhealthyThreshold: *v.UnhealthyThreshold,
	}
}
*/
