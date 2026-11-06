//go:build darwin && cgo

package biometric

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework LocalAuthentication -framework Foundation
#import <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>

static int shellvane_bio_available(void) {
	LAContext *context = [[LAContext alloc] init];
	NSError *error = nil;
	return [context canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&error] ? 1 : 0;
}

// 1: verified, 2: cancelled, 3: failed. Touch ID, or the Mac's password
// when Touch ID is not possible (DeviceOwnerAuthentication).
static int shellvane_bio_verify(const char *reason) {
	LAContext *context = [[LAContext alloc] init];
	__block int result = 3;
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	NSString *text = [NSString stringWithUTF8String:reason];
	[context evaluatePolicy:LAPolicyDeviceOwnerAuthentication
	        localizedReason:text
	                  reply:^(BOOL ok, NSError *error) {
		if (ok) {
			result = 1;
		} else if (error.code == LAErrorUserCancel || error.code == LAErrorAppCancel || error.code == LAErrorSystemCancel) {
			result = 2;
		}
		dispatch_semaphore_signal(done);
	}];
	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	return result;
}
*/
import "C"

import (
	"context"
	"unsafe"

	"github.com/csturiale/shellvane/core/errs"
)

type system struct{}

func (system) Kind() string { return KindTouchID }

func (system) Available(context.Context) bool { return C.shellvane_bio_available() == 1 }

func (system) Verify(_ context.Context, reason string) error {
	text := C.CString(reason)
	defer C.free(unsafe.Pointer(text))
	switch C.shellvane_bio_verify(text) {
	case 1:
		return nil
	case 2:
		return errs.New(ErrCancelled, nil)
	}
	return errs.New(ErrFailed, nil)
}
