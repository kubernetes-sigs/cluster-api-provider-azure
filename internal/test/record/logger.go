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

// Package record provides a test-friendly logr.Logger.
package record

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// LogEntry defines the information that can be used for composing a log line.
type LogEntry struct {
	// Prefix of the log line, composed of the hierarchy of log.WithName values.
	Prefix string

	// LogFunc of the log entry, e.g. "Info", "Error"
	LogFunc string

	// Level of the LogEntry.
	Level int

	// Values of the log line, composed of the concatenation of log.WithValues and KeyValue pairs passed to log.Info.
	Values []interface{}
}

// Option is a configuration option supplied to NewLogger.
type Option func(*Logger)

// WithThreshold implements a New Option that allows to set the threshold level for a new logger.
// The logger will write only log messages with a level/V(x) equal or higher to the threshold.
func WithThreshold(threshold *int) Option {
	return func(c *Logger) {
		c.threshold = threshold
	}
}

// WithWriter configures the logger to write to the io.Writer.
func WithWriter(wr io.Writer) Option {
	return func(c *Logger) {
		c.writer = wr
	}
}

// NewLogger returns a new instance of the clusterctl.
func NewLogger(options ...Option) *Logger {
	l := &Logger{
		listeners: map[string]*Listener{},
	}
	for _, o := range options {
		o(l)
	}
	return l
}

var _ logr.LogSink = (*Logger)(nil)

type (
	// Logger defines a test-friendly logr.Logger.
	Logger struct {
		threshold  *int
		level      int
		prefix     string
		values     []interface{}
		listenerMu sync.Mutex
		listeners  map[string]*Listener
		writer     io.Writer
		root       *Logger
		cloneMu    sync.Mutex
		info       logr.RuntimeInfo
	}

	// Listener defines a listener for log entries.
	Listener struct {
		logger    *Logger
		entriesMu sync.RWMutex
		entries   []LogEntry
	}
)

// NewListener returns a new listener with the specified logger.
func NewListener(logger *Logger) *Listener {
	return &Listener{
		logger: logger,
	}
}

// Listen adds this listener to its logger.
func (li *Listener) Listen() func() {
	return li.logger.addListener(li)
}

// GetEntries returns a copy of the list of log entries.
func (li *Listener) GetEntries() []LogEntry {
	li.entriesMu.RLock()
	defer li.entriesMu.RUnlock()

	return copyLogEntries(li.entries)
}

func (li *Listener) addEntry(entry LogEntry) {
	li.entriesMu.Lock()
	defer li.entriesMu.Unlock()

	li.entries = append(li.entries, entry)
}

var _ logr.LogSink = &Logger{}

func (l *Logger) addListener(listener *Listener) func() {
	if l.root != nil {
		return l.root.addListener(listener)
	}

	l.listenerMu.Lock()
	defer l.listenerMu.Unlock()

	id := uuid.New().String()
	l.listeners[id] = listener
	return func() {
		l.removeListener(id)
	}
}

func (l *Logger) removeListener(id string) {
	l.listenerMu.Lock()
	defer l.listenerMu.Unlock()

	delete(l.listeners, id)
}

// Init initializes the logger from runtime information.
func (l *Logger) Init(info logr.RuntimeInfo) {
	l.info = info
}

// Enabled is always enabled.
func (l *Logger) Enabled(_ int) bool {
	return true
}

// Info logs a non-error message with the given key/value pairs as context.
func (l *Logger) Info(_ int, msg string, kvs ...interface{}) {
	values := copySlice(l.values)
	values = append(values, kvs...)
	values = append(values, "msg", msg)
	l.write("Info", values)
}

// Error logs an error message with the given key/value pairs as context.
func (l *Logger) Error(err error, msg string, kvs ...interface{}) {
	values := copySlice(l.values)
	values = append(values, kvs...)
	values = append(values, "msg", msg, "error", err)
	l.write("Error", values)
}

// V returns an Logger value for a specific verbosity level.
func (l *Logger) V(level int) logr.LogSink {
	nl := l.clone()
	nl.level = level
	return nl
}

// WithName adds a new element to the logger's name.
func (l *Logger) WithName(name string) logr.LogSink {
	nl := l.clone()
	if l.prefix != "" {
		nl.prefix = l.prefix + "/"
	}
	nl.prefix += name
	return nl
}

// WithValues adds some key-value pairs of context to a logger.
func (l *Logger) WithValues(kvList ...interface{}) logr.LogSink {
	nl := l.clone()
	nl.values = append(nl.values, kvList...)
	return nl
}

func (l *Logger) write(logFunc string, values []interface{}) {
	entry := LogEntry{
		Prefix:  l.prefix,
		LogFunc: logFunc,
		Level:   l.level,
		Values:  copySlice(values),
	}
	f, err := flatten(entry)
	if err != nil {
		panic(err)
	}

	l.writeToListeners(entry)

	if l.writer != nil {
		str := fmt.Sprintf("%s\n", f)
		if _, err = l.writer.Write([]byte(str)); err != nil {
			panic(err)
		}
		return
	}

	fmt.Println(f)
}

func (l *Logger) writeToListeners(entry LogEntry) {
	if l.root != nil {
		l.root.writeToListeners(entry)
		return
	}

	l.listenerMu.Lock()
	defer l.listenerMu.Unlock()

	for _, listener := range l.listeners {
		listener.addEntry(entry)
	}
}

func (l *Logger) clone() *Logger {
	l.cloneMu.Lock()
	defer l.cloneMu.Unlock()

	root := l.root
	if root == nil {
		root = l
	}

	return &Logger{
		threshold: l.threshold,
		level:     l.level,
		prefix:    l.prefix,
		values:    copySlice(l.values),
		writer:    l.writer,
		root:      root,
	}
}

func copyLogEntries(in []LogEntry) []LogEntry {
	out := make([]LogEntry, len(in))
	copy(out, in)
	return out
}

func copySlice(in []interface{}) []interface{} {
	out := make([]interface{}, len(in))
	copy(out, in)
	return out
}

// flatten returns a human readable/machine parsable text representing the LogEntry.
// Most notable difference with the klog implementation are:
//   - The message is printed at the beginning of the line, without the Msg= variable name e.g.
//     "Msg"="This is a message" --> This is a message
//   - Variables name are not quoted, eg.
//     This is a message "Var1"="value" --> This is a message Var1="value"
//   - Variables are not sorted, thus allowing full control to the developer on the output.
func flatten(entry LogEntry) (string, error) {
	var msgValue string
	var errorValue error
	if len(entry.Values)%2 == 1 {
		return "", errors.New("log entry cannot have odd number off keyAndValues")
	}

	keys := make([]string, 0, len(entry.Values)/2)
	values := make(map[string]interface{}, len(entry.Values)/2)
	for i := 0; i < len(entry.Values); i += 2 {
		k, ok := entry.Values[i].(string)
		if !ok {
			panic(fmt.Sprintf("key is not a string: %s", entry.Values[i]))
		}
		var v interface{}
		if i+1 < len(entry.Values) {
			v = entry.Values[i+1]
		}
		switch k {
		case "msg":
			msgValue, ok = v.(string)
			if !ok {
				panic(fmt.Sprintf("the msg value is not of type string: %s", v))
			}
		case "error":
			errorValue, ok = v.(error)
			if !ok {
				panic(fmt.Sprintf("the error value is not of type error: %s", v))
			}
		default:
			if _, ok := values[k]; !ok {
				keys = append(keys, k)
			}
			values[k] = v
		}
	}
	str := ""
	if entry.Prefix != "" {
		str += fmt.Sprintf("[%s] ", entry.Prefix)
	}
	str += msgValue
	if errorValue != nil {
		if msgValue != "" {
			str += ": "
		}
		str += errorValue.Error()
	}
	for _, k := range keys {
		prettyValue, err := pretty(values[k])
		if err != nil {
			return "", err
		}
		str += fmt.Sprintf(" %s=%s", k, prettyValue)
	}
	return str, nil
}

func pretty(value interface{}) (string, error) {
	jb, err := json.Marshal(value)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to marshal %s", value)
	}
	return string(jb), nil
}
