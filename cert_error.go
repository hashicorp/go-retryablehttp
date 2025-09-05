// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retryablehttp

import "crypto/tls"

func isCertError(err error) bool {
	_, ok := err.(*tls.CertificateVerificationError)
	return ok
}
