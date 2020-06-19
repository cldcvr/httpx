// Package httpx provides an expressive framework to test http endpoints and handlers.
package httpx

import (
	"context"
	"net/http"
	"reflect"
	"runtime"
)

// ExecFn defines a function that can take an http.Request and return an http.Response (and optionally, an error).
//
// This is the core type defined by this package and instances of this type does the
// actual heavy lifting work of making the request, receiving responses and more.
// How the actual execution is done is left to the implementation. Some may make actual
// http calls to remote endpoint whereas others would call in-memory http.Handler.
//
// The core package provides two implementations that works with net/http package.
type ExecFn func(*http.Request) (*http.Response, error)

// Assertable defines a function that can take a slice of assertions and apply it on http.Response.
//
// Although exported, user's won't be able to do much with this type. Instead they should use
// the ExpectIt(...) method to allow fluent chaining with MakeRequest(...).
type Assertable func(...Assertion)

// MakeRequest is the primary entry point into the framework.
//
// This method builds a request object, apply the given customisations / builders to it and
// then pass it to the ExecFn for execution returning an Assertable which you can then use to perform
// assertions on the response etc.
//
// The core library provides certain general purpose builders. See RequestBuilder and it's implementations
// for more details on builders and how you can create a custom builder.
func (fn ExecFn) MakeRequest(t TestingT, method, url string, builders ...RequestBuilder) Assertable {
	var err error

	// mark as helper to exclude from logs
	if th, ok := t.(interface {
		Helper()
	}); ok {
		th.Helper()
	}

	// build a new request and apply customisations
	var request *http.Request
	if request, err = http.NewRequestWithContext(context.Background(), method, url, http.NoBody); err != nil {
		return fail(t, "httpx: failed to create request: %v", err)
	}

	for _, fn := range builders {
		if err = fn(request); err != nil {
			return fail(t, "httpx: %s failed: %v", fn.String(), err)
		}
	}

	// execute the request
	var response *http.Response
	if response, err = fn(request); err != nil {
		return fail(t, "httpx: failed to execute request: %v", err)
	}

	// return an Assertable to run assertions on response
	return func(assertions ...Assertion) {
		for _, fn := range assertions {
			if err = fn(response); err != nil {
				t.Errorf("httpx: assertion %s failed: %v", fn.String(), err)
			}
		}
	}
}

// ExpectIt allows us to implement fluent chaining with MakeRequest(...).
// Use this method instead of directly invoking the Assertable to improve readability of your code.
func (a Assertable) ExpectIt(assertions ...Assertion) {
	a(assertions...)
}

// RequestBuilder defines a function that customises the request before it's sent out.
type RequestBuilder func(*http.Request) error

// String returns the name of the method which it looks up reflectively.
// Because of the way it work, it would return very weird names for inline functions.
// To keep your logs sane and make sense out of them, we recommend to define your
// custom RequestBuilder as separate method (not inline).
func (fn RequestBuilder) String() string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// Assertion defines a function that performs some sort of assertion on the response
// to make sure that request was executed as expected.
type Assertion func(*http.Response) error

// String returns the name of the method which it looks up reflectively.
// Because of the way it work, it would return very weird names for inline functions.
// To keep your logs sane and make sense out of them, we recommend to define your
// custom Assertion as separate method (not inline).
func (fn Assertion) String() string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// TestingT allows us to decouple our code from the actual testing.T type.
// Most end user shouldn't care about it.
type TestingT interface {
	Errorf(format string, args ...interface{})
	FailNow()
}

// fail returns a no-op Assertable that allows us to break out of MakeRequest(...) quicker.
func fail(t TestingT, format string, args ...interface{}) Assertable {
	return func(...Assertion) {
		t.Errorf(format, args...)
		t.FailNow() // doesn't return
	}
}
