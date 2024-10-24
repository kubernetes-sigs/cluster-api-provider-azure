/*
Copyright 2024 The Kubernetes Authors.

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

package azure

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

type fakeTokenCredential struct {
	tenantID string
}

func (t fakeTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, nil
}

func TestGetOrStore(t *testing.T) {
	g := NewGomegaWithT(t)

	credCache := &credentialCache{
		mut:   new(sync.Mutex),
		cache: make(map[credentialCacheKey]azcore.TokenCredential),
	}

	newCredCount := 0
	newCredFunc := func(cred fakeTokenCredential, err error) func() (azcore.TokenCredential, error) {
		return func() (azcore.TokenCredential, error) {
			newCredCount++
			return cred, err
		}
	}

	// the first call for a new key should invoke newCredFunc
	cred, err := credCache.getOrStore(credentialCacheKey{tenantID: "1"}, newCredFunc(fakeTokenCredential{tenantID: "1"}, nil))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cred).To(Equal(fakeTokenCredential{tenantID: "1"}))
	g.Expect(newCredCount).To(Equal(1))

	// subsequent calls for the same key should not create a new credential
	cred, err = credCache.getOrStore(credentialCacheKey{tenantID: "1"}, newCredFunc(fakeTokenCredential{tenantID: "1"}, nil))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cred).To(Equal(fakeTokenCredential{tenantID: "1"}))
	g.Expect(newCredCount).To(Equal(1))
	cred, err = credCache.getOrStore(credentialCacheKey{tenantID: "1"}, newCredFunc(fakeTokenCredential{tenantID: "1"}, nil))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cred).To(Equal(fakeTokenCredential{tenantID: "1"}))
	g.Expect(newCredCount).To(Equal(1))

	expectedErr := errors.New("an error")
	cred, err = credCache.getOrStore(credentialCacheKey{tenantID: "2"}, newCredFunc(fakeTokenCredential{tenantID: "2"}, expectedErr))
	g.Expect(err).To(MatchError(expectedErr))
	g.Expect(cred).To(BeNil())
	g.Expect(newCredCount).To(Equal(2))
}

func TestGetOrStoreRace(t *testing.T) {
	// This test makes no assertions, it only fails when the race detector finds race conditions.

	credCache := &credentialCache{
		mut:   new(sync.Mutex),
		cache: make(map[credentialCacheKey]azcore.TokenCredential),
	}
	newCredFunc := func(cred fakeTokenCredential, err error) func() (azcore.TokenCredential, error) {
		return func() (azcore.TokenCredential, error) {
			return cred, err
		}
	}

	wg := new(sync.WaitGroup)
	n := 1000
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = credCache.getOrStore(credentialCacheKey{tenantID: strconv.Itoa(i % 100)}, newCredFunc(fakeTokenCredential{}, nil))
		}()
	}
	wg.Wait()
}
