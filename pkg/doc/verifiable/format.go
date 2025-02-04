/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifiable

import (
	"encoding/json"
	"fmt"
)

type Format string

const (
	Jwt Format = "jwt"
	Ldp Format = "ldp"
)

func ValidateFormat(data interface{}, formats []Format) ([]byte, error) {
	strRep, isStr := data.(string)

	var dataBytes []byte

	if isStr {
		if !isFormatSupported(Jwt, formats) {
			return nil, fmt.Errorf("invlaid format, should be %s", Jwt)
		}

		dataBytes = []byte(strRep)
	}

	if !isStr {
		if !isFormatSupported(Ldp, formats) {
			return nil, fmt.Errorf("invlaid format, should be %s", Ldp)
		}

		var err error
		dataBytes, err = json.Marshal(data)

		if err != nil {
			return nil, fmt.Errorf("invlaid format: %w", err)
		}
	}

	return dataBytes, nil
}

func isFormatSupported(format Format, supportedFormats []Format) bool {
	for _, supported := range supportedFormats {
		if format == supported {
			return true
		}
	}
	return false
}
