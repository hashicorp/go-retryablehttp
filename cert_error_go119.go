// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !go1.20
// +build !go1.20

package retryablehttp

import "crypto/x509"

func isCertError(err error) bool {
	_, ok := err.(x509.UnknownAuthorityError)
	return ok
}
