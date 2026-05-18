package object_store

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/smithy-go"

	"lazuli.dev/runtime/lazuli/storage"
)

// fakeAPIError is the minimal smithy.APIError implementation used to
// drive mapErr without standing up an AWS endpoint.
type fakeAPIError struct {
	code string
	msg  string
}

func (f *fakeAPIError) Error() string                     { return f.code + ": " + f.msg }
func (f *fakeAPIError) ErrorCode() string                 { return f.code }
func (f *fakeAPIError) ErrorMessage() string              { return f.msg }
func (f *fakeAPIError) ErrorFault() smithy.ErrorFault     { return smithy.FaultUnknown }

func TestAdapterSatisfiesObjectStore(t *testing.T) {
	// The compile-time assertion in adapter.go already enforces this;
	// the runtime test makes the intent explicit.
	var _ storage.ObjectStore = (*Adapter)(nil)
}

func TestUnconfiguredAdapterDefersErrors(t *testing.T) {
	a := &Adapter{err: errors.New("boom")}

	if err := a.Put(context.Background(), "k", strings.NewReader("x"), "text/plain"); err == nil {
		t.Fatal("Put: want error, got nil")
	}
	if _, err := a.Get(context.Background(), "k"); err == nil {
		t.Fatal("Get: want error, got nil")
	}
	if err := a.Delete(context.Background(), "k"); err == nil {
		t.Fatal("Delete: want error, got nil")
	}
	if _, err := a.Sign(context.Background(), "k", 1); err == nil {
		t.Fatal("Sign: want error, got nil")
	}
	if _, err := a.ListPrefix(context.Background(), "p/"); err == nil {
		t.Fatal("ListPrefix: want error, got nil")
	}
}

func TestSignRefusesZeroTTL(t *testing.T) {
	a := &Adapter{}
	_, err := a.Sign(context.Background(), "k", 0)
	if !errors.Is(err, storage.ErrVisibilityMismatch) {
		t.Errorf("Sign(ttl=0): want ErrVisibilityMismatch, got %v", err)
	}
}

func TestMapErrTranslatesNotFoundCodes(t *testing.T) {
	cases := []string{"NoSuchKey", "NotFound", "NoSuchBucket"}
	for _, code := range cases {
		err := mapErr(&fakeAPIError{code: code, msg: code})
		if !errors.Is(err, storage.ErrFileNotFound) {
			t.Errorf("mapErr(%s): want ErrFileNotFound, got %v", code, err)
		}
	}
}

func TestMapErrTranslatesAuthCodes(t *testing.T) {
	cases := []string{"AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch"}
	for _, code := range cases {
		err := mapErr(&fakeAPIError{code: code, msg: code})
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("mapErr(%s): want ErrUnauthorized, got %v", code, err)
		}
	}
}

func TestMapErrPassesThroughNetworkErrors(t *testing.T) {
	want := errors.New("dial tcp: lookup unreachable: no such host")
	got := mapErr(want)
	if !errors.Is(got, want) {
		t.Errorf("mapErr(network): want passthrough, got %v", got)
	}
}

func TestMapErrNilStaysNil(t *testing.T) {
	if mapErr(nil) != nil {
		t.Fatal("mapErr(nil): want nil")
	}
}

// Compile-time: io.Reader is a valid Put body.
var _ io.Reader = strings.NewReader("")
