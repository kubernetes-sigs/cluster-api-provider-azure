/*
Copyright 2022 The Kubernetes Authors.

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

package data

import (
	"testing"

	. "github.com/onsi/gomega"
)

const (
	testChunkSize = 3
)

func TestSplitStringIntoChunks(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []Chunk
	}{
		{
			name:     "empty data should return empty chunk",
			data:     []byte{},
			expected: []Chunk{},
		},
		{
			name: "data with one chunk",
			data: []byte{'1', '2', '3'},
			expected: []Chunk{{
				Data:  []byte{'1', '2', '3'},
				Index: 0,
			}},
		},
		{
			name: "data size is a multiple of chunk size",
			data: []byte{'1', '2', '3', '4', '5', '6', '7', '8', '9'},
			expected: []Chunk{
				{
					Data:  []byte{'1', '2', '3'},
					Index: 0,
				},
				{
					Data:  []byte{'4', '5', '6'},
					Index: 1,
				},
				{
					Data:  []byte{'7', '8', '9'},
					Index: 2,
				},
			},
		},
		{
			name: "data size is not a multiple of chunk size",
			data: []byte{'1', '2', '3', '4', '5', '6', '7'},
			expected: []Chunk{
				{
					Data:  []byte{'1', '2', '3'},
					Index: 0,
				},
				{
					Data:  []byte{'4', '5', '6'},
					Index: 1,
				},
				{
					Data:  []byte{'7'},
					Index: 2,
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(SplitIntoChunks(tc.data, testChunkSize)).Should(Equal(tc.expected))
		})
	}
}

func TestGzipBytes(t *testing.T) {
	g := NewWithT(t)
	// we need a sufficiently large string to compress.
	toCompress := "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum."
	compressed, err := GzipBytes([]byte(toCompress))

	g.Expect(err).To(BeNil())
	g.Expect(len(compressed)).Should(BeNumerically("<", len(toCompress)))
}
