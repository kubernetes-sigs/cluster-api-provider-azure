/*
Copyright 2020 The Kubernetes Authors.

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

package ttllru

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	mockttllru "sigs.k8s.io/cluster-api-provider-azure/util/cache/ttllru/mocks"
)

const (
	defaultCacheDuration = 30 * time.Second
)

func TestNew(t *testing.T) {
	g := NewWithT(t)
	subject, err := New(128, defaultCacheDuration)
	g.Expect(err).Should(BeNil())
	g.Expect(subject).ShouldNot(BeNil())
}

func TestCache_Add(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	mockCache := mockttllru.NewMockCacher(mockCtrl)
	defer mockCtrl.Finish()
	subject, err := newCache(defaultCacheDuration, mockCache)
	g.Expect(err).Should(BeNil())

	key, value := "foo", "bar"
	mockCache.EXPECT().Add(gomock.Eq(key), gomockinternal.CustomMatcher(
		func(val interface{}, state map[string]interface{}) bool {
			ttl, ok := val.(*timeToLiveItem)
			if !ok {
				state["error"] = "value was not a time to live item"
				return false
			}

			if ttl.Value != value || time.Since(ttl.LastTouch) >= defaultCacheDuration {
				state["error"] = fmt.Sprintf("failed matching %+v", ttl)
				return false
			}

			return true
		},
		func(state map[string]interface{}) string {
			return state["error"].(string)
		},
	))
	subject.Add(key, value)
}

func TestCache_Get(t *testing.T) {
	const (
		key   = "key"
		value = "value"
	)

	cases := []struct {
		Name     string
		TestCase func(g *GomegaWithT, subject PeekingCacher, mock *mockttllru.MockCacher)
	}{
		{
			Name: "NoItemsInCache",
			TestCase: func(g *GomegaWithT, subject PeekingCacher, mock *mockttllru.MockCacher) {
				key := "not_there"
				mock.EXPECT().Get(key).Return(nil, false)
				val, ok := subject.Get(key)
				g.Expect(ok).To(BeFalse())
				g.Expect(val).To(BeNil())
			},
		},
		{
			Name: "ExistingItemNotExpired",
			TestCase: func(g *GomegaWithT, subject PeekingCacher, mock *mockttllru.MockCacher) {
				mock.EXPECT().Get(key).Return(&timeToLiveItem{
					LastTouch: time.Now(),
					Value:     value,
				}, true)
				val, ok := subject.Get(key)
				g.Expect(ok).To(BeTrue())
				g.Expect(val).To(Equal(value))
			},
		},
		{
			Name: "ExistingItemExpired",
			TestCase: func(g *GomegaWithT, subject PeekingCacher, mock *mockttllru.MockCacher) {
				mock.EXPECT().Get(key).Return(&timeToLiveItem{
					LastTouch: time.Now().Add(-(10*time.Second + defaultCacheDuration)),
					Value:     value,
				}, true)
				mock.EXPECT().Remove(key).Return(false)
				val, ok := subject.Get(key)
				g.Expect(ok).To(BeFalse())
				g.Expect(val).To(BeNil())
			},
		},
		{
			Name: "ExistingItemGetAdvancesLastTouch",
			TestCase: func(g *GomegaWithT, subject PeekingCacher, mock *mockttllru.MockCacher) {
				lastTouch := time.Now().Add(defaultCacheDuration - 10*time.Second)
				item := &timeToLiveItem{
					LastTouch: lastTouch,
					Value:     value,
				}

				mock.EXPECT().Get(key).Return(item, true)
				val, ok := subject.Get(key)
				g.Expect(ok).To(BeTrue())
				g.Expect(val).To(Equal(value))
				g.Expect(lastTouch.After(item.LastTouch)).To(BeTrue())
			},
		},
		{
			Name: "ExistingItemIsNotTTLItem",
			TestCase: func(g *GomegaWithT, subject PeekingCacher, mock *mockttllru.MockCacher) {
				item := &struct {
					Value string
				}{
					Value: value,
				}

				mock.EXPECT().Get(key).Return(item, true)
				val, ok := subject.Get(key)
				g.Expect(ok).To(BeFalse())
				g.Expect(val).To(BeNil())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			mockCache := mockttllru.NewMockCacher(mockCtrl)
			defer mockCtrl.Finish()
			subject, err := newCache(defaultCacheDuration, mockCache)
			g.Expect(err).Should(BeNil())
			c.TestCase(g, subject, mockCache)
		})
	}
}
