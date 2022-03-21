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

// Chunk represents a chunk of data.
type Chunk struct {
	Data  []byte
	Index int
}

// SplitIntoChunks splits data into chunks, each of size chunkSize.
func SplitIntoChunks(bytes []byte, chunkSize int) []Chunk {
	if len(bytes) == 0 {
		return []Chunk{}
	}

	var chunks []Chunk
	i := 0
	var eof bool
	for {
		var from, to int
		from = i * chunkSize
		if !(from+chunkSize < len(bytes)) {
			to = len(bytes)
			eof = true
		} else {
			to = from + chunkSize
		}

		chunks = append(chunks, Chunk{Data: bytes[from:to], Index: i})

		if eof {
			break
		}

		i++
	}

	return chunks
}
